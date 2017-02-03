package flux

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	dockerHubHost    = "index.docker.io"
	dockerHubLibrary = "library"
)

var (
	ErrInvalidImageID   = errors.New("invalid image ID")
	ErrBlankImageID     = errors.Wrap(ErrInvalidImageID, "blank image name")
	ErrMalformedImageID = errors.Wrap(ErrInvalidImageID, `expected image name as either <image>:<tag> or just <image>`)
)

// ImageID is a fully qualified name that refers to a particular Image.
// It is in the format: host[:port]/Namespace/Image[:tag]
// Here, we refer to the "name" == Namespace/Image
type ImageID struct {
	Host, Namespace, Image, Tag string
}

func ParseImageID(s string) (ImageID, error) {
	if s == "" {
		return ImageID{}, ErrBlankImageID
	}
	var img ImageID
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 0:
		return ImageID{}, ErrMalformedImageID
	case 1:
		img.Tag = "latest"
	case 2:
		img.Tag = parts[1]
		s = parts[0]
	default:
		return ImageID{}, ErrMalformedImageID
	}
	if s == "" {
		return ImageID{}, ErrBlankImageID
	}
	parts = strings.Split(s, "/")
	switch len(parts) {
	case 1:
		img.Host = dockerHubHost
		img.Namespace = dockerHubLibrary
		img.Image = parts[0]
	case 2:
		img.Host = dockerHubHost
		img.Namespace = parts[0]
		img.Image = parts[1]
	case 3:
		img.Host = parts[0]
		img.Namespace = parts[1]
		img.Image = parts[2]
	default:
		return ImageID{}, ErrMalformedImageID
	}
	return img, nil
}

// Fully qualified name
func (i ImageID) String() string {
	if i.Image == "" {
		return "" // Doesn't make sense to return anything if it doesn't even have an image
	}
	var ta string
	if i.Tag != "" {
		ta = fmt.Sprintf(":%s", i.Tag)
	}
	return fmt.Sprintf("%s%s", i.Repository(), ta)
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

// Repository returns the short version of an image's repository (trimming if dockerhub)
func (i ImageID) Repository() string {
	r := i.HostNamespaceImage()
	r = strings.TrimPrefix(r, dockerHubHost+"/")
	r = strings.TrimPrefix(r, dockerHubLibrary+"/")
	return r
}

// HostNamespaceImage includes all parts of the image, even if it is from dockerhub.
func (i ImageID) HostNamespaceImage() string {
	return fmt.Sprintf("%s/%s/%s", i.Host, i.Namespace, i.Image)
}

func (i ImageID) NamespaceImage() string {
	return fmt.Sprintf("%s/%s", i.Namespace, i.Image)
}

func (i ImageID) FullID() string {
	return fmt.Sprintf("%s/%s/%s:%s", i.Host, i.Namespace, i.Image, i.Tag)
}

func (i ImageID) Components() (host, repo, tag string) {
	return i.Host, fmt.Sprintf("%s/%s", i.Namespace, i.Image), i.Tag
}

// Image can't really be a primitive string only, because we need to also
// record information about its creation time. (maybe more in the future)
type Image struct {
	ImageID
	CreatedAt *time.Time `json:",omitempty"`
}

func ParseImage(s string, createdAt *time.Time) (Image, error) {
	id, err := ParseImageID(s)
	if err != nil {
		return Image{}, err
	}
	return Image{
		ImageID:   id,
		CreatedAt: createdAt,
	}, nil
}
