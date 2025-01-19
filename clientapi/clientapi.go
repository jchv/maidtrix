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

package clientapi

import (
	"github.com/jchv/maidtrix/internal/httputil"
	"github.com/jchv/maidtrix/internal/matrixserver/fclient"
	"github.com/jchv/maidtrix/setup/config"
	"github.com/jchv/maidtrix/setup/process"
	userapi "github.com/jchv/maidtrix/userapi/api"

	appserviceAPI "github.com/jchv/maidtrix/appservice/api"
	"github.com/jchv/maidtrix/clientapi/api"
	"github.com/jchv/maidtrix/clientapi/producers"
	"github.com/jchv/maidtrix/clientapi/routing"
	federationAPI "github.com/jchv/maidtrix/federationapi/api"
	"github.com/jchv/maidtrix/internal/transactions"
	roomserverAPI "github.com/jchv/maidtrix/roomserver/api"
	"github.com/jchv/maidtrix/setup/jetstream"
)

// AddPublicRoutes sets up and registers HTTP handlers for the ClientAPI component.
func AddPublicRoutes(
	processContext *process.ProcessContext,
	routers httputil.Routers,
	cfg *config.Dendrite,
	natsInstance *jetstream.NATSInstance,
	federation fclient.FederationClient,
	rsAPI roomserverAPI.ClientRoomserverAPI,
	asAPI appserviceAPI.AppServiceInternalAPI,
	transactionsCache *transactions.Cache,
	fsAPI federationAPI.ClientFederationAPI,
	userAPI userapi.ClientUserAPI,
	userDirectoryProvider userapi.QuerySearchProfilesAPI,
	extRoomsProvider api.ExtraPublicRoomsProvider, enableMetrics bool,
) {
	js, natsClient := natsInstance.Prepare(processContext, &cfg.Global.JetStream)

	syncProducer := &producers.SyncAPIProducer{
		JetStream:              js,
		TopicReceiptEvent:      cfg.Global.JetStream.Prefixed(jetstream.OutputReceiptEvent),
		TopicSendToDeviceEvent: cfg.Global.JetStream.Prefixed(jetstream.OutputSendToDeviceEvent),
		TopicTypingEvent:       cfg.Global.JetStream.Prefixed(jetstream.OutputTypingEvent),
		TopicPresenceEvent:     cfg.Global.JetStream.Prefixed(jetstream.OutputPresenceEvent),
		UserAPI:                userAPI,
		ServerName:             cfg.Global.ServerName,
	}

	routing.Setup(
		routers,
		cfg, rsAPI, asAPI,
		userAPI, userDirectoryProvider, federation,
		syncProducer, transactionsCache, fsAPI,
		extRoomsProvider, natsClient, enableMetrics,
	)
}
