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
	"net/http"

	"github.com/cs3org/reva/pkg/rhttp/utils"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type HttpMiddlewarer interface {
	SetMiddleware(name string, prefix string)
	Middleware(h http.Handler) http.Handler
}

type HttpMiddleware struct {
	middleware func(http.Handler) http.Handler
}

func (m *HttpMiddleware) SetMiddleware(name string, prefix string) {
	m.middleware = func(h http.Handler) http.Handler {
		return otelhttp.NewHandler(h, prefix,
			otelhttp.WithTracerProvider(tr.tracerProvider(name)),
			otelhttp.WithPropagators(tr.prop),
		)
	}
}

func (m *HttpMiddleware) Middleware(h http.Handler) http.Handler {
	return m.middleware(h)
}

func Middleware(h http.Handler, ms map[string]HttpMiddlewarer) http.Handler {
	handlers := map[string]http.Handler{}
	for prefix, m := range ms {
		handlers[prefix] = m.Middleware(h)
	}

	noopHandler := otelhttp.NewHandler(h, "",
		otelhttp.WithTracerProvider(tr.noop),
		otelhttp.WithPropagators(tr.prop),
	)

	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h, ok := handlers[r.URL.Path]; ok {
			h.ServeHTTP(w, r)
			return
		}

		var match string
		for prefix := range handlers {
			if utils.UrlHasPrefix(r.URL.Path, prefix) && len(prefix) > len(match) {
				match = prefix
			}
		}

		if h, ok := handlers[match]; ok {
			h.ServeHTTP(w, r)
			return
		}

		noopHandler.ServeHTTP(w, r)
	})
	return http.Handler(handlerFunc)
}
