package websocket

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/fluxcd/flux/pkg/http/client"
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

	ws, err := Dial(http.DefaultClient, "fluxd/test", client.Token(token), url)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
}

func TestByteStream(t *testing.T) {
	buf := &bytes.Buffer{}
	var wg sync.WaitGroup
	wg.Add(1)
	upgrade := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(buf, ws); err != nil {
			t.Fatal(err)
		}
		wg.Done()
	})

	srv := httptest.NewServer(upgrade)

	url, _ := url.Parse(srv.URL)
	url.Scheme = "ws"

	ws, err := Dial(http.DefaultClient, "fluxd/test", client.Token(""), url)
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
	wg.Wait()
	if buf.String() != "hey there champ" {
		t.Fatalf("did not collect message as expected, got %s", buf.String())
	}
}
