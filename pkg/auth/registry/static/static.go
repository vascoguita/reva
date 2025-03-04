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

package static

import (
	"context"

	registrypb "github.com/cs3org/go-cs3apis/cs3/auth/registry/v1beta1"
	"github.com/cs3org/reva/pkg/auth"
	"github.com/cs3org/reva/pkg/auth/registry/registry"
	"github.com/cs3org/reva/pkg/errtypes"
	"github.com/cs3org/reva/pkg/sharedconf"
	"github.com/cs3org/reva/pkg/tracing"
	"github.com/mitchellh/mapstructure"
)

const tracerName = "static"

func init() {
	registry.Register("static", New)
}

type config struct {
	Rules map[string]string `mapstructure:"rules"`
}

func (c *config) init() {
	if len(c.Rules) == 0 {
		c.Rules = map[string]string{
			"basic": sharedconf.GetGatewaySVC(""),
		}
	}
}

type reg struct {
	rules map[string]string
}

func (r *reg) ListProviders(ctx context.Context) ([]*registrypb.ProviderInfo, error) {
	_, span := tracing.SpanStartFromContext(ctx, tracerName, "ListProviders")
	defer span.End()

	providers := make([]*registrypb.ProviderInfo, len(r.rules))
	for k, v := range r.rules {
		providers = append(providers, &registrypb.ProviderInfo{
			ProviderType: k,
			Address:      v,
		})
	}
	return providers, nil
}

func (r *reg) GetProvider(ctx context.Context, authType string) (*registrypb.ProviderInfo, error) {
	_, span := tracing.SpanStartFromContext(ctx, tracerName, "GetProvider")
	defer span.End()

	if address, ok := r.rules[authType]; ok {
		return &registrypb.ProviderInfo{
			ProviderType: authType,
			Address:      address,
		}, nil
	}
	return nil, errtypes.NotFound("static: auth type not found: " + authType)
}

func parseConfig(m map[string]interface{}) (*config, error) {
	c := &config{}
	if err := mapstructure.Decode(m, c); err != nil {
		return nil, err
	}
	return c, nil
}

// New returns an implementation of the auth.Registry interface.
func New(m map[string]interface{}) (auth.Registry, error) {
	c, err := parseConfig(m)
	if err != nil {
		return nil, err
	}
	c.init()
	return &reg{rules: c.Rules}, nil
}
