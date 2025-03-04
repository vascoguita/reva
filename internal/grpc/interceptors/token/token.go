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

package token

import (
	"context"

	ctxpkg "github.com/cs3org/reva/pkg/ctx"
	"github.com/cs3org/reva/pkg/tracing"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const tracerName = "token"

// NewUnary returns a new unary interceptor that adds
// the token to the context.
func NewUnary() grpc.UnaryServerInterceptor {
	interceptor := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "token UnaryServerInterceptor")
		defer span.End()

		md, ok := metadata.FromIncomingContext(ctx)
		if ok && md != nil {
			if val, ok := md[ctxpkg.TokenHeader]; ok {
				if len(val) > 0 && val[0] != "" {
					tkn := val[0]
					ctx = ctxpkg.ContextSetToken(ctx, tkn)
					ctx = metadata.AppendToOutgoingContext(ctx, ctxpkg.TokenHeader, tkn)
				}
			}
		}

		return handler(ctx, req)
	}
	return interceptor
}

// NewStream returns a new server stream interceptor
// that adds trace information to the request.
func NewStream() grpc.StreamServerInterceptor {
	interceptor := func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "token StreamServerInterceptor")
		defer span.End()

		md, ok := metadata.FromIncomingContext(ss.Context())
		if ok && md != nil {
			if val, ok := md[ctxpkg.TokenHeader]; ok {
				if len(val) > 0 && val[0] != "" {
					tkn := val[0]
					ctx = ctxpkg.ContextSetToken(ctx, tkn)
					ctx = metadata.AppendToOutgoingContext(ctx, ctxpkg.TokenHeader, tkn)
				}
			}
		}

		wrapped := newWrappedServerStream(ctx, ss)
		return handler(srv, wrapped)
	}
	return interceptor
}

func newWrappedServerStream(ctx context.Context, ss grpc.ServerStream) *wrappedServerStream {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "token newWrappedServerStream")
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
