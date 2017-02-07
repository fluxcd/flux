package github

import (
	"fmt"
	gh "github.com/google/go-github/github"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

var (
	// mux is the HTTP request multiplexer used with the test server.
	mux *http.ServeMux

	// client is the GitHub client being tested.
	client *gh.Client

	// server is a test HTTP server used to provide mock API responses.
	server *httptest.Server
)

// setup sets up a test HTTP server along with a github.Client that is
// configured to talk to that test server. Tests should register handlers on
// mux which provide mock responses for the API method being tested.
func setup() {
	// test server
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)

	// github client configured to use test server
	client = gh.NewClient(nil)
	url, _ := url.Parse(server.URL)
	client.BaseURL = url
	client.UploadURL = url
}

// teardown closes the test HTTP server.
func teardown() {
	server.Close()
}

var didGET, didPOST, didDELETE bool

func initHandlers(t *testing.T, keyTitle string) {
	mux.HandleFunc("/repos/o/r/keys", func(w http.ResponseWriter, r *http.Request) {
		t.Log(r.Method, r.URL)
		if r.Method == "GET" {
			fmt.Fprint(w, `[{"id":1,"title":"`+keyTitle+`"}]`)
			didGET = true
		} else if r.Method == "POST" {
			didPOST = true
		}
	})

	mux.HandleFunc("/repos/o/r/keys/1", func(w http.ResponseWriter, r *http.Request) {
		t.Log(r.Method, r.URL)
		testMethod(t, r, "DELETE")
		didDELETE = true
	})
}

func TestInsertDeployKey_KeyDoesntExist(t *testing.T) {
	setup()
	defer teardown()
	initHandlers(t, "doesntMatch")

	g := github{
		client: client,
	}

	err := g.InsertDeployKey("o", "r", "ssh-rsa AAA")
	if err != nil {
		t.Fatal(err)
	}
	if didGET != true {
		t.Fatal("Should have requested keys")
	}
	if didPOST != true {
		t.Fatal("Should have created key")
	}
	if didDELETE != false {
		t.Fatal("Should have not deleted key")
	}
}

func TestInsertDeployKey_KeyDoesExist(t *testing.T) {
	setup()
	defer teardown()
	initHandlers(t, deployKeyName)

	g := github{
		client: client,
	}

	err := g.InsertDeployKey("o", "r", "ssh-rsa AAA")
	if err != nil {
		t.Fatal(err)
	}
	if didGET != true {
		t.Fatal("Should have requested keys")
	}
	if didPOST != true {
		t.Fatal("Should have created key")
	}
	if didDELETE != true {
		t.Fatal("Should have deleted key")
	}
}

func testMethod(t *testing.T, r *http.Request, want string) {
	if got := r.Method; got != want {
		t.Errorf("Request method: %v, want %v", got, want)
	}
}
