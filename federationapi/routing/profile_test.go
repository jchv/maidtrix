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

package routing_test

import (
	"context"
	"encoding/hex"
	"io"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/mux"
	"github.com/jchv/maidtrix/clientapi/auth/authtypes"
	fedAPI "github.com/jchv/maidtrix/federationapi"
	"github.com/jchv/maidtrix/federationapi/routing"
	"github.com/jchv/maidtrix/internal/caching"
	"github.com/jchv/maidtrix/internal/httputil"
	"github.com/jchv/maidtrix/internal/matrixserver"
	"github.com/jchv/maidtrix/internal/matrixserver/fclient"
	"github.com/jchv/maidtrix/internal/matrixserver/spec"
	"github.com/jchv/maidtrix/internal/signing"
	"github.com/jchv/maidtrix/internal/sqlutil"
	"github.com/jchv/maidtrix/setup/jetstream"
	"github.com/jchv/maidtrix/test"
	"github.com/jchv/maidtrix/test/testrig"
	userAPI "github.com/jchv/maidtrix/userapi/api"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ed25519"
)

type fakeUserAPI struct {
	userAPI.FederationUserAPI
}

func (u *fakeUserAPI) QueryProfile(ctx context.Context, userID string) (*authtypes.Profile, error) {
	return &authtypes.Profile{}, nil
}

func TestHandleQueryProfile(t *testing.T) {
	test.WithAllDatabases(t, func(t *testing.T, dbType test.DBType) {
		cfg, processCtx, close := testrig.CreateConfig(t, dbType)
		cm := sqlutil.NewConnectionManager(processCtx, cfg.Global.DatabaseOptions)
		routers := httputil.NewRouters()
		defer close()

		fedMux := mux.NewRouter().SkipClean(true).PathPrefix(httputil.PublicFederationPathPrefix).Subrouter().UseEncodedPath()
		natsInstance := jetstream.NATSInstance{}
		routers.Federation = fedMux
		cfg.FederationAPI.Matrix.SigningIdentity.ServerName = testOrigin
		cfg.FederationAPI.Matrix.Metrics.Enabled = false
		fedClient := fakeFedClient{}
		serverKeyAPI := &signing.YggdrasilKeys{}
		keyRing := serverKeyAPI.KeyRing()
		fedapi := fedAPI.NewInternalAPI(processCtx, cfg, cm, &natsInstance, &fedClient, nil, nil, keyRing, true)
		userapi := fakeUserAPI{}

		routing.Setup(routers, cfg, nil, fedapi, keyRing, &fedClient, &userapi, &cfg.MSCs, nil, caching.DisableMetrics)

		handler := fedMux.Get(routing.QueryProfileRouteName).GetHandler().ServeHTTP
		_, sk, _ := ed25519.GenerateKey(nil)
		keyID := signing.KeyID
		pk := sk.Public().(ed25519.PublicKey)
		serverName := spec.ServerName(hex.EncodeToString(pk))
		req := fclient.NewFederationRequest("GET", serverName, testOrigin, "/query/profile?user_id="+url.QueryEscape("@user:"+string(testOrigin)))
		type queryContent struct{}
		content := queryContent{}
		err := req.SetContent(content)
		if err != nil {
			t.Fatalf("Error: %s", err.Error())
		}
		req.Sign(serverName, gomatrixserverlib.KeyID(keyID), sk)
		httpReq, err := req.HTTPRequest()
		if err != nil {
			t.Fatalf("Error: %s", err.Error())
		}
		// vars := map[string]string{"room_alias": "#room:server"}
		w := httptest.NewRecorder()
		// httpReq = mux.SetURLVars(httpReq, vars)
		handler(w, httpReq)

		res := w.Result()
		data, _ := io.ReadAll(res.Body)
		println(string(data))
		assert.Equal(t, 200, res.StatusCode)
	})
}
