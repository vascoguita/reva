// Copyright 2018-2023 CERN
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
//
// In applying this license, CERN does not waive the privileges and immunities
// granted to it by virtue of its status as an Intergovernmental Organization
// or submit itself to any jurisdiction.

package preferences

import (
	"encoding/json"
	"net/http"

	preferences "github.com/cs3org/go-cs3apis/cs3/preferences/v1beta1"
	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	"github.com/cs3org/reva/pkg/appctx"
	"github.com/cs3org/reva/pkg/rgrpc/todo/pool"
	"github.com/cs3org/reva/pkg/rhttp/global"
	"github.com/cs3org/reva/pkg/sharedconf"
	"github.com/cs3org/reva/pkg/tracing"
	"github.com/go-chi/chi/v5"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog"
)

const serviceName = "preferences"
const tracerName = "preferences"

func init() {
	global.Register("preferences", New)
}

// Config holds the config options that for the preferences HTTP service.
type Config struct {
	Prefix     string `mapstructure:"prefix"`
	GatewaySvc string `mapstructure:"gatewaysvc"`
}

func (c *Config) init() {
	if c.Prefix == "" {
		c.Prefix = "preferences"
	}
	c.GatewaySvc = sharedconf.GetGatewaySVC(c.GatewaySvc)
}

type svc struct {
	tracing.HTTPMiddleware
	conf   *Config
	router *chi.Mux
}

// New returns a new ocmd object.
func New(m map[string]interface{}, log *zerolog.Logger) (global.Service, error) {
	conf := &Config{}
	if err := mapstructure.Decode(m, conf); err != nil {
		return nil, err
	}
	conf.init()

	r := chi.NewRouter()
	s := &svc{
		conf:   conf,
		router: r,
	}

	if err := s.routerInit(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *svc) routerInit() error {
	s.router.Get("/", s.handleGet)
	s.router.Post("/", s.handlePost)
	return nil
}

// Close performs cleanup.
func (s *svc) Close() error {
	return nil
}

func (s *svc) Prefix() string {
	return s.conf.Prefix
}

func (s *svc) Unprotected() []string {
	return []string{}
}

func (s *svc) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r, span := tracing.SpanStartFromRequest(r, tracerName, "Preferences Service HTTP Handler")
		defer span.End()

		s.router.ServeHTTP(w, r)
	})
}

func (s *svc) handleGet(w http.ResponseWriter, r *http.Request) {
	r, span := tracing.SpanStartFromRequest(r, tracerName, "handleGet")
	defer span.End()

	ctx := r.Context()
	log := appctx.GetLogger(ctx)

	key := r.URL.Query().Get("key")
	ns := r.URL.Query().Get("ns")

	if key == "" || ns == "" {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("key or namespace query missing")); err != nil {
			log.Error().Err(err).Msg("error writing to response")
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	client, err := pool.GetGatewayServiceClient(ctx, pool.Endpoint(s.conf.GatewaySvc))
	if err != nil {
		log.Error().Err(err).Msg("error getting grpc gateway client")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	res, err := client.GetKey(ctx, &preferences.GetKeyRequest{
		Key: &preferences.PreferenceKey{
			Namespace: ns,
			Key:       key,
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("error retrieving key")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		if res.Status.Code == rpc.Code_CODE_NOT_FOUND {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		log.Error().Interface("status", res.Status).Msg("error retrieving key")
		return
	}

	js, err := json.Marshal(map[string]interface{}{
		"namespace": ns,
		"key":       key,
		"value":     res.Val,
	})
	if err != nil {
		log.Error().Err(err).Msg("error marshalling response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(js); err != nil {
		log.Error().Err(err).Msg("error writing JSON response")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *svc) handlePost(w http.ResponseWriter, r *http.Request) {
	r, span := tracing.SpanStartFromRequest(r, tracerName, "handlePost")
	defer span.End()

	ctx := r.Context()
	log := appctx.GetLogger(ctx)

	key := r.FormValue("key")
	ns := r.FormValue("ns")
	val := r.FormValue("value")

	if key == "" || ns == "" || val == "" {
		w.WriteHeader(http.StatusBadRequest)
		if _, err := w.Write([]byte("key, namespace or value parameter missing")); err != nil {
			log.Error().Err(err).Msg("error writing to response")
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	client, err := pool.GetGatewayServiceClient(ctx, pool.Endpoint(s.conf.GatewaySvc))
	if err != nil {
		log.Error().Err(err).Msg("error getting grpc gateway client")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	res, err := client.SetKey(ctx, &preferences.SetKeyRequest{
		Key: &preferences.PreferenceKey{
			Namespace: ns,
			Key:       key,
		},
		Val: val,
	})
	if err != nil {
		log.Error().Err(err).Msg("error setting key")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error().Interface("status", res.Status).Msg("error setting key")
		return
	}
}
