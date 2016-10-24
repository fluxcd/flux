package websocket

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/weaveworks/fluxy"
)

func TestByteStream(t *testing.T) {
	buf := &bytes.Buffer{}
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(buf, ws); err != nil {
			t.Fatal(err)
		}
	})

	go http.ListenAndServe(":15678", nil)

	url, _ := url.Parse("ws://127.0.0.1:15678/ws")
	ws, err := Dial(http.DefaultClient, flux.Token(""), url)
	if err != nil {
		t.Fatal(err)
	}
	ws.Write([]byte("hello"))
	ws.Write([]byte(" there"))
	ws.Write([]byte(" champ"))
	ws.Close()

	if buf.String() != "hello there champ" {
		t.Fatalf("did not collect message as expected, got %s", buf.String())
	}
}
