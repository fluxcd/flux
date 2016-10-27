package websocket

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/weaveworks/fluxy"
)

func TestToken(t *testing.T) {
	token := "toooookkkkkeeeeennnnnn"
	upgrade := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := r.Header.Get("Authorization")
		if tok != "Scope-Probe token="+token {
			t.Fatal("Did not get authorisation header, got: " + tok)
		}
		_, err := Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
	})

	srv := httptest.NewServer(upgrade)
	defer srv.Close()

	url, _ := url.Parse(srv.URL)
	url.Scheme = "ws"

	ws, err := Dial(http.DefaultClient, flux.Token(token), url)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
}

func TestByteStream(t *testing.T) {
	mx := sync.Mutex{}
	buf := &bytes.Buffer{}
	upgrade := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		mx.Lock()
		defer mx.Unlock()
		if _, err := io.Copy(buf, ws); err != nil {
			t.Fatal(err)
		}
	})

	srv := httptest.NewServer(upgrade)

	url, _ := url.Parse(srv.URL)
	url.Scheme = "ws"

	ws, err := Dial(http.DefaultClient, flux.Token(""), url)
	if err != nil {
		t.Fatal(err)
	}

	checkWrite := func(msg string) {
		if _, err := ws.Write([]byte(msg)); err != nil {
			t.Fatal(err)
		}
	}

	checkWrite("hey")
	checkWrite(" there")
	checkWrite(" champ")
	if err := ws.Close(); err != nil {
		t.Fatal(err)
	}

	// Make sure the server reads everything from the connection
	srv.Close()
	mx.Lock()
	defer mx.Unlock()
	if buf.String() != "hey there champ" {
		t.Fatalf("did not collect message as expected, got %s", buf.String())
	}
}
