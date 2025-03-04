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

package ocmproviderauthorizer

import (
	"context"

	ocmprovider "github.com/cs3org/go-cs3apis/cs3/ocm/provider/v1beta1"
	"github.com/cs3org/reva/pkg/errtypes"
	"github.com/cs3org/reva/pkg/ocm/provider"
	"github.com/cs3org/reva/pkg/ocm/provider/authorizer/registry"
	"github.com/cs3org/reva/pkg/rgrpc"
	"github.com/cs3org/reva/pkg/rgrpc/status"
	"github.com/cs3org/reva/pkg/tracing"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

const tracerName = "ocmproviderauthorizer"

func init() {
	rgrpc.Register("ocmproviderauthorizer", New)
}

type config struct {
	Driver  string                            `mapstructure:"driver"`
	Drivers map[string]map[string]interface{} `mapstructure:"drivers"`
}

type service struct {
	tracing.GrpcMiddleware
	conf *config
	pa   provider.Authorizer
}

func (c *config) init() {
	if c.Driver == "" {
		c.Driver = "json"
	}
}

func (s *service) Register(ss *grpc.Server) {
	ocmprovider.RegisterProviderAPIServer(ss, s)
}

func getProviderAuthorizer(c *config) (provider.Authorizer, error) {
	if f, ok := registry.NewFuncs[c.Driver]; ok {
		return f(c.Drivers[c.Driver])
	}
	return nil, errtypes.NotFound("driver not found: " + c.Driver)
}

func parseConfig(m map[string]interface{}) (*config, error) {
	c := &config{}
	if err := mapstructure.Decode(m, c); err != nil {
		err = errors.Wrap(err, "error decoding conf")
		return nil, err
	}
	return c, nil
}

// New creates a new OCM provider authorizer svc.
func New(m map[string]interface{}, ss *grpc.Server) (rgrpc.Service, error) {
	c, err := parseConfig(m)
	if err != nil {
		return nil, err
	}
	c.init()

	pa, err := getProviderAuthorizer(c)
	if err != nil {
		return nil, err
	}

	service := &service{
		conf: c,
		pa:   pa,
	}
	return service, nil
}

func (s *service) Close() error {
	return nil
}

func (s *service) UnprotectedEndpoints() []string {
	return []string{
		"/cs3.ocm.provider.v1beta1.ProviderAPI/IsProviderAllowed",
		"/cs3.ocm.provider.v1beta1.ProviderAPI/ListAllProviders",
	}
}

func (s *service) GetInfoByDomain(ctx context.Context, req *ocmprovider.GetInfoByDomainRequest) (*ocmprovider.GetInfoByDomainResponse, error) {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "GetInfoByDomain")
	defer span.End()

	domainInfo, err := s.pa.GetInfoByDomain(ctx, req.Domain)
	if err != nil {
		return &ocmprovider.GetInfoByDomainResponse{
			Status: status.NewInternal(ctx, err, "error getting provider info"),
		}, nil
	}

	return &ocmprovider.GetInfoByDomainResponse{
		Status:       status.NewOK(ctx),
		ProviderInfo: domainInfo,
	}, nil
}

func (s *service) IsProviderAllowed(ctx context.Context, req *ocmprovider.IsProviderAllowedRequest) (*ocmprovider.IsProviderAllowedResponse, error) {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "IsProviderAllowed")
	defer span.End()

	err := s.pa.IsProviderAllowed(ctx, req.Provider)
	if err != nil {
		return &ocmprovider.IsProviderAllowedResponse{
			Status: status.NewInternal(ctx, err, "error verifying mesh provider"),
		}, nil
	}

	return &ocmprovider.IsProviderAllowedResponse{
		Status: status.NewOK(ctx),
	}, nil
}

func (s *service) ListAllProviders(ctx context.Context, req *ocmprovider.ListAllProvidersRequest) (*ocmprovider.ListAllProvidersResponse, error) {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "ListAllProviders")
	defer span.End()

	providers, err := s.pa.ListAllProviders(ctx)
	if err != nil {
		return &ocmprovider.ListAllProvidersResponse{
			Status: status.NewInternal(ctx, err, "error retrieving mesh providers"),
		}, nil
	}

	return &ocmprovider.ListAllProvidersResponse{
		Status:    status.NewOK(ctx),
		Providers: providers,
	}, nil
}
