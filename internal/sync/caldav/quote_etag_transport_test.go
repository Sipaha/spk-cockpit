package caldav

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuoteRawETags_AddsQuotesToBare(t *testing.T) {
	in := []byte(`<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:propstat><d:prop>
      <d:getetag>4a3b1c7e</d:getetag>
    </d:prop></d:propstat>
  </d:response>
</d:multistatus>`)
	out := string(quoteRawETags(in))
	require.Contains(t, out, `<d:getetag>"4a3b1c7e"</d:getetag>`)
}

func TestQuoteRawETags_LeavesQuotedAlone(t *testing.T) {
	in := []byte(`<getetag>"abc"</getetag>`)
	out := string(quoteRawETags(in))
	require.Equal(t, `<getetag>"abc"</getetag>`, out)
}

func TestQuoteRawETags_LeavesWeakAlone(t *testing.T) {
	in := []byte(`<getetag>W/"abc"</getetag>`)
	out := string(quoteRawETags(in))
	require.Equal(t, `<getetag>W/"abc"</getetag>`, out)
}

func TestQuoteRawETags_EmptyTagSafe(t *testing.T) {
	in := []byte(`<getetag></getetag>`)
	out := string(quoteRawETags(in))
	require.Equal(t, `<getetag></getetag>`, out)
}

func TestQuoteRawETags_HandlesMultiple(t *testing.T) {
	in := []byte(`<a:getetag>x1</a:getetag>...<a:getetag>x2</a:getetag>`)
	out := string(quoteRawETags(in))
	require.Equal(t, 2, strings.Count(out, `"x1"`)+strings.Count(out, `"x2"`))
}
