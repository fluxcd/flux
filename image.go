package flux

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	dockerHubHost    = "index.docker.io"
	dockerHubLibrary = "library/"

	oldDockerHubHost = "docker.io"
)

var (
	ErrInvalidImageID   = errors.New("invalid image ID")
	ErrBlankImageID     = errors.Wrap(ErrInvalidImageID, "blank image name")
	ErrMalformedImageID = errors.Wrap(ErrInvalidImageID, `expected image name as either <image>:<tag> or just <image>`)
)

// ImageID is a fully qualified name that refers to a particular
// (tagged) image or image repository.  It is usually found
// stringified in the format: `[host[:port]]/Image[:tag]`
type ImageID struct {
	Domain, Image, Tag string
}

// ParseImageID parses a string representation of an image id into an
// ImageID value. The grammar is shown here:
// https://github.com/docker/distribution/blob/master/reference/reference.go
// (but we do not care about all the productions.)
func ParseImageID(s string) (ImageID, error) {
	if s == "" {
		return ImageID{}, ErrBlankImageID
	}
	if strings.HasPrefix(s, "/") || strings.HasSuffix(s, "/") {
		return ImageID{}, ErrMalformedImageID
	}

	var id ImageID

	elements := strings.Split(s, "/")
	switch len(elements) {
	case 0: // NB strings.Split will never return []
		return id, ErrBlankImageID
	case 1: // no slashes, e.g., "alpine:1.5"; treat as library image
		id.Image = s
	case 2: // may have a domain e.g., "localhost/foo", or not e.g., "weaveworks/scope"
		if domainRegexp.MatchString(elements[0]) {
			id.Domain = elements[0]
			id.Image = elements[1]
		} else {
			id.Image = s
		}
	default: // cannot be a library image, so the first element is assumed to be a domain
		id.Domain = elements[0]
		id.Image = strings.Join(elements[1:], "/")
	}

	// Figure out if there's a tag
	imageParts := strings.Split(id.Image, ":")
	switch len(imageParts) {
	case 1:
		break
	case 2:
		if imageParts[0] == "" || imageParts[1] == "" {
			return id, ErrMalformedImageID
		}
		id.Image = imageParts[0]
		id.Tag = imageParts[1]
	default:
		return id, ErrMalformedImageID
	}

	return id, nil
}

var (
	domainComponent = `([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])`
	domain          = fmt.Sprintf(`localhost|(%s([.]%s)+)(:[0-9]+)?`, domainComponent, domainComponent)
	domainRegexp    = regexp.MustCompile(domain)
)

// String returns the ImageID as a string (i.e., unparsed) without canonicalising it.
func (i ImageID) String() string {
	if i.Image == "" {
		return "" // Doesn't make sense to return anything if it doesn't even have an image
	}
	var tag string
	if i.Tag != "" {
		tag = ":" + i.Tag
	}
	var host string
	if i.Domain != "" {
		host = i.Domain + "/"
	}
	return fmt.Sprintf("%s%s%s", host, i.Image, tag)
}

// ImageID is serialized/deserialized as a string
func (i ImageID) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

// ImageID is serialized/deserialized as a string
func (i *ImageID) UnmarshalJSON(data []byte) (err error) {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*i, err = ParseImageID(string(str))
	return err
}

// Repository returns the canonicalised path part of an ImageID.
func (i ImageID) Repository() string {
	switch i.Domain {
	case "", oldDockerHubHost, dockerHubHost:
		path := strings.Split(i.Image, "/")
		if len(path) == 1 {
			return "library/" + i.Image
		}
		return i.Image
	default:
		return i.Image
	}
}

// Registry returns the domain name of the Docker image registry to
// use to fetch the image or image metadata.
func (i ImageID) Registry() string {
	switch i.Domain {
	case "", oldDockerHubHost:
		return dockerHubHost
	default:
		return i.Domain
	}
}

// CanonicalName returns the canonicalised registry host and image parts
// of the ID.
func (i ImageID) CanonicalName() string {
	return fmt.Sprintf("%s/%s", i.Registry(), i.Repository())
}

// CanonicalRef returns the full, canonicalised ID including the tag if present.
func (i ImageID) CanonicalRef() string {
	if i.Tag == "" {
		return fmt.Sprintf("%s/%s", i.Registry(), i.Repository())
	}
	return fmt.Sprintf("%s/%s:%s", i.Registry(), i.Repository(), i.Tag)
}

func (i ImageID) Components() (domain, repo, tag string) {
	return i.Domain, i.Image, i.Tag
}

// WithNewTag makes a new copy of an ImageID with a new tag
func (i ImageID) WithNewTag(t string) ImageID {
	var img ImageID
	img = i
	img.Tag = t
	return img
}

// Image can't really be a primitive string only, because we need to also
// record information about its creation time. (maybe more in the future)
type Image struct {
	ID        ImageID
	CreatedAt time.Time
}

func (im Image) MarshalJSON() ([]byte, error) {
	var t string
	if !im.CreatedAt.IsZero() {
		t = im.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	encode := struct {
		ID        ImageID
		CreatedAt string `json:",omitempty"`
	}{im.ID, t}
	return json.Marshal(encode)
}

func (im *Image) UnmarshalJSON(b []byte) error {
	unencode := struct {
		ID        ImageID
		CreatedAt string `json:",omitempty"`
	}{}
	json.Unmarshal(b, &unencode)
	im.ID = unencode.ID
	if unencode.CreatedAt == "" {
		im.CreatedAt = time.Time{}
	} else {
		t, err := time.Parse(time.RFC3339, unencode.CreatedAt)
		if err != nil {
			return err
		}
		im.CreatedAt = t.UTC()
	}
	return nil
}

func ParseImage(s string, createdAt time.Time) (Image, error) {
	id, err := ParseImageID(s)
	if err != nil {
		return Image{}, err
	}
	return Image{
		ID:        id,
		CreatedAt: createdAt,
	}, nil
}

// Sort image by creation date
type ByCreatedDesc []Image

func (is ByCreatedDesc) Len() int      { return len(is) }
func (is ByCreatedDesc) Swap(i, j int) { is[i], is[j] = is[j], is[i] }
func (is ByCreatedDesc) Less(i, j int) bool {
	switch {
	case is[i].CreatedAt.IsZero():
		return true
	case is[j].CreatedAt.IsZero():
		return false
	case is[i].CreatedAt.Equal(is[j].CreatedAt):
		return is[i].ID.String() < is[j].ID.String()
	default:
		return is[i].CreatedAt.After(is[j].CreatedAt)
	}
}
