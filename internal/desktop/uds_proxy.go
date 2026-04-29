//go:build wails

package desktop

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// udsMiddleware proxies any /api/* request from the embedded webview onto the
// daemon's Unix domain socket. Non-/api paths fall through to the asset
// handler.
//
// The middleware buffers the request body before handing it to
// httputil.ReverseProxy. v3 alpha.78's WebKit2GTK URI-scheme handler exposes
// the body as a GInputStream wrapper whose readability beyond the synchronous
// portion of the scheme-request callback is unreliable (POST/PUT/PATCH JSON
// bodies arrive at the daemon empty when the proxy streams them). Reading
// the full body up-front and replacing r.Body with a bytes.Reader makes the
// forward deterministic at the cost of buffering the payload — fine for our
// JSON-shaped /api/* traffic.
func udsMiddleware(socketPath string) application.Middleware {
	transport := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}
	target, _ := url.Parse("http://unix")
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
		},
		Transport: transport,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}
			if r.Body != nil && r.Body != http.NoBody {
				buf, err := io.ReadAll(r.Body)
				_ = r.Body.Close()
				if err != nil {
					http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(buf))
				r.ContentLength = int64(len(buf))
			}
			proxy.ServeHTTP(w, r)
		})
	}
}
