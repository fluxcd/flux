package websocket

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/weaveworks/fluxy"
)

func TestByteStream(t *testing.T) {
	buf := &bytes.Buffer{}
	upgrade := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(buf, ws); err != nil {
			t.Fatal(err)
		}
	})

	srv := httptest.NewServer(upgrade)
	defer srv.Close()

	url, _ := url.Parse(srv.URL)
	url.Scheme = "ws"

	ws, err := Dial(http.DefaultClient, flux.Token(""), url)
	if err != nil {
		t.Fatal(err)
	}
	ws.Write([]byte("hey"))
	ws.Write([]byte(" there"))
	ws.Write([]byte(" champ"))
	ws.Close()

	if buf.String() != "hey there champ" {
		t.Fatalf("did not collect message as expected, got %s", buf.String())
	}
}
