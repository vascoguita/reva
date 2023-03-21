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

package appregistry

import (
	"context"

	registrypb "github.com/cs3org/go-cs3apis/cs3/app/registry/v1beta1"
	"github.com/cs3org/reva/pkg/app"
	"github.com/cs3org/reva/pkg/app/registry/registry"
	"github.com/cs3org/reva/pkg/errtypes"
	"github.com/cs3org/reva/pkg/rgrpc"
	"github.com/cs3org/reva/pkg/rgrpc/status"
	"github.com/cs3org/reva/pkg/tracing"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc"
)

const serviceName = "appregistry"
const tracerName = "appregistry"

func init() {
	rgrpc.Register(serviceName, New)
}

type svc struct {
	tracing.GrpcMiddleware
	reg app.Registry
}

func (s *svc) Close() error {
	return nil
}

func (s *svc) UnprotectedEndpoints() []string {
	return []string{"/cs3.app.registry.v1beta1.RegistryAPI/AddAppProvider", "/cs3.app.registry.v1beta1.RegistryAPI/ListSupportedMimeTypes"}
}

func (s *svc) Register(ss *grpc.Server) {
	registrypb.RegisterRegistryAPIServer(ss, s)
}

type config struct {
	Driver  string                            `mapstructure:"driver"`
	Drivers map[string]map[string]interface{} `mapstructure:"drivers"`
}

func (c *config) init() {
	if c.Driver == "" {
		c.Driver = "static"
	}
}

// New creates a new StorageRegistryService.
func New(m map[string]interface{}, ss *grpc.Server) (rgrpc.Service, error) {
	c, err := parseConfig(m)
	if err != nil {
		return nil, err
	}

	reg, err := getRegistry(c)
	if err != nil {
		return nil, err
	}

	svc := &svc{
		reg: reg,
	}

	return svc, nil
}

func parseConfig(m map[string]interface{}) (*config, error) {
	c := &config{}
	if err := mapstructure.Decode(m, c); err != nil {
		return nil, err
	}
	c.init()
	return c, nil
}

func getRegistry(c *config) (app.Registry, error) {
	if f, ok := registry.NewFuncs[c.Driver]; ok {
		return f(c.Drivers[c.Driver])
	}
	return nil, errtypes.NotFound("appregistrysvc: driver not found: " + c.Driver)
}

func (s *svc) GetAppProviders(ctx context.Context, req *registrypb.GetAppProvidersRequest) (*registrypb.GetAppProvidersResponse, error) {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "GetAppProviders")
	defer span.End()

	p, err := s.reg.FindProviders(ctx, req.ResourceInfo.MimeType)
	if err != nil {
		return &registrypb.GetAppProvidersResponse{
			Status: status.NewInternal(ctx, err, "error looking for the app provider"),
		}, nil
	}

	res := &registrypb.GetAppProvidersResponse{
		Status:    status.NewOK(ctx),
		Providers: p,
	}
	return res, nil
}

func (s *svc) AddAppProvider(ctx context.Context, req *registrypb.AddAppProviderRequest) (*registrypb.AddAppProviderResponse, error) {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "AddAppProvider")
	defer span.End()

	err := s.reg.AddProvider(ctx, req.Provider)
	if err != nil {
		return &registrypb.AddAppProviderResponse{
			Status: status.NewInternal(ctx, err, "error adding the app provider"),
		}, nil
	}

	res := &registrypb.AddAppProviderResponse{
		Status: status.NewOK(ctx),
	}
	return res, nil
}

func (s *svc) ListAppProviders(ctx context.Context, req *registrypb.ListAppProvidersRequest) (*registrypb.ListAppProvidersResponse, error) {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "ListAppProviders")
	defer span.End()

	providers, err := s.reg.ListProviders(ctx)
	if err != nil {
		return &registrypb.ListAppProvidersResponse{
			Status: status.NewInternal(ctx, err, "error listing the app providers"),
		}, nil
	}

	res := &registrypb.ListAppProvidersResponse{
		Status:    status.NewOK(ctx),
		Providers: providers,
	}
	return res, nil
}

func (s *svc) ListSupportedMimeTypes(ctx context.Context, req *registrypb.ListSupportedMimeTypesRequest) (*registrypb.ListSupportedMimeTypesResponse, error) {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "ListSupportedMimeTypes")
	defer span.End()

	mimeTypes, err := s.reg.ListSupportedMimeTypes(ctx)
	if err != nil {
		return &registrypb.ListSupportedMimeTypesResponse{
			Status: status.NewInternal(ctx, err, "error listing the supported mime types"),
		}, nil
	}

	// hide mimetypes for app providers
	for _, mime := range mimeTypes {
		for _, app := range mime.AppProviders {
			app.MimeTypes = nil
		}
	}

	res := &registrypb.ListSupportedMimeTypesResponse{
		Status:    status.NewOK(ctx),
		MimeTypes: mimeTypes,
	}
	return res, nil
}

func (s *svc) GetDefaultAppProviderForMimeType(ctx context.Context, req *registrypb.GetDefaultAppProviderForMimeTypeRequest) (*registrypb.GetDefaultAppProviderForMimeTypeResponse, error) {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "GetDefaultAppProviderForMimeType")
	defer span.End()

	provider, err := s.reg.GetDefaultProviderForMimeType(ctx, req.MimeType)
	if err != nil {
		return &registrypb.GetDefaultAppProviderForMimeTypeResponse{
			Status: status.NewInternal(ctx, err, "error getting the default app provider for the mimetype"),
		}, nil
	}

	res := &registrypb.GetDefaultAppProviderForMimeTypeResponse{
		Status:   status.NewOK(ctx),
		Provider: provider,
	}
	return res, nil
}

func (s *svc) SetDefaultAppProviderForMimeType(ctx context.Context, req *registrypb.SetDefaultAppProviderForMimeTypeRequest) (*registrypb.SetDefaultAppProviderForMimeTypeResponse, error) {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "SetDefaultAppProviderForMimeType")
	defer span.End()

	err := s.reg.SetDefaultProviderForMimeType(ctx, req.MimeType, req.Provider)
	if err != nil {
		return &registrypb.SetDefaultAppProviderForMimeTypeResponse{
			Status: status.NewInternal(ctx, err, "error setting the default app provider for the mimetype"),
		}, nil
	}

	res := &registrypb.SetDefaultAppProviderForMimeTypeResponse{
		Status: status.NewOK(ctx),
	}
	return res, nil
}
