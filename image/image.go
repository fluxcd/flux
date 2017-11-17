package image

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

// Name represents an unversioned (i.e., untagged) image a.k.a.,
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
type Name struct {
	Domain, Image string
}

// CanonicalName is an image name with none of the fields left to be
// implied by convention.
type CanonicalName struct {
	Name
}

//
func (i Name) String() string {
	if i.Image == "" {
		return "" // Doesn't make sense to return anything if it doesn't even have an image
	}
	var host string
	if i.Domain != "" {
		host = i.Domain + "/"
	}
	return fmt.Sprintf("%s%s", host, i.Image)
}

// Repository returns the canonicalised path part of an Name.
func (i Name) Repository() string {
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
func (i Name) Registry() string {
	switch i.Domain {
	case "", oldDockerHubHost:
		return dockerHubHost
	default:
		return i.Domain
	}
}

// CanonicalName returns the canonicalised registry host and image parts
// of the ID.
func (i Name) CanonicalName() CanonicalName {
	return CanonicalName{
		Name: Name{
			Domain: i.Registry(),
			Image:  i.Repository(),
		},
	}
}

func (i Name) ToRef(tag string) Ref {
	return Ref{
		Name: i,
		Tag:  tag,
	}
}

// Ref represents a versioned (i.e., tagged) image. The tag is
// allowed to be empty, though it is in general undefined what that
// means. As such, `Ref` also includes all `Name` values.
//
// Examples (stringified):
//  * alpine:3.5
//  * library/alpine:3.5
//  * quay.io/weaveworks/flux:1.1.0
//  * localhost:5000/arbitrary/path/to/repo:revision-sha1
type Ref struct {
	Name
	Tag string
}

// CanonicalRef is an image ref with none of the fields left to be
// implied by convention.
type CanonicalRef struct {
	Ref
}

// String returns the Ref as a string (i.e., unparsed) without canonicalising it.
func (i Ref) String() string {
	var tag string
	if i.Tag != "" {
		tag = ":" + i.Tag
	}
	return fmt.Sprintf("%s%s", i.Name.String(), tag)
}

// ParseRef parses a string representation of an image id into an
// Ref value. The grammar is shown here:
// https://github.com/docker/distribution/blob/master/reference/reference.go
// (but we do not care about all the productions.)
func ParseRef(s string) (Ref, error) {
	var id Ref
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
func (i Ref) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.String())
}

// ImageID is serialized/deserialized as a string
func (i *Ref) UnmarshalJSON(data []byte) (err error) {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	*i, err = ParseRef(string(str))
	return err
}

// CanonicalRef returns the canonicalised reference including the tag
// if present.
func (i Ref) CanonicalRef() CanonicalRef {
	name := i.CanonicalName()
	return CanonicalRef{
		Ref: Ref{
			Name: name.Name,
			Tag:  i.Tag,
		},
	}
}

func (i Ref) Components() (domain, repo, tag string) {
	return i.Domain, i.Image, i.Tag
}

// WithNewTag makes a new copy of an ImageID with a new tag
func (i Ref) WithNewTag(t string) Ref {
	var img Ref
	img = i
	img.Tag = t
	return img
}

// Info has the metadata we are able to determine about an image ref,
// from its registry.
type Info struct {
	// the reference to this image; probably a tagged image name
	ID Ref
	// the digest we got when fetching the metadata, which will be
	// different each time a manifest is uploaded for the reference
	Digest string
	// an identifier for the *image* this reference points to; this
	// will be the same for references that point at the same image
	// (but does not necessarily equal Docker's image ID)
	ImageID string
	// the time at which the image pointed at was created
	CreatedAt time.Time
}

// MarshalJSON returns the Info value in JSON (as bytes). It is
// implemented so that we can omit the `CreatedAt` value when it's
// zero, which would otherwise be tricky for e.g., JavaScript to
// detect.
func (im Info) MarshalJSON() ([]byte, error) {
	type InfoAlias Info // alias to shed existing MarshalJSON implementation
	var t string
	if !im.CreatedAt.IsZero() {
		t = im.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	encode := struct {
		InfoAlias
		CreatedAt string `json:",omitempty"`
	}{InfoAlias(im), t}
	return json.Marshal(encode)
}

// UnmarshalJSON populates an Info from JSON (as bytes). It's the
// companion to MarshalJSON above.
func (im *Info) UnmarshalJSON(b []byte) error {
	type InfoAlias Info
	unencode := struct {
		InfoAlias
		CreatedAt string `json:",omitempty"`
	}{}
	json.Unmarshal(b, &unencode)
	*im = Info(unencode.InfoAlias)
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

// ByCreatedDesc is a shim used to sort image info by creation date
type ByCreatedDesc []Info

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
