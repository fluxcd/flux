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
