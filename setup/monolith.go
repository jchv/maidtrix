// Copyright 2020 The Matrix.org Foundation C.I.C.
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

package setup

import (
	appserviceAPI "github.com/jchv/maidtrix/appservice/api"
	"github.com/jchv/maidtrix/clientapi"
	"github.com/jchv/maidtrix/clientapi/api"
	"github.com/jchv/maidtrix/federationapi"
	federationAPI "github.com/jchv/maidtrix/federationapi/api"
	"github.com/jchv/maidtrix/internal/caching"
	"github.com/jchv/maidtrix/internal/httputil"
	"github.com/jchv/maidtrix/internal/matrixserver"
	"github.com/jchv/maidtrix/internal/matrixserver/fclient"
	"github.com/jchv/maidtrix/internal/sqlutil"
	"github.com/jchv/maidtrix/internal/transactions"
	"github.com/jchv/maidtrix/mediaapi"
	"github.com/jchv/maidtrix/relayapi"
	relayAPI "github.com/jchv/maidtrix/relayapi/api"
	roomserverAPI "github.com/jchv/maidtrix/roomserver/api"
	"github.com/jchv/maidtrix/setup/config"
	"github.com/jchv/maidtrix/setup/jetstream"
	"github.com/jchv/maidtrix/setup/process"
	"github.com/jchv/maidtrix/syncapi"
	userapi "github.com/jchv/maidtrix/userapi/api"
)

// Monolith represents an instantiation of all dependencies required to build
// all components of Dendrite, for use in monolith mode.
type Monolith struct {
	Config    *config.Dendrite
	KeyRing   *gomatrixserverlib.KeyRing
	Client    *fclient.Client
	FedClient fclient.FederationClient

	AppserviceAPI appserviceAPI.AppServiceInternalAPI
	FederationAPI federationAPI.FederationInternalAPI
	RoomserverAPI roomserverAPI.RoomserverInternalAPI
	UserAPI       userapi.UserInternalAPI
	RelayAPI      relayAPI.RelayInternalAPI

	// Optional
	ExtPublicRoomsProvider   api.ExtraPublicRoomsProvider
	ExtUserDirectoryProvider userapi.QuerySearchProfilesAPI
}

// AddAllPublicRoutes attaches all public paths to the given router
func (m *Monolith) AddAllPublicRoutes(
	processCtx *process.ProcessContext,
	cfg *config.Dendrite,
	routers httputil.Routers,
	cm *sqlutil.Connections,
	natsInstance *jetstream.NATSInstance,
	caches *caching.Caches,
	enableMetrics bool,
) {
	userDirectoryProvider := m.ExtUserDirectoryProvider
	if userDirectoryProvider == nil {
		userDirectoryProvider = m.UserAPI
	}
	clientapi.AddPublicRoutes(
		processCtx, routers, cfg, natsInstance, m.FedClient, m.RoomserverAPI, m.AppserviceAPI, transactions.New(),
		m.FederationAPI, m.UserAPI, userDirectoryProvider,
		m.ExtPublicRoomsProvider, enableMetrics,
	)
	federationapi.AddPublicRoutes(
		processCtx, routers, cfg, natsInstance, m.UserAPI, m.FedClient, m.KeyRing, m.RoomserverAPI, m.FederationAPI, enableMetrics,
	)
	mediaapi.AddPublicRoutes(routers, cm, cfg, m.UserAPI, m.Client, m.FedClient, m.KeyRing)
	syncapi.AddPublicRoutes(processCtx, routers, cfg, cm, natsInstance, m.UserAPI, m.RoomserverAPI, caches, enableMetrics)

	if m.RelayAPI != nil {
		relayapi.AddPublicRoutes(routers, cfg, m.KeyRing, m.RelayAPI)
	}
}
