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

package appctx

import (
	"context"

	"github.com/cs3org/reva/pkg/appctx"
	"github.com/cs3org/reva/pkg/tracing"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

const tracerName = "appctx"

// NewUnary returns a new unary interceptor that creates the application context.
func NewUnary(log zerolog.Logger) grpc.UnaryServerInterceptor {
	interceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "appctx UnaryServerInterceptor")
		defer span.End()

		sub := log.With().Str("TraceID", span.SpanContext().TraceID().String()).Logger()
		ctx = appctx.WithLogger(ctx, &sub)
		res, err := handler(ctx, req)
		return res, err
	}
	return interceptor
}

// NewStream returns a new server stream interceptor
// that creates the application context.
func NewStream(log zerolog.Logger) grpc.StreamServerInterceptor {
	interceptor := func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "appctx StreamServerInterceptor")
		defer span.End()

		sub := log.With().Str("TraceID", span.SpanContext().TraceID().String()).Logger()
		ctx = appctx.WithLogger(ctx, &sub)

		wrapped := newWrappedServerStream(ctx, ss)
		return handler(srv, wrapped)
	}
	return interceptor
}

func newWrappedServerStream(ctx context.Context, ss grpc.ServerStream) *wrappedServerStream {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "appctx newWrappedServerStream")
	defer span.End()
	return &wrappedServerStream{ServerStream: ss, newCtx: ctx}
}

type wrappedServerStream struct {
	grpc.ServerStream
	newCtx context.Context
}

func (ss *wrappedServerStream) Context() context.Context {
	return ss.newCtx
}
