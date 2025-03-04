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

package ocdav

import (
	"fmt"
	"net/http"
	"path"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/cs3org/reva/pkg/appctx"
	"github.com/cs3org/reva/pkg/tracing"
	"github.com/rs/zerolog"
)

func (s *svc) handlePathDelete(w http.ResponseWriter, r *http.Request, ns string) {
	r, span := tracing.SpanStartFromRequest(r, tracerName, "handlePathDelete")
	defer span.End()

	fn := path.Join(ns, r.URL.Path)

	sublog := appctx.GetLogger(r.Context()).With().Str("path", fn).Logger()
	ref := &provider.Reference{Path: fn}
	s.handleDelete(w, r, ref, sublog)
}

func (s *svc) handleDelete(w http.ResponseWriter, r *http.Request, ref *provider.Reference, log zerolog.Logger) {
	r, span := tracing.SpanStartFromRequest(r, tracerName, "handleDelete")
	defer span.End()

	ctx := r.Context()
	client, err := s.getClient(ctx)
	if err != nil {
		log.Error().Err(err).Msg("error getting grpc client")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	req := &provider.DeleteRequest{Ref: ref}
	res, err := client.Delete(ctx, req)
	if err != nil {
		log.Error().Err(err).Msg("error performing delete grpc request")
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if res.Status.Code != rpc.Code_CODE_OK {
		if res.Status.Code == rpc.Code_CODE_NOT_FOUND {
			w.WriteHeader(http.StatusNotFound)
			// TODO path might be empty or relative...
			m := fmt.Sprintf("Resource %v not found", ref.Path)
			b, err := Marshal(exception{
				code:    SabredavNotFound,
				message: m,
			})
			HandleWebdavError(ctx, &log, w, b, err)
		}
		if res.Status.Code == rpc.Code_CODE_PERMISSION_DENIED {
			w.WriteHeader(http.StatusForbidden)
			// TODO path might be empty or relative...
			m := fmt.Sprintf("Permission denied to delete %v", ref.Path)
			b, err := Marshal(exception{
				code:    SabredavPermissionDenied,
				message: m,
			})
			HandleWebdavError(ctx, &log, w, b, err)
		}
		if res.Status.Code == rpc.Code_CODE_INTERNAL && res.Status.Message == "can't delete mount path" {
			w.WriteHeader(http.StatusForbidden)
			b, err := Marshal(exception{
				code:    SabredavPermissionDenied,
				message: res.Status.Message,
			})
			HandleWebdavError(ctx, &log, w, b, err)
		}

		HandleErrorStatus(ctx, &log, w, res.Status)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *svc) handleSpacesDelete(w http.ResponseWriter, r *http.Request, spaceID string) {
	r, span := tracing.SpanStartFromRequest(r, tracerName, "handleSpacesDelete")
	defer span.End()

	ctx := r.Context()
	sublog := appctx.GetLogger(ctx).With().Logger()
	// retrieve a specific storage space
	ref, rpcStatus, err := s.lookUpStorageSpaceReference(ctx, spaceID, r.URL.Path)
	if err != nil {
		sublog.Error().Err(err).Msg("error sending a grpc request")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if rpcStatus.Code != rpc.Code_CODE_OK {
		HandleErrorStatus(ctx, &sublog, w, rpcStatus)
		return
	}

	s.handleDelete(w, r, ref, sublog)
}
