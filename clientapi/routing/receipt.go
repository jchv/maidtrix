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
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jchv/maidtrix/clientapi/producers"
	"github.com/jchv/maidtrix/internal/matrixserver/spec"

	"github.com/jchv/maidtrix/internal/util"
	userapi "github.com/jchv/maidtrix/userapi/api"
	"github.com/sirupsen/logrus"
)

func SetReceipt(req *http.Request, userAPI userapi.ClientUserAPI, syncProducer *producers.SyncAPIProducer, device *userapi.Device, roomID, receiptType, eventID string) util.JSONResponse {
	timestamp := spec.AsTimestamp(time.Now())
	logrus.WithFields(logrus.Fields{
		"roomID":      roomID,
		"receiptType": receiptType,
		"eventID":     eventID,
		"userId":      device.UserID,
		"timestamp":   timestamp,
	}).Debug("Setting receipt")

	switch receiptType {
	case "m.read", "m.read.private":
		if err := syncProducer.SendReceipt(req.Context(), device.UserID, roomID, eventID, receiptType, timestamp); err != nil {
			return util.ErrorResponse(err)
		}

	case "m.fully_read":
		data, err := json.Marshal(fullyReadEvent{EventID: eventID})
		if err != nil {
			return util.JSONResponse{
				Code: http.StatusInternalServerError,
				JSON: spec.InternalServerError{},
			}
		}

		dataReq := userapi.InputAccountDataRequest{
			UserID:      device.UserID,
			DataType:    "m.fully_read",
			RoomID:      roomID,
			AccountData: data,
		}
		dataRes := userapi.InputAccountDataResponse{}
		if err := userAPI.InputAccountData(req.Context(), &dataReq, &dataRes); err != nil {
			util.GetLogger(req.Context()).WithError(err).Error("userAPI.InputAccountData failed")
			return util.ErrorResponse(err)
		}

	default:
		return util.MessageResponse(400, fmt.Sprintf("Receipt type '%s' not known", receiptType))
	}

	return util.JSONResponse{
		Code: http.StatusOK,
		JSON: struct{}{},
	}
}
