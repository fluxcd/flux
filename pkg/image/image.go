package image

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
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
//   * docker.io/fluxcd/flux
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
//  * docker.io/fluxcd/flux:1.1.0
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
		return id, errors.Wrapf(ErrBlankImageID, "parsing %q", s)
	}
	if strings.HasPrefix(s, "/") || strings.HasSuffix(s, "/") {
		return id, errors.Wrapf(ErrMalformedImageID, "parsing %q", s)
	}

	elements := strings.Split(s, "/")
	switch len(elements) {
	case 0: // NB strings.Split will never return []
		return id, errors.Wrapf(ErrMalformedImageID, "parsing %q", s)
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
			return id, errors.Wrapf(ErrMalformedImageID, "parsing %q", s)
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

type LabelTimestampFormatError struct {
	Labels []string
}

func (e *LabelTimestampFormatError) Error() string {
	return fmt.Sprintf(
		"failed to parse %d timestamp label(s) as RFC3339 (%s); ask the repository administrator to correct this as it conflicts with the spec",
		len(e.Labels),
		strings.Join(e.Labels, ", "))
}

// Labels has all the image labels we are interested in for an image
// ref, the JSON struct tag keys should be equal to the label.
type Labels struct {
	// BuildDate holds the Label Schema spec 'build date' label
	// Ref: http://label-schema.org/rc1/#build-time-labels
	BuildDate time.Time `json:"org.label-schema.build-date,omitempty"`
	// Created holds the Open Container Image spec 'created' label
	// Ref: https://github.com/opencontainers/image-spec/blob/master/annotations.md#pre-defined-annotation-keys
	Created time.Time `json:"org.opencontainers.image.created,omitempty"`
}

// MarshalJSON returns the Labels value in JSON (as bytes). It is
// implemented so that we can omit the time values when they are
// zero, which would otherwise be tricky for e.g., JavaScript to
// detect.
func (l Labels) MarshalJSON() ([]byte, error) {
	var bd, c string
	if !l.BuildDate.IsZero() {
		bd = l.BuildDate.UTC().Format(time.RFC3339Nano)
	}
	if !l.Created.IsZero() {
		c = l.Created.UTC().Format(time.RFC3339Nano)
	}
	encode := struct {
		BuildDate string `json:"org.label-schema.build-date,omitempty"`
		Created   string `json:"org.opencontainers.image.created,omitempty"`
	}{BuildDate: bd, Created: c}
	return json.Marshal(encode)
}

// UnmarshalJSON populates Labels from JSON (as bytes). It's the
// companion to MarshalJSON above.
func (l *Labels) UnmarshalJSON(b []byte) error {
	unencode := struct {
		BuildDate string `json:"org.label-schema.build-date,omitempty"`
		Created   string `json:"org.opencontainers.image.created,omitempty"`
	}{}
	json.Unmarshal(b, &unencode)
	labelErr := LabelTimestampFormatError{}
	if err := decodeTime(unencode.BuildDate, &l.BuildDate); err != nil {
		if _, ok := err.(*time.ParseError); !ok {
			return err
		}
		labelErr.Labels = append(labelErr.Labels, "org.label-schema.build-date")
	}
	if err := decodeTime(unencode.Created, &l.Created); err != nil {
		if _, ok := err.(*time.ParseError); !ok {
			return err
		}
		labelErr.Labels = append(labelErr.Labels, "org.opencontainers.image.created")
	}
	if len(labelErr.Labels) >= 1 {
		return &labelErr
	}
	return nil
}

// Info has the metadata we are able to determine about an image ref,
// from its registry.
type Info struct {
	// the reference to this image; probably a tagged image name
	ID Ref `json:",omitempty"`
	// the digest we got when fetching the metadata, which will be
	// different each time a manifest is uploaded for the reference
	Digest string `json:",omitempty"`
	// an identifier for the *image* this reference points to; this
	// will be the same for references that point at the same image
	// (but does not necessarily equal Docker's image ID)
	ImageID string `json:",omitempty"`
	// all labels we are interested in and could find for the image ref
	Labels Labels `json:",omitempty"`
	// the time at which the image pointed at was created
	CreatedAt time.Time `json:",omitempty"`
	// the last time this image manifest was fetched
	LastFetched time.Time `json:",omitempty"`
}

// MarshalJSON returns the Info value in JSON (as bytes). It is
// implemented so that we can omit the `CreatedAt` value when it's
// zero, which would otherwise be tricky for e.g., JavaScript to
// detect.
func (im Info) MarshalJSON() ([]byte, error) {
	type InfoAlias Info // alias to shed existing MarshalJSON implementation
	var ca, lf string
	if !im.CreatedAt.IsZero() {
		ca = im.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	if !im.LastFetched.IsZero() {
		lf = im.LastFetched.UTC().Format(time.RFC3339Nano)
	}
	encode := struct {
		InfoAlias
		CreatedAt   string `json:",omitempty"`
		LastFetched string `json:",omitempty"`
	}{InfoAlias(im), ca, lf}
	return json.Marshal(encode)
}

// UnmarshalJSON populates an Info from JSON (as bytes). It's the
// companion to MarshalJSON above.
func (im *Info) UnmarshalJSON(b []byte) error {
	type InfoAlias Info
	unencode := struct {
		InfoAlias
		CreatedAt   string `json:",omitempty"`
		LastFetched string `json:",omitempty"`
	}{}
	json.Unmarshal(b, &unencode)
	*im = Info(unencode.InfoAlias)

	var err error
	if err = decodeTime(unencode.CreatedAt, &im.CreatedAt); err == nil {
		err = decodeTime(unencode.LastFetched, &im.LastFetched)
	}
	return err
}

// RepositoryMetadata contains the image metadata information found in an
// image repository.
//
// `Images` is indexed by `Tags`. Note that `Images` may be partial/incomplete,
// (i.e. entries from `Tags` may not have a corresponding key in `Images`),
// this indicates that the tag manifest was missing or corrupted in the
// repository.
type RepositoryMetadata struct {
	Tags   []string        // all the tags found in the repository
	Images map[string]Info // indexed by `Tags`, but may not include keys for all entries in `Tags`
}

// FindImageWithRef returns image.Info given an image ref. If the image cannot be
// found, it returns the image.Info with the ID provided.
func (rm RepositoryMetadata) FindImageWithRef(ref Ref) Info {
	for _, img := range rm.Images {
		if img.ID == ref {
			return img
		}
	}
	return Info{ID: ref}
}

// GetImageTagInfo gets the information of all image tags.
// If there are tags missing information, an error is returned
func (rm RepositoryMetadata) GetImageTagInfo() ([]Info, error) {
	result := make([]Info, len(rm.Tags), len(rm.Tags))
	for i, tag := range rm.Tags {
		info, ok := rm.Images[tag]
		if !ok {
			return nil, fmt.Errorf("missing metadata for image tag %q", tag)
		}
		result[i] = info
	}
	return result, nil
}

func decodeTime(s string, t *time.Time) error {
	if s == "" {
		*t = time.Time{}
	} else {
		var err error
		*t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return err
		}
	}
	return nil
}

// NewerByCreated returns true if lhs image should be sorted
// before rhs with regard to their creation date descending.
func NewerByCreated(lhs, rhs *Info) bool {
	if lhs.CreatedAt.Equal(rhs.CreatedAt) {
		return lhs.ID.String() < rhs.ID.String()
	}
	return lhs.CreatedAt.After(rhs.CreatedAt)
}

// NewerBySemver returns true if lhs image should be sorted
// before rhs with regard to their semver order descending.
func NewerBySemver(lhs, rhs *Info) bool {
	lv, lerr := semver.NewVersion(lhs.ID.Tag)
	rv, rerr := semver.NewVersion(rhs.ID.Tag)
	if (lerr != nil && rerr != nil) || (lv == rv) {
		return lhs.ID.String() < rhs.ID.String()
	}
	if lerr != nil {
		return false
	}
	if rerr != nil {
		return true
	}
	cmp := lv.Compare(rv)
	// In semver, `1.10` and `1.10.0` is the same but in favor of explicitness
	// we should consider the latter newer.
	if cmp == 0 {
		return lhs.ID.String() > rhs.ID.String()
	}
	return cmp > 0
}

// Sort orders the given image infos according to `newer` func.
func Sort(infos []Info, newer func(a, b *Info) bool) {
	if newer == nil {
		newer = NewerByCreated
	}
	sort.Sort(&infoSort{infos: infos, newer: newer})
}

type infoSort struct {
	infos []Info
	newer func(a, b *Info) bool
}

func (s *infoSort) Len() int {
	return len(s.infos)
}

func (s *infoSort) Swap(i, j int) {
	s.infos[i], s.infos[j] = s.infos[j], s.infos[i]
}

func (s *infoSort) Less(i, j int) bool {
	return s.newer(&s.infos[i], &s.infos[j])
}
