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

package routing

import (
	"errors"
	"net/http"

	appserviceAPI "github.com/jchv/maidtrix/appservice/api"
	"github.com/jchv/maidtrix/clientapi/httputil"
	"github.com/jchv/maidtrix/internal/eventutil"
	"github.com/jchv/maidtrix/internal/matrixserver"
	"github.com/jchv/maidtrix/internal/matrixserver/spec"
	"github.com/jchv/maidtrix/internal/util"
	roomserverAPI "github.com/jchv/maidtrix/roomserver/api"
	"github.com/jchv/maidtrix/roomserver/version"
	"github.com/jchv/maidtrix/setup/config"
	userapi "github.com/jchv/maidtrix/userapi/api"
)

type upgradeRoomRequest struct {
	NewVersion string `json:"new_version"`
}

type upgradeRoomResponse struct {
	ReplacementRoom string `json:"replacement_room"`
}

// UpgradeRoom implements /upgrade
func UpgradeRoom(
	req *http.Request, device *userapi.Device,
	cfg *config.ClientAPI,
	roomID string, profileAPI userapi.ClientUserAPI,
	rsAPI roomserverAPI.ClientRoomserverAPI,
	asAPI appserviceAPI.AppServiceInternalAPI,
) util.JSONResponse {
	var r upgradeRoomRequest
	if rErr := httputil.UnmarshalJSONRequest(req, &r); rErr != nil {
		return *rErr
	}

	// Validate that the room version is supported
	if _, err := version.SupportedRoomVersion(gomatrixserverlib.RoomVersion(r.NewVersion)); err != nil {
		return util.JSONResponse{
			Code: http.StatusBadRequest,
			JSON: spec.UnsupportedRoomVersion("This server does not support that room version"),
		}
	}

	userID, err := spec.NewUserID(device.UserID, true)
	if err != nil {
		util.GetLogger(req.Context()).WithError(err).Error("device UserID is invalid")
		return util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: spec.InternalServerError{},
		}
	}
	newRoomID, err := rsAPI.PerformRoomUpgrade(req.Context(), roomID, *userID, gomatrixserverlib.RoomVersion(r.NewVersion))
	switch e := err.(type) {
	case nil:
	case roomserverAPI.ErrNotAllowed:
		return util.JSONResponse{
			Code: http.StatusForbidden,
			JSON: spec.Forbidden(e.Error()),
		}
	default:
		if errors.Is(err, eventutil.ErrRoomNoExists{}) {
			return util.JSONResponse{
				Code: http.StatusNotFound,
				JSON: spec.NotFound("Room does not exist"),
			}
		}
		return util.JSONResponse{
			Code: http.StatusInternalServerError,
			JSON: spec.InternalServerError{},
		}
	}

	return util.JSONResponse{
		Code: http.StatusOK,
		JSON: upgradeRoomResponse{
			ReplacementRoom: newRoomID,
		},
	}
}
