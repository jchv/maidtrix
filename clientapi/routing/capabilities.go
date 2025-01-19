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

package routing

import (
	"net/http"

	"github.com/jchv/maidtrix/internal/matrixserver"
	"github.com/jchv/maidtrix/internal/util"
	roomserverAPI "github.com/jchv/maidtrix/roomserver/api"
	"github.com/jchv/maidtrix/roomserver/version"
)

// GetCapabilities returns information about the server's supported feature set
// and other relevant capabilities to an authenticated user.
func GetCapabilities(rsAPI roomserverAPI.ClientRoomserverAPI) util.JSONResponse {
	versionsMap := map[gomatrixserverlib.RoomVersion]string{}
	for v, desc := range version.SupportedRoomVersions() {
		if desc.Stable() {
			versionsMap[v] = "stable"
		} else {
			versionsMap[v] = "unstable"
		}
	}

	response := map[string]interface{}{
		"capabilities": map[string]interface{}{
			"m.change_password": map[string]bool{
				"enabled": true,
			},
			"m.room_versions": map[string]interface{}{
				"default":   rsAPI.DefaultRoomVersion(),
				"available": versionsMap,
			},
		},
	}

	return util.JSONResponse{
		Code: http.StatusOK,
		JSON: response,
	}
}
