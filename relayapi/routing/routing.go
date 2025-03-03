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
	"fmt"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gorilla/mux"
	"github.com/jchv/maidtrix/internal/httputil"
	"github.com/jchv/maidtrix/internal/matrixserver"
	"github.com/jchv/maidtrix/internal/matrixserver/fclient"
	"github.com/jchv/maidtrix/internal/matrixserver/spec"
	"github.com/jchv/maidtrix/internal/util"
	relayInternal "github.com/jchv/maidtrix/relayapi/internal"
	"github.com/jchv/maidtrix/setup/config"
	"github.com/sirupsen/logrus"
)

// Setup registers HTTP handlers with the given ServeMux.
// The provided publicAPIMux MUST have `UseEncodedPath()` enabled or else routes will incorrectly
// path unescape twice (once from the router, once from MakeRelayAPI). We need to have this enabled
// so we can decode paths like foo/bar%2Fbaz as [foo, bar/baz] - by default it will decode to [foo, bar, baz]
func Setup(
	fedMux *mux.Router,
	cfg *config.FederationAPI,
	relayAPI *relayInternal.RelayInternalAPI,
	keys gomatrixserverlib.JSONVerifier,
) {
	v1fedmux := fedMux.PathPrefix("/v1").Subrouter()

	v1fedmux.Handle("/send_relay/{txnID}/{userID}", MakeRelayAPI(
		"send_relay_transaction", "", cfg.Matrix.IsLocalServerName, keys,
		func(httpReq *http.Request, request *fclient.FederationRequest, vars map[string]string) util.JSONResponse {
			logrus.Infof("Handling send_relay from: %s", request.Origin())
			if !relayAPI.RelayingEnabled() {
				logrus.Warnf("Dropping send_relay from: %s", request.Origin())
				return util.JSONResponse{
					Code: http.StatusNotFound,
				}
			}

			userID, err := spec.NewUserID(vars["userID"], false)
			if err != nil {
				return util.JSONResponse{
					Code: http.StatusBadRequest,
					JSON: spec.InvalidUsername("Username was invalid"),
				}
			}
			return SendTransactionToRelay(
				httpReq, request, relayAPI, gomatrixserverlib.TransactionID(vars["txnID"]),
				*userID,
			)
		},
	)).Methods(http.MethodPut, http.MethodOptions)

	v1fedmux.Handle("/relay_txn/{userID}", MakeRelayAPI(
		"get_relay_transaction", "", cfg.Matrix.IsLocalServerName, keys,
		func(httpReq *http.Request, request *fclient.FederationRequest, vars map[string]string) util.JSONResponse {
			logrus.Infof("Handling relay_txn from: %s", request.Origin())
			if !relayAPI.RelayingEnabled() {
				logrus.Warnf("Dropping relay_txn from: %s", request.Origin())
				return util.JSONResponse{
					Code: http.StatusNotFound,
				}
			}

			userID, err := spec.NewUserID(vars["userID"], false)
			if err != nil {
				return util.JSONResponse{
					Code: http.StatusBadRequest,
					JSON: spec.InvalidUsername("Username was invalid"),
				}
			}
			return GetTransactionFromRelay(httpReq, request, relayAPI, *userID)
		},
	)).Methods(http.MethodGet, http.MethodOptions)
}

// MakeRelayAPI makes an http.Handler that checks matrix relay authentication.
func MakeRelayAPI(
	metricsName string, serverName spec.ServerName,
	isLocalServerName func(spec.ServerName) bool,
	keyRing gomatrixserverlib.JSONVerifier,
	f func(*http.Request, *fclient.FederationRequest, map[string]string) util.JSONResponse,
) http.Handler {
	h := func(req *http.Request) util.JSONResponse {
		fedReq, errResp := fclient.VerifyHTTPRequest(
			req, time.Now(), serverName, isLocalServerName, keyRing,
		)
		if fedReq == nil {
			return errResp
		}
		// add the user to Sentry, if enabled
		hub := sentry.GetHubFromContext(req.Context())
		if hub != nil {
			// clone the hub, so we don't send garbage events with e.g. mismatching rooms/event_ids
			hub = hub.Clone()
			hub.Scope().SetTag("origin", string(fedReq.Origin()))
			hub.Scope().SetTag("uri", fedReq.RequestURI())
		}
		defer func() {
			if r := recover(); r != nil {
				if hub != nil {
					hub.CaptureException(fmt.Errorf("%s panicked", req.URL.Path))
				}
				// re-panic to return the 500
				panic(r)
			}
		}()
		vars, err := httputil.URLDecodeMapValues(mux.Vars(req))
		if err != nil {
			return util.MatrixErrorResponse(400, string(spec.ErrorUnrecognized), "badly encoded query params")
		}

		jsonRes := f(req, fedReq, vars)
		// do not log 4xx as errors as they are client fails, not server fails
		if hub != nil && jsonRes.Code >= 500 {
			hub.Scope().SetExtra("response", jsonRes)
			hub.CaptureException(fmt.Errorf("%s returned HTTP %d", req.URL.Path, jsonRes.Code))
		}
		return jsonRes
	}
	return httputil.MakeExternalAPI(metricsName, h)
}
