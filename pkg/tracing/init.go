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

package tracing

import (
	"fmt"
	"net"
	"sync"

	jaegerExporter "go.opentelemetry.io/otel/exporters/jaeger"
)

var initOnce sync.Once

func Init(v interface{}, l ...LoggerOption) {
	initOnce.Do(func() {
		initLogger(l...)
		log.Info().Msg("initializing tracing")

		c, err := newConfig(v)
		if err != nil {
			log.Error().Err(err).Msgf("error initializing tracing")
			return
		}

		var endpointOption jaegerExporter.EndpointOption
		switch {
		case c.Collector != "" && c.Agent != "":
			err := fmt.Errorf("more than one tracing endpoint option provided - agent: \"%s\", collector: \"%s\"", c.Agent, c.Collector)
			log.Error().Err(err).Msg("error initializing tracing")
			return
		case c.Agent != "":
			// Endpoint option to create a Jaeger exporter that sends spans to the Jaeger Agent
			// https://pkg.go.dev/go.opentelemetry.io/otel/exporters/jaeger#WithAgentEndpoint
			endpointOption, err = withAgentEndpoint(c.Agent)
			if err != nil {
				log.Error().Err(err).Msgf("error initializing tracing")
				return
			}
		case c.Collector != "":
			// Endpoint option to create a Jaeger exporter that sends spans
			// directly to the Jaeger Collector (without a Jaeger Agent in the middle)
			// https://pkg.go.dev/go.opentelemetry.io/otel/exporters/jaeger#WithCollectorEndpoint
			endpointOption = withCollectorEndpoint(c.Collector)
		default:
			log.Warn().Msg("tracing disabled - using NoopExporter")
			return
		}

		log.Info().Msg("creating jaegerExporter")
		exp, err := jaegerExporter.New(endpointOption)
		if err != nil {
			log.Error().Err(err).Msgf("error initializing tracing")
			return
		}
		tr.exp = exp
	})
}

func withAgentEndpoint(agent string) (jaegerExporter.EndpointOption, error) {
	log.Info().Msgf("creating jaegerExporter.EndpointOption for agent \"%s\"", agent)

	var options []jaegerExporter.AgentEndpointOption
	if agent != "" {
		host, port, err := net.SplitHostPort(agent)
		if err != nil {
			log.Error().Err(err).Msgf("error creating jaegerExporter.EndpointOption for agent \"%s\"", agent)
			return nil, err
		}
		// If the Jaeger Agent host address is not provided, "localhost" is used by default
		// https://github.com/open-telemetry/opentelemetry-go/blob/a50cf6aadd582f9760c578e2c4b5230b6c30913d/exporters/jaeger/uploader.go#L61
		if host != "" {
			option := jaegerExporter.WithAgentHost(host)
			options = append(options, option)
		}
		// If the Jaeger Agent host port is not provided, "6831" is used by default
		// https://github.com/open-telemetry/opentelemetry-go/blob/a50cf6aadd582f9760c578e2c4b5230b6c30913d/exporters/jaeger/uploader.go#L62
		if port != "" {
			option := jaegerExporter.WithAgentPort(port)
			options = append(options, option)
		}
	}
	return jaegerExporter.WithAgentEndpoint(options...), nil
}

func withCollectorEndpoint(collector string) jaegerExporter.EndpointOption {
	log.Info().Msgf("creating jaegerExporter.EndpointOption for collector \"%s\"", collector)

	var options []jaegerExporter.CollectorEndpointOption
	// If the Jaeger Collector URL is not provided, "http://localhost:14268/api/traces" is used by default
	// https://pkg.go.dev/go.opentelemetry.io/otel/exporters/jaeger#WithCollectorEndpoint
	if collector != "" {
		option := jaegerExporter.WithEndpoint(collector)
		options = append(options, option)
	}
	return jaegerExporter.WithCollectorEndpoint(options...)
}
