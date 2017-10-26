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
	dockerHubHost = "index.docker.io"

	oldDockerHubHost = "docker.io"
)

var (
	ErrInvalidImageID   = errors.New("invalid image ID")
	ErrBlankImageID     = errors.Wrap(ErrInvalidImageID, "blank image name")
	ErrMalformedImageID = errors.Wrap(ErrInvalidImageID, `expected image name as either <image>:<tag> or just <image>`)
)

// ImageName represents an unversioned (i.e., untagged) image a.k.a.,
// an image repo. These sometimes include a domain, e.g., quay.io, and
// always include a path with at least one element. By convention,
// images at DockerHub may have the domain omitted; and, if they only
// have single path element, the prefix `library` is implied.
//
// Examples (stringified):
//   * alpine
//   * library/alpine
//   * quay.io/weaveworks/flux
//   * localhost:5000/arbitrary/path/to/repo
type ImageName struct {
	Domain, Image string
}

// CanonicalName is an image name with none of the fields left to be
// implied by convention.
type CanonicalName struct {
	ImageName
}

//
func (i ImageName) String() string {
	if i.Image == "" {
		return "" // Doesn't make sense to return anything if it doesn't even have an image
	}
	var host string
	if i.Domain != "" {
		host = i.Domain + "/"
	}
	return fmt.Sprintf("%s%s", host, i.Image)
}

// Repository returns the canonicalised path part of an ImageName.
func (i ImageName) Repository() string {
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

// Registry returns the domain name of the Docker image registry, to
// use to fetch the image or image metadata.
func (i ImageName) Registry() string {
	switch i.Domain {
	case "", oldDockerHubHost:
		return dockerHubHost
	default:
		return i.Domain
	}
}

// CanonicalName returns the canonicalised registry host and image parts
// of the ID.
func (i ImageName) CanonicalName() CanonicalName {
	return CanonicalName{
		ImageName: ImageName{
			Domain: i.Registry(),
			Image:  i.Repository(),
		},
	}
}

func (i ImageName) ToRef(tag string) ImageRef {
	return ImageRef{
		ImageName: i,
		Tag:       tag,
	}
}

// ImageRef represents a versioned (i.e., tagged) image. The tag is
// allowed to be empty, though it is in general undefined what that
// means. As such, `ImageRef` also includes all `ImageName` values.
//
// Examples (stringified):
//  * alpine:3.5
//  * library/alpine:3.5
//  * quay.io/weaveworks/flux:1.1.0
//  * localhost:5000/arbitrary/path/to/repo:revision-sha1
type ImageRef struct {
	ImageName
	Tag string
}

// CanonicalRef is an image ref with none of the fields left to be
// implied by convention.
type CanonicalRef struct {
	ImageRef
}

// String returns the ImageRef as a string (i.e., unparsed) without canonicalising it.
func (i ImageRef) String() string {
	var tag string
	if i.Tag != "" {
		tag = ":" + i.Tag
	}
	return fmt.Sprintf("%s%s", i.ImageName.String(), tag)
}

func (i ImageRef) Name() ImageName {
	return i.ImageName
}

// ParseImageRef parses a string representation of an image id into an
// ImageRef value. The grammar is shown here:
// https://github.com/docker/distribution/blob/master/reference/reference.go
// (but we do not care about all the productions.)
func ParseImageRef(s string) (ImageRef, error) {
	var id ImageRef
	if s == "" {
		return id, ErrBlankImageID
	}
	if strings.HasPrefix(s, "/") || strings.HasSuffix(s, "/") {
		return id, ErrMalformedImageID
	}

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

// ImageID is serialized/deserialized as a string
func (i ImageRef) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

// ImageID is serialized/deserialized as a string
func (i *ImageRef) UnmarshalJSON(data []byte) (err error) {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*i, err = ParseImageRef(string(str))
	return err
}

// CanonicalRef returns the canonicalised reference including the tag
// if present.
func (i ImageRef) CanonicalRef() CanonicalRef {
	name := i.CanonicalName()
	return CanonicalRef{
		ImageRef: ImageRef{
			ImageName: name.ImageName,
			Tag:       i.Tag,
		},
	}
}

func (i ImageRef) Components() (domain, repo, tag string) {
	return i.Domain, i.Image, i.Tag
}

// WithNewTag makes a new copy of an ImageID with a new tag
func (i ImageRef) WithNewTag(t string) ImageRef {
	var img ImageRef
	img = i
	img.Tag = t
	return img
}

// Image can't really be a primitive string only, because we need to also
// record information about its creation time. (maybe more in the future)
type Image struct {
	ID        ImageRef
	CreatedAt time.Time
}

func (im Image) MarshalJSON() ([]byte, error) {
	var t string
	if !im.CreatedAt.IsZero() {
		t = im.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	encode := struct {
		ID        ImageRef
		CreatedAt string `json:",omitempty"`
	}{im.ID, t}
	return json.Marshal(encode)
}

func (im *Image) UnmarshalJSON(b []byte) error {
	unencode := struct {
		ID        ImageRef
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
	id, err := ParseImageRef(s)
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
