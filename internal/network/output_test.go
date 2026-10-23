package network

import "testing"

func TestLooksBinary(t *testing.T) {
	cases := []struct {
		name        string
		body        []byte
		contentType string
		want        bool
	}{
		{"json", []byte(`{"ok":true}`), "application/json", false},
		{"plain text", []byte("hello\nworld\n"), "text/plain", false},
		{"text with no Content-Type", []byte("hello"), "", false},
		{"utf-8 accents", []byte("caf\u00e9"), "text/plain", false},
		{"utf-8 cjk and emoji", []byte("\u65e5\u672c\u8a9e \U0001F600"), "application/json", false},
		{"tabs and newlines", []byte("a\tb\r\nc"), "text/plain", false},
		{"png header", []byte("\x89PNG\r\n\x1a\n\x00\x00"), "image/png", true},
		{"png header, no Content-Type", []byte("\x89PNG\r\n\x1a\n"), "", true},
		{"gzip", []byte{0x1f, 0x8b, 0x08, 0x00}, "application/gzip", true},
		{"a NUL byte", []byte("ab\x00cd"), "text/plain", true},
		{"NUL in an otherwise valid body", []byte("{\"a\":1}\x00"), "application/json", true},
		{"latin-1 page sent as text", []byte("caf\xe9"), "text/html; charset=iso-8859-1", false},
		{"latin-1 bytes sent as octet-stream", []byte("caf\xe9"), "application/octet-stream", true},
		{"invalid utf-8 with no Content-Type", []byte("caf\xe9"), "", true},
		{"Content-Type is matched in any case", []byte("caf\xe9"), "  TEXT/Plain", false},
		{"empty", nil, "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := looksBinary(c.body, c.contentType); got != c.want {
				t.Errorf("looksBinary(%q, %q) = %v; want %v", c.body, c.contentType, got, c.want)
			}
		})
	}
}
