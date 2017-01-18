package registry

import (
	"fmt"
	"strings"
	"time"
)

const (
	dockerHubHost    = "index.docker.io"
	dockerHubLibrary = "library"
)

// An Image is a fully qualified name that refers to a particular Image.
// It is in the format: host[:port]/Namespace/Image[:tag]
// Here, we refer to the "name" == Namespace/Image

// Image can't really be a primitive string only, because we need to also
// record information about it's creation time. (maybe more in the future)
type Image struct {
	Host, Namespace, Image, Tag string
	CreatedAt                   *time.Time `json:",omitempty"`
}

func ParseImage(s string, createdAt *time.Time) (img Image, err error) {
	img = Image{
		CreatedAt: createdAt,
	}
	if s == "" {
		err = fmt.Errorf(`expected Image name as either <Image>:<tag> or just <Image>`)
		return
	}
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 0:
		err = fmt.Errorf(`expected Image name as either <Image>:<tag> or just <Image>`)
		return
	case 1:
		img.Tag = "latest"
		break
	case 2:
		img.Tag = parts[1]
		s = parts[0]
	default:
		err = fmt.Errorf(`expected Image name as either <Image>:<tag> or just <Image>`)
		return
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
		err = fmt.Errorf(`expected Image name as either <Image>:<tag> or just <Image>`)
		return
	}
	return
}

// Fully qualified name
func (i *Image) String() string {
	if i.Image == "" {
		return "" // Doesn't make sense to return anything if it doesn't even have an image
	}
	var ho, na, ta string
	if i.Host != "" {
		ho = fmt.Sprintf("%s/", i.Host)
	}
	if i.Namespace != "" {
		na = fmt.Sprintf("%s/", i.Namespace)
	}
	if i.Tag != "" {
		ta = fmt.Sprintf(":%s", i.Tag)
	}

	return fmt.Sprintf("%s%s%s%s", ho, na, i.Image, ta)
}

func (i *Image) HostNamespaceImage() string {
	return fmt.Sprintf("%s/%s/%s", i.Host, i.Namespace, i.Image)
}

func (i *Image) NamespaceImage() string {
	return fmt.Sprintf("%s/%s", i.Namespace, i.Image)
}
