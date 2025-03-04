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

package authprovider

import (
	"context"
	"fmt"
	"path/filepath"

	provider "github.com/cs3org/go-cs3apis/cs3/auth/provider/v1beta1"
	"github.com/cs3org/reva/pkg/appctx"
	"github.com/cs3org/reva/pkg/auth"
	"github.com/cs3org/reva/pkg/auth/manager/registry"
	"github.com/cs3org/reva/pkg/errtypes"
	"github.com/cs3org/reva/pkg/plugin"
	"github.com/cs3org/reva/pkg/rgrpc"
	"github.com/cs3org/reva/pkg/rgrpc/status"
	"github.com/cs3org/reva/pkg/sharedconf"
	"github.com/cs3org/reva/pkg/tracing"
	"github.com/cs3org/reva/pkg/user"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

const serviceName = "authprovider"
const tracerName = "authprovider"

func init() {
	rgrpc.Register(serviceName, New)
}

type config struct {
	AuthManager  string                            `mapstructure:"auth_manager"`
	AuthManagers map[string]map[string]interface{} `mapstructure:"auth_managers"`
	blockedUsers []string
}

func (c *config) init() {
	if c.AuthManager == "" {
		c.AuthManager = "json"
	}
	c.blockedUsers = sharedconf.GetBlockedUsers()
}

type service struct {
	tracing.GrpcMiddleware
	authmgr      auth.Manager
	conf         *config
	plugin       *plugin.RevaPlugin
	blockedUsers user.BlockedUsers
}

func parseConfig(m map[string]interface{}) (*config, error) {
	c := &config{}
	if err := mapstructure.Decode(m, c); err != nil {
		err = errors.Wrap(err, "error decoding conf")
		return nil, err
	}
	c.init()
	return c, nil
}

func getAuthManager(manager string, m map[string]map[string]interface{}) (auth.Manager, *plugin.RevaPlugin, error) {
	if manager == "" {
		return nil, nil, errtypes.InternalError("authsvc: driver not configured for auth manager")
	}
	p, err := plugin.Load("authprovider", manager)
	if err == nil {
		authManager, ok := p.Plugin.(auth.Manager)
		if !ok {
			return nil, nil, fmt.Errorf("could not assert the loaded plugin")
		}
		pluginConfig := filepath.Base(manager)
		err = authManager.Configure(m[pluginConfig])
		if err != nil {
			return nil, nil, err
		}
		return authManager, p, nil
	} else if _, ok := err.(errtypes.NotFound); ok {
		if f, ok := registry.NewFuncs[manager]; ok {
			authmgr, err := f(m[manager])
			return authmgr, nil, err
		}
	} else {
		return nil, nil, err
	}
	return nil, nil, errtypes.NotFound(fmt.Sprintf("authsvc: driver %s not found for auth manager", manager))
}

// New returns a new AuthProviderServiceServer.
func New(m map[string]interface{}, ss *grpc.Server) (rgrpc.Service, error) {
	c, err := parseConfig(m)
	if err != nil {
		return nil, err
	}

	authManager, plug, err := getAuthManager(c.AuthManager, c.AuthManagers)
	if err != nil {
		return nil, err
	}

	svc := &service{
		conf:         c,
		authmgr:      authManager,
		plugin:       plug,
		blockedUsers: user.NewBlockedUsersSet(c.blockedUsers),
	}

	return svc, nil
}

func (s *service) Close() error {
	if s.plugin != nil {
		s.plugin.Kill()
	}
	return nil
}

func (s *service) UnprotectedEndpoints() []string {
	return []string{"/cs3.auth.provider.v1beta1.ProviderAPI/Authenticate"}
}

func (s *service) Register(ss *grpc.Server) {
	provider.RegisterProviderAPIServer(ss, s)
}

func (s *service) Authenticate(ctx context.Context, req *provider.AuthenticateRequest) (*provider.AuthenticateResponse, error) {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "Authenticate")
	defer span.End()

	log := appctx.GetLogger(ctx)
	username := req.ClientId
	password := req.ClientSecret

	if s.blockedUsers.IsBlocked(username) {
		return &provider.AuthenticateResponse{
			Status: status.NewPermissionDenied(ctx, errtypes.PermissionDenied(""), "user is blocked"),
		}, nil
	}

	u, scope, err := s.authmgr.Authenticate(ctx, username, password)
	switch v := err.(type) {
	case nil:
		log.Info().Interface("userId", u.Id).Msg("user authenticated")
		return &provider.AuthenticateResponse{
			Status:     status.NewOK(ctx),
			User:       u,
			TokenScope: scope,
		}, nil
	case errtypes.InvalidCredentials:
		return &provider.AuthenticateResponse{
			Status: status.NewPermissionDenied(ctx, v, "wrong password"),
		}, nil
	case errtypes.NotFound:
		return &provider.AuthenticateResponse{
			Status: status.NewNotFound(ctx, "unknown client id"),
		}, nil
	default:
		err = errors.Wrap(err, "authsvc: error in Authenticate")
		return &provider.AuthenticateResponse{
			Status: status.NewUnauthenticated(ctx, err, "error authenticating user"),
		}, nil
	}
}
