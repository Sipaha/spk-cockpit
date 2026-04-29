//go:build wails

package desktop

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// udsMiddleware proxies any /api/* request from the embedded webview onto the
// daemon's Unix domain socket. Non-/api paths fall through to the asset
// handler. The signature matches v2's assetserver.Middleware exactly
// (`func(next http.Handler) http.Handler`), so the body is identical.
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
			if len(r.URL.Path) >= 5 && r.URL.Path[:5] == "/api/" {
				proxy.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
