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

// An image is a fully qualified name that refers to a particular image.
// It is in the format: host[:port]/orgname/reponame[:tag]
// Here, we refer to the "name" == orgname/reponame

// Image can't really be a primitive string only, because we need to also
// record information about it's creation time. (maybe more in the future)
type Image interface {
	WithTag(tag string) Image
	WithCreatedAt(t *time.Time) Image
	Components() (host, org, repo, tag string)
	Host() string
	Org() string
	Repo() string
	Tag() string
	FQN() string
	HostOrgRepo() string
	OrgRepo() string
	CreatedAt() *time.Time
	Clone() Image
}

type image struct {
	host, org, repo, tag string
	createdAt            *time.Time `json:",omitempty"`
}

func ParseImage(s string, createdAt *time.Time) (Image, error) {
	img := &image{
		createdAt: createdAt,
	}
	if s == "" {
		return nil, fmt.Errorf(`expected image name as either <image>:<tag> or just <image>`)
	}
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 0:
		return nil, fmt.Errorf(`expected image name as either <image>:<tag> or just <image>`)
	case 1:
		img.tag = "latest"
		break
	case 2:
		img.tag = parts[1]
		s = parts[0]
	default:
		return nil, fmt.Errorf(`expected image name as either <image>:<tag> or just <image>`)
	}
	parts = strings.Split(s, "/")
	switch len(parts) {
	case 1:
		img.host = dockerHubHost
		img.org = dockerHubLibrary
		img.repo = parts[0]
	case 2:
		img.host = dockerHubHost
		img.org = parts[0]
		img.repo = parts[1]
	case 3:
		img.host = parts[0]
		img.org = parts[1]
		img.repo = parts[2]
	default:
		return nil, fmt.Errorf(`expected image name as either "<host>/<org>/<image>", "<org>/<image>", or "<image>"`)
	}
	return img, nil
}

func (i *image) WithTag(t string) Image {
	img := i.Clone().(*image)
	img.tag = t
	return img
}

func (i *image) WithCreatedAt(c *time.Time) Image {
	img := i.Clone().(*image)
	img.createdAt = c
	return img
}

func (i *image) Components() (host, org, repo, tag string) {
	return i.host, i.org, i.repo, i.tag
}
func (i *image) Host() string {
	return i.host
}
func (i *image) Org() string {
	return i.org
}
func (i *image) Repo() string {
	return i.repo
}
func (i *image) Tag() string {
	return i.tag
}
func (i *image) CreatedAt() *time.Time {
	return i.createdAt
}

// Fully qualified name
func (i *image) FQN() string {
	return fmt.Sprintf("%s/%s/%s:%s", i.host, i.org, i.repo, i.tag)
}

func (i *image) HostOrgRepo() string {
	return fmt.Sprintf("%s/%s/%s", i.host, i.org, i.repo)
}

func (i *image) OrgRepo() string {
	return fmt.Sprintf("%s/%s", i.org, i.repo)
}

func (i *image) Clone() Image {
	return &image{
		createdAt: i.createdAt,
		host:      i.host,
		org:       i.org,
		repo:      i.repo,
		tag:       i.tag,
	}
}
