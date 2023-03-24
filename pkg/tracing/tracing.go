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
	"os"
	"sync"

	jaegerPropagator "go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
	"go.opentelemetry.io/otel/trace"
)

var tr *tracing

type tracing struct {
	exp  tracesdk.SpanExporter
	prop jaegerPropagator.Jaeger
	noop trace.TracerProvider
	reg  sync.Map
	mux  sync.Mutex
}

func init() {
	tr = &tracing{
		noop: trace.NewNoopTracerProvider(),
		exp:  tracetest.NewNoopExporter(),
		prop: jaegerPropagator.Jaeger{},
	}
}

func (t *tracing) tracerProvider(name string) trace.TracerProvider {
	t.mux.Lock()
	defer t.mux.Unlock()

	if value, ok := t.reg.Load(name); ok {
		if tp, ok := value.(trace.TracerProvider); ok {
			return tp
		}
	}

	var tp = t.noop

	hostname, err := os.Hostname()
	if err != nil {
		t.reg.Store(name, tp)
		return tp
	}

	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(name),
			semconv.HostNameKey.String(hostname),
		),
	)
	if err != nil {
		t.reg.Store(name, tp)
		return tp
	}

	tp = tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(t.exp),
		tracesdk.WithResource(r),
	)
	t.reg.Store(name, tp)
	return tp
}
