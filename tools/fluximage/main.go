package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/weaveworks/flux/image"
	//	"github.com/docker/distribution/registry/client"
	//	"github.com/docker/distribution/registry/client/transport"
	"github.com/docker/distribution/registry/client/auth/challenge"
	//	"github.com/docker/distribution"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
)

func bail(err error) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", err)
	os.Exit(1)
}

var (
	raw bool
)

func main() {
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
	out := tabwriter.NewWriter(os.Stdout, 4, 4, 1, ' ', 0)
	fmt.Fprintf(out, "Tag\t%s\n", im.Tag)
	GetManifest(urlStr, "", repo, out)
	out.Flush()
}

func GetManifest(urlStr, token string, repo image.CanonicalName, out io.Writer) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		bail(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Accept", strings.Join([]string{
		schema2.MediaTypeManifest,
		schema1.MediaTypeSignedManifest,
		schema1.MediaTypeManifest,
		manifestlist.MediaTypeManifestList,
	}, ","))
	fmt.Fprintf(os.Stderr, "GET %s\n", req.URL.String())

request:
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		bail(err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusUnauthorized {
		res.Body.Close()
		challenges := challenge.ResponseChallenges(res)
		for _, c := range challenges {
			if c.Scheme == "bearer" {
				token, err = getAuthToken(c)
				if err != nil {
					bail(err)
				}
			}
		}
		if token == "" {
			fmt.Fprintln(os.Stderr, "Unable to authorise against repo")
			os.Exit(1)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		goto request
	}

	schemaType := res.Header.Get("Content-Type")
	digest := res.Header.Get("Docker-Content-Digest")

	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, res.Status)
		goto raw
	} else {
		if raw {
			goto raw
		}

		var (
			schema, imageID string
			arch, os        string
			created         time.Time
		)

		var v1 struct {
			ID      string    `json:"id"`
			Created time.Time `json:"created"`
			OS      string    `json:"os"`
			Arch    string    `json:"architecture"`
		}

		decoder := json.NewDecoder(res.Body)
		switch schemaType {
		case schema1.MediaTypeManifest:
			schema = "schema1.Manifest"
			var man schema1.Manifest
			if err = decoder.Decode(&man); err != nil {
				bail(err)
			}
			if err = json.Unmarshal([]byte(man.History[0].V1Compatibility), &v1); err != nil {
				bail(err)
			}
			imageID = v1.ID
			created = v1.Created
			arch = v1.Arch
			os = v1.OS
		case schema1.MediaTypeSignedManifest:
			schema = "schema1.SignedManifest"
			var man schema1.SignedManifest
			if err = decoder.Decode(&man); err != nil {
				bail(err)
			}
			if err = json.Unmarshal([]byte(man.History[0].V1Compatibility), &v1); err != nil {
				bail(err)
			}
			imageID = v1.ID
			created = v1.Created
			arch = v1.Arch
			os = v1.OS
		case schema2.MediaTypeManifest:
			schema = "schema2.Manifest"
			var man schema2.Manifest
			if err = decoder.Decode(&man); err != nil {
				bail(err)
			}
			imageID = man.Config.Digest.String()
			urlStr = fmt.Sprintf("https://%s/v2/%s/blobs/%s", repo.Domain, repo.Image, imageID)
			req, err := http.NewRequest("GET", urlStr, nil)
			if err != nil {
				bail(err)
			}
			if token != "" {
				req.Header = http.Header{}
				req.Header.Add("Authorization", "Bearer "+token)
			}
			configRes, err := http.DefaultClient.Do(req)
			if err != nil {
				bail(err)
			}
			defer configRes.Body.Close()

			var config struct {
				Arch    string    `json:"architecture"`
				Created time.Time `json:"created"`
				OS      string    `json:"os"`
			}
			if err = json.NewDecoder(configRes.Body).Decode(&config); err != nil {
				bail(err)
			}
			created = config.Created
			os = config.OS
			arch = config.Arch
		case manifestlist.MediaTypeManifestList:
			var list manifestlist.ManifestList
			if err = decoder.Decode(&list); err != nil {
				bail(err)
			}
			for _, manifest := range list.Manifests {
				urlStr = fmt.Sprintf("https://%s/v2/%s/manifests/%s", repo.Domain, repo.Image, manifest.Digest.String())
				GetManifest(urlStr, token, repo, out)
				fmt.Fprintln(out, "---\t---\n")
			}
			return
		}

		fmt.Fprintf(out, "Schema\t%s\n", schema)
		fmt.Fprintf(out, "Digest\t%s\n", digest)
		fmt.Fprintf(out, "ImageID\t%s\n", imageID)
		fmt.Fprintf(out, "Arch\t%s\n", arch)
		fmt.Fprintf(out, "OS\t%s\n", os)
		fmt.Fprintf(out, "Created\t%s\n", created)
	}

raw:
	if !raw {
		return
	}

	fmt.Fprintf(os.Stderr, "Content-Type: %s\nDocker-Content-Digest: %s\n\n", schemaType, digest)

	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		bail(err)
	}
	fmt.Println(string(bytes))
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
