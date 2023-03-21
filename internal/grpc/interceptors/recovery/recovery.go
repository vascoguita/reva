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

package recovery

import (
	"context"
	"runtime/debug"

	"github.com/cs3org/reva/pkg/appctx"
	"github.com/cs3org/reva/pkg/tracing"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const tracerName = "recovery"

// NewUnary returns a server interceptor that adds telemetry to
// grpc calls.
func NewUnary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "recovery UnaryServerInterceptor")
		defer span.End()

		interceptor := grpc_recovery.UnaryServerInterceptor(grpc_recovery.WithRecoveryHandlerContext(recoveryFunc))
		return interceptor(ctx, req, info, handler)
	}
}

// NewStream returns a streaming server interceptor that adds telemetry to
// streaming grpc calls.
func NewStream() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()
		ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "recovery StreamServerInterceptor")
		defer span.End()

		interceptor := grpc_recovery.StreamServerInterceptor(grpc_recovery.WithRecoveryHandlerContext(recoveryFunc))
		return interceptor(srv, ss, info, handler)
	}
}

func recoveryFunc(ctx context.Context, p interface{}) (err error) {
	ctx, span := tracing.SpanStartFromContext(ctx, tracerName, "recovery recoveryFunc")
	defer span.End()

	debug.PrintStack()
	log := appctx.GetLogger(ctx)
	log.Error().Msgf("%+v; stack: %s", p, debug.Stack())
	return status.Errorf(codes.Internal, "%s", p)
}
