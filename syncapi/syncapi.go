// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package syncapi

import (
	"context"

	"github.com/jchv/maidtrix/internal/fulltext"
	"github.com/jchv/maidtrix/internal/httputil"
	"github.com/jchv/maidtrix/internal/sqlutil"
	"github.com/jchv/maidtrix/setup/config"
	"github.com/jchv/maidtrix/setup/process"
	"github.com/sirupsen/logrus"

	"github.com/jchv/maidtrix/internal/caching"

	"github.com/jchv/maidtrix/roomserver/api"
	"github.com/jchv/maidtrix/setup/jetstream"
	userapi "github.com/jchv/maidtrix/userapi/api"

	"github.com/jchv/maidtrix/syncapi/consumers"
	"github.com/jchv/maidtrix/syncapi/notifier"
	"github.com/jchv/maidtrix/syncapi/producers"
	"github.com/jchv/maidtrix/syncapi/routing"
	"github.com/jchv/maidtrix/syncapi/storage"
	"github.com/jchv/maidtrix/syncapi/streams"
	"github.com/jchv/maidtrix/syncapi/sync"
)

// AddPublicRoutes sets up and registers HTTP handlers for the SyncAPI
// component.
func AddPublicRoutes(
	processContext *process.ProcessContext,
	routers httputil.Routers,
	dendriteCfg *config.Dendrite,
	cm *sqlutil.Connections,
	natsInstance *jetstream.NATSInstance,
	userAPI userapi.SyncUserAPI,
	rsAPI api.SyncRoomserverAPI,
	caches caching.LazyLoadCache,
	enableMetrics bool,
) {
	js, natsClient := natsInstance.Prepare(processContext, &dendriteCfg.Global.JetStream)

	syncDB, err := storage.NewSyncServerDatasource(processContext.Context(), cm, &dendriteCfg.SyncAPI.Database)
	if err != nil {
		logrus.WithError(err).Panicf("failed to connect to sync db")
	}

	eduCache := caching.NewTypingCache()
	notifier := notifier.NewNotifier(rsAPI)
	streams := streams.NewSyncStreamProviders(syncDB, userAPI, rsAPI, eduCache, caches, notifier)
	notifier.SetCurrentPosition(streams.Latest(context.Background()))
	if err = notifier.Load(context.Background(), syncDB); err != nil {
		logrus.WithError(err).Panicf("failed to load notifier ")
	}

	var fts *fulltext.Search
	if dendriteCfg.SyncAPI.Fulltext.Enabled {
		fts, err = fulltext.New(processContext, dendriteCfg.SyncAPI.Fulltext)
		if err != nil {
			logrus.WithError(err).Panicf("failed to create full text")
		}
	}

	federationPresenceProducer := &producers.FederationAPIPresenceProducer{
		Topic:     dendriteCfg.Global.JetStream.Prefixed(jetstream.OutputPresenceEvent),
		JetStream: js,
	}
	presenceConsumer := consumers.NewPresenceConsumer(
		processContext, &dendriteCfg.SyncAPI, js, natsClient, syncDB,
		notifier, streams.PresenceStreamProvider,
		userAPI,
	)

	requestPool := sync.NewRequestPool(syncDB, &dendriteCfg.SyncAPI, userAPI, rsAPI, streams, notifier, federationPresenceProducer, presenceConsumer, enableMetrics)

	if err = presenceConsumer.Start(); err != nil {
		logrus.WithError(err).Panicf("failed to start presence consumer")
	}

	keyChangeConsumer := consumers.NewOutputKeyChangeEventConsumer(
		processContext, &dendriteCfg.SyncAPI, dendriteCfg.Global.JetStream.Prefixed(jetstream.OutputKeyChangeEvent),
		js, rsAPI, syncDB, notifier,
		streams.DeviceListStreamProvider,
	)
	if err = keyChangeConsumer.Start(); err != nil {
		logrus.WithError(err).Panicf("failed to start key change consumer")
	}

	var asProducer *producers.AppserviceEventProducer
	if len(dendriteCfg.AppServiceAPI.Derived.ApplicationServices) > 0 {
		asProducer = &producers.AppserviceEventProducer{
			JetStream: js, Topic: dendriteCfg.Global.JetStream.Prefixed(jetstream.OutputAppserviceEvent),
		}
	}

	roomConsumer := consumers.NewOutputRoomEventConsumer(
		processContext, &dendriteCfg.SyncAPI, js, syncDB, notifier, streams.PDUStreamProvider,
		streams.InviteStreamProvider, rsAPI, fts, asProducer,
	)
	if err = roomConsumer.Start(); err != nil {
		logrus.WithError(err).Panicf("failed to start room server consumer")
	}

	clientConsumer := consumers.NewOutputClientDataConsumer(
		processContext, &dendriteCfg.SyncAPI, js, natsClient, syncDB, notifier,
		streams.AccountDataStreamProvider, fts,
	)
	if err = clientConsumer.Start(); err != nil {
		logrus.WithError(err).Panicf("failed to start client data consumer")
	}

	notificationConsumer := consumers.NewOutputNotificationDataConsumer(
		processContext, &dendriteCfg.SyncAPI, js, syncDB, notifier, streams.NotificationDataStreamProvider,
	)
	if err = notificationConsumer.Start(); err != nil {
		logrus.WithError(err).Panicf("failed to start notification data consumer")
	}

	typingConsumer := consumers.NewOutputTypingEventConsumer(
		processContext, &dendriteCfg.SyncAPI, js, eduCache, notifier, streams.TypingStreamProvider,
	)
	if err = typingConsumer.Start(); err != nil {
		logrus.WithError(err).Panicf("failed to start typing consumer")
	}

	sendToDeviceConsumer := consumers.NewOutputSendToDeviceEventConsumer(
		processContext, &dendriteCfg.SyncAPI, js, syncDB, userAPI, notifier, streams.SendToDeviceStreamProvider,
	)
	if err = sendToDeviceConsumer.Start(); err != nil {
		logrus.WithError(err).Panicf("failed to start send-to-device consumer")
	}

	receiptConsumer := consumers.NewOutputReceiptEventConsumer(
		processContext, &dendriteCfg.SyncAPI, js, syncDB, notifier, streams.ReceiptStreamProvider,
	)
	if err = receiptConsumer.Start(); err != nil {
		logrus.WithError(err).Panicf("failed to start receipts consumer")
	}

	rateLimits := httputil.NewRateLimits(&dendriteCfg.ClientAPI.RateLimiting)

	routing.Setup(
		routers.Client, requestPool, syncDB, userAPI,
		rsAPI, &dendriteCfg.SyncAPI, caches, fts,
		rateLimits,
	)
}
