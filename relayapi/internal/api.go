// Copyright 2022 The Matrix.org Foundation C.I.C.
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

package internal

import (
	"sync"

	"github.com/jchv/dendrite/federationapi/producers"
	"github.com/jchv/dendrite/internal/matrixserver"
	"github.com/jchv/dendrite/internal/matrixserver/fclient"
	"github.com/jchv/dendrite/internal/matrixserver/spec"
	"github.com/jchv/dendrite/relayapi/storage"
	rsAPI "github.com/jchv/dendrite/roomserver/api"
)

type RelayInternalAPI struct {
	db                     storage.Database
	fedClient              fclient.FederationClient
	rsAPI                  rsAPI.RoomserverInternalAPI
	keyRing                *gomatrixserverlib.KeyRing
	producer               *producers.SyncAPIProducer
	presenceEnabledInbound bool
	serverName             spec.ServerName
	relayingEnabledMutex   sync.Mutex
	relayingEnabled        bool
}

func NewRelayInternalAPI(
	db storage.Database,
	fedClient fclient.FederationClient,
	rsAPI rsAPI.RoomserverInternalAPI,
	keyRing *gomatrixserverlib.KeyRing,
	producer *producers.SyncAPIProducer,
	presenceEnabledInbound bool,
	serverName spec.ServerName,
	relayingEnabled bool,
) *RelayInternalAPI {
	return &RelayInternalAPI{
		db:                     db,
		fedClient:              fedClient,
		rsAPI:                  rsAPI,
		keyRing:                keyRing,
		producer:               producer,
		presenceEnabledInbound: presenceEnabledInbound,
		serverName:             serverName,
		relayingEnabled:        relayingEnabled,
	}
}
