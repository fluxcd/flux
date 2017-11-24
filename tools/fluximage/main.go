package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/weaveworks/flux/image"
	//	"github.com/docker/distribution/registry/client"
	//	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/distribution/registry/client/auth/challenge"
	//	"github.com/docker/distribution"
)

func bail(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}

func main() {
	var raw bool
	flag.BoolVar(&raw, "raw", false, "show raw response body")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Error: needs an image ref to get metadata for")
		os.Exit(1)
	}

	im, err := image.ParseRef(args[0])
	if err != nil {
		bail(err)
	}

	repo := im.CanonicalName()
	urlStr := fmt.Sprintf("https://%s/v2/%s/manifests/%s", repo.Domain, repo.Image, im.Tag)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		bail(err)
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	fmt.Println("GET ", req.URL.String())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		bail(err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized {
		res.Body.Close()
		challenges := challenge.ResponseChallenges(res)
		var token string
		for _, c := range challenges {
			if c.Scheme == "bearer" {
				token, err = getAuthToken(c)
				if err != nil {
					bail(err)
				}
			}
		}
		if token == "" {
			fmt.Fprintln(os.Stderr, "Unable to authorise against repo", repo.String())
			os.Exit(1)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		res, err = http.DefaultClient.Do(req)
		if err != nil {
			bail(err)
		}
	}

	if res.StatusCode == http.StatusOK {
		fmt.Fprintf(os.Stderr, "%s %s\n", res.Header.Get("Docker-Content-Digest"), res.Header.Get("Content-Type"))
	} else {
		fmt.Fprintln(os.Stderr, res.Status)
	}
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		bail(err)
	}
	if raw {
		fmt.Println(string(bytes))
	}
}

func getAuthToken(c challenge.Challenge) (string, error) {
	realm := c.Parameters["realm"]
	vals := url.Values{}
	vals.Add("service", c.Parameters["service"])
	vals.Add("scope", c.Parameters["scope"])
	req, err := http.NewRequest("GET", realm, nil)
	if err != nil {
		return "", err
	}
	req.URL.RawQuery = vals.Encode()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", errors.New("authentication failed: " + res.Status)
	}

	type auth struct {
		Token string `json:"token"`
	}

	var a auth
	if err = json.NewDecoder(res.Body).Decode(&a); err != nil {
		return "", err
	}
	return a.Token, nil
}
