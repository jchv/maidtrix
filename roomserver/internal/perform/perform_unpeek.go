// Copyright 2020 New Vector Ltd
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

package perform

import (
	"context"
	"fmt"
	"strings"

	fsAPI "github.com/jchv/maidtrix/federationapi/api"
	"github.com/jchv/maidtrix/internal/matrixserver"
	"github.com/jchv/maidtrix/internal/matrixserver/spec"
	"github.com/jchv/maidtrix/roomserver/api"
	"github.com/jchv/maidtrix/roomserver/internal/input"
	"github.com/jchv/maidtrix/setup/config"
)

type Unpeeker struct {
	ServerName spec.ServerName
	Cfg        *config.RoomServer
	FSAPI      fsAPI.RoomserverFederationAPI
	Inputer    *input.Inputer
}

// PerformUnpeek handles un-peeking matrix rooms, including over federation by talking to the federationapi.
func (r *Unpeeker) PerformUnpeek(
	ctx context.Context,
	roomID, userID, deviceID string,
) error {
	// FIXME: there's way too much duplication with performJoin
	_, domain, err := gomatrixserverlib.SplitID('@', userID)
	if err != nil {
		return api.ErrInvalidID{Err: fmt.Errorf("supplied user ID %q in incorrect format", userID)}
	}
	if !r.Cfg.Matrix.IsLocalServerName(domain) {
		return api.ErrInvalidID{Err: fmt.Errorf("user %q does not belong to this homeserver", userID)}
	}
	if strings.HasPrefix(roomID, "!") {
		return r.performUnpeekRoomByID(ctx, roomID, userID, deviceID)
	}
	return api.ErrInvalidID{Err: fmt.Errorf("room ID %q is invalid", roomID)}
}

func (r *Unpeeker) performUnpeekRoomByID(
	_ context.Context,
	roomID, userID, deviceID string,
) (err error) {
	// Get the domain part of the room ID.
	_, _, err = gomatrixserverlib.SplitID('!', roomID)
	if err != nil {
		return api.ErrInvalidID{Err: fmt.Errorf("room ID %q is invalid: %w", roomID, err)}
	}

	// TODO: handle federated peeks
	// By this point, if req.RoomIDOrAlias contained an alias, then
	// it will have been overwritten with a room ID by performPeekRoomByAlias.
	// We should now include this in the response so that the CS API can
	// return the right room ID.
	return r.Inputer.OutputProducer.ProduceRoomEvents(roomID, []api.OutputEvent{
		{
			Type: api.OutputTypeRetirePeek,
			RetirePeek: &api.OutputRetirePeek{
				RoomID:   roomID,
				UserID:   userID,
				DeviceID: deviceID,
			},
		},
	})
}
