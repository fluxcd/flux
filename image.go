package flux

import (
	"time"
)

// Image can't really be a primitive string only, because we need to also
// record information about it's creation time. (maybe more in the future)
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
