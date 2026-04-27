package caldav

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Yandex CalDAV (and a few other servers) return ETag values without the
// surrounding double quotes that RFC 7232 requires. emersion/go-webdav refuses
// to parse those responses with `failed to unquote ETag: invalid syntax`.
//
// quoteETagTransport rewrites the multistatus XML body to wrap any unquoted
// `<DAV:getetag>...</DAV:getetag>` value in quotes so the parser stays happy.
// Already-quoted values, weak ETags (`W/"..."`), and empty bodies are left alone.
//
// This is a wire-level shim, not a fork — only the response body bytes are
// touched, never request bodies.
type quoteETagTransport struct {
	inner http.RoundTripper
}

func (t *quoteETagTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil || resp == nil {
		return resp, err
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(ct), "xml") || resp.Body == nil {
		return resp, nil
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	body = quoteRawETags(body)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return resp, nil
}

// Match `<{prefix:}getetag {attrs}>VALUE</{prefix:}getetag>` (case-insensitive).
// Group 1 = full opening tag minus brackets; group 2 = inner value; group 3 = closing tag name minus brackets.
var etagTagRE = regexp.MustCompile(`(?is)<([a-zA-Z0-9_:.-]*getetag(?:\s[^>]*)?)>([^<]*)</([a-zA-Z0-9_:.-]*getetag)>`)

// quoteRawETags scans the multistatus XML and wraps any unquoted ETag values in
// quotes. Already-quoted values and weak ETags are left untouched.
func quoteRawETags(body []byte) []byte {
	return etagTagRE.ReplaceAllFunc(body, func(match []byte) []byte {
		m := etagTagRE.FindSubmatch(match)
		open := m[1]
		raw := m[2]
		closeName := m[3]
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 {
			return match
		}
		if trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
			return match // already quoted
		}
		if len(trimmed) >= 3 && trimmed[0] == 'W' && trimmed[1] == '/' && trimmed[2] == '"' {
			return match // weak ETag in proper form
		}
		return []byte(fmt.Sprintf(`<%s>"%s"</%s>`, open, trimmed, closeName))
	})
}
