// Copyright 2018-2021 CERN
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
	"context"
	"net/http"
	"sync"

	"go.opentelemetry.io/otel/trace"
)

var mu sync.Mutex

func spanStart(ctx context.Context, tp trace.TracerProvider, tracerName string, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	mu.Lock()
	defer mu.Unlock()
	return tp.Tracer(tracerName).Start(ctx, spanName, opts...)
}

func SpanStartFromContext(ctx context.Context, tracerName string, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	span := trace.SpanFromContext(ctx)
	tp := span.TracerProvider()
	return spanStart(ctx, tp, tracerName, spanName, opts...)
}

func SpanStartFromRequest(r *http.Request, tracerName string, spanName string, opts ...trace.SpanStartOption) (*http.Request, trace.Span) {
	ctx := r.Context()
	ctx, span := SpanStartFromContext(ctx, tracerName, spanName, opts...)
	r = r.WithContext(ctx)
	return r, span
}

func SpanStart(ctx context.Context, serviceName string, tracerName string, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tp := tr.tracerProvider(serviceName)
	return spanStart(ctx, tp, tracerName, spanName, opts...)
}
