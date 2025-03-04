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

package sciencemesh

import (
	"context"
	"net/http"

	"github.com/cs3org/reva/pkg/appctx"
	"github.com/cs3org/reva/pkg/rhttp/global"
	"github.com/cs3org/reva/pkg/sharedconf"
	"github.com/cs3org/reva/pkg/smtpclient"
	"github.com/cs3org/reva/pkg/tracing"
	"github.com/go-chi/chi/v5"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog"
)

const serviceName = "sciencemesh"
const tracerName = "sciencemesh"

func init() {
	global.Register("sciencemesh", New)
}

// New returns a new sciencemesh service.
func New(m map[string]interface{}, log *zerolog.Logger) (global.Service, error) {
	ctx, span := tracing.SpanStart(context.Background(), serviceName, tracerName, "New")
	defer span.End()

	conf := &config{}
	if err := mapstructure.Decode(m, conf); err != nil {
		return nil, err
	}

	conf.init()

	r := chi.NewRouter()
	s := &svc{
		conf:   conf,
		router: r,
	}

	if err := s.routerInit(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

// Close performs cleanup.
func (s *svc) Close() error {
	return nil
}

type config struct {
	Prefix             string                      `mapstructure:"prefix"`
	SMTPCredentials    *smtpclient.SMTPCredentials `mapstructure:"smtp_credentials"`
	GatewaySvc         string                      `mapstructure:"gatewaysvc"`
	MeshDirectoryURL   string                      `mapstructure:"mesh_directory_url"`
	ProviderDomain     string                      `mapstructure:"provider_domain"`
	SubjectTemplate    string                      `mapstructure:"subject_template"`
	BodyTemplatePath   string                      `mapstructure:"body_template_path"`
	OCMMountPoint      string                      `mapstructure:"ocm_mount_point"`
	InviteLinkTemplate string                      `mapstructure:"invite_link_template"`
}

func (c *config) init() {
	if c.Prefix == "" {
		c.Prefix = "sciencemesh"
	}

	c.GatewaySvc = sharedconf.GetGatewaySVC(c.GatewaySvc)
}

type svc struct {
	tracing.HTTPMiddleware
	conf   *config
	router chi.Router
}

func (s *svc) routerInit(ctx context.Context) error {
	tokenHandler := new(tokenHandler)
	if err := tokenHandler.init(ctx, s.conf); err != nil {
		return err
	}
	providersHandler := new(providersHandler)
	if err := providersHandler.init(ctx, s.conf); err != nil {
		return err
	}

	appsHandler := new(appsHandler)
	if err := appsHandler.init(ctx, s.conf); err != nil {
		return err
	}

	s.router.Get("/generate-invite", tokenHandler.Generate)
	s.router.Get("/list-invite", tokenHandler.ListInvite)
	s.router.Post("/accept-invite", tokenHandler.AcceptInvite)
	s.router.Get("/find-accepted-users", tokenHandler.FindAccepted)
	s.router.Get("/list-providers", providersHandler.ListProviders)
	s.router.Post("/open-in-app", appsHandler.OpenInApp)

	return nil
}

func (s *svc) Prefix() string {
	return s.conf.Prefix
}

func (s *svc) Unprotected() []string {
	return nil
}

func (s *svc) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := appctx.GetLogger(r.Context())
		log.Debug().Str("path", r.URL.Path).Msg("sciencemesh routing")

		// unset raw path, otherwise chi uses it to route and then fails to match percent encoded path segments
		r.URL.RawPath = ""
		s.router.ServeHTTP(w, r)
	})
}
