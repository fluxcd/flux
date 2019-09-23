package v6

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fluxcd/flux/pkg/image"
	"github.com/fluxcd/flux/pkg/policy"
	"github.com/fluxcd/flux/pkg/update"
)

type justSlice []image.Info

func (j justSlice) Images() []image.Info {
	return []image.Info(j)
}

func (j justSlice) SortedImages(p policy.Pattern) update.SortedImageInfos {
	return update.SortImages(j.Images(), p)
}

func TestNewContainer(t *testing.T) {

	testImage := image.Info{ImageID: "test"}

	currentSemver := image.Info{ID: image.Ref{Tag: "1.0.0"}}
	oldSemver := image.Info{ID: image.Ref{Tag: "0.9.0"}}
	newSemver := image.Info{ID: image.Ref{Tag: "1.2.3"}}

	type args struct {
		name         string
		images       []image.Info
		currentImage image.Info
		tagPattern   policy.Pattern
		fields       []string
	}
	tests := []struct {
		name    string
		args    args
		want    Container
		wantErr bool
	}{
		{
			name: "Simple",
			args: args{
				name:         "container1",
				images:       []image.Info{testImage},
				currentImage: testImage,
				tagPattern:   policy.PatternAll,
			},
			want: Container{
				Name:                    "container1",
				Current:                 testImage,
				LatestFiltered:          testImage,
				Available:               []image.Info{testImage},
				AvailableImagesCount:    1,
				NewAvailableImagesCount: 0,
				FilteredImagesCount:     1,
				NewFilteredImagesCount:  0,
			},
			wantErr: false,
		},
		{
			name: "Semver filtering and sorting",
			args: args{
				name:         "container-semver",
				images:       []image.Info{currentSemver, newSemver, oldSemver, testImage},
				currentImage: currentSemver,
				tagPattern:   policy.NewPattern("semver:*"),
			},
			want: Container{
				Name:                    "container-semver",
				Current:                 currentSemver,
				LatestFiltered:          newSemver,
				Available:               []image.Info{newSemver, currentSemver, oldSemver, testImage},
				AvailableImagesCount:    4,
				NewAvailableImagesCount: 1,
				FilteredImagesCount:     3,
				NewFilteredImagesCount:  1,
			},
			wantErr: false,
		},
		{
			name: "Require only some calculations",
			args: args{
				name:         "container-some",
				images:       []image.Info{currentSemver, newSemver, oldSemver, testImage},
				currentImage: currentSemver,
				tagPattern:   policy.NewPattern("semver:*"),
				fields:       []string{"Name", "NewFilteredImagesCount"}, // but not, e.g., "FilteredImagesCount"
			},
			want: Container{
				Name:                   "container-some",
				NewFilteredImagesCount: 1,
			},
		},
		{
			name: "Fields in one order",
			args: args{
				name:         "container-ordered1",
				images:       []image.Info{currentSemver, newSemver, oldSemver, testImage},
				currentImage: currentSemver,
				tagPattern:   policy.NewPattern("semver:*"),
				fields: []string{"Name",
					"AvailableImagesCount", "Available", // these two both depend on the same intermediate result
					"LatestFiltered", "FilteredImagesCount", // these two both depend on another intermediate result
				},
			},
			want: Container{
				Name:                 "container-ordered1",
				Available:            []image.Info{newSemver, currentSemver, oldSemver, testImage},
				AvailableImagesCount: 4,
				LatestFiltered:       newSemver,
				FilteredImagesCount:  3,
			},
		},
		{
			name: "Fields in another order",
			args: args{
				name:         "container-ordered2",
				images:       []image.Info{currentSemver, newSemver, oldSemver, testImage},
				currentImage: currentSemver,
				tagPattern:   policy.NewPattern("semver:*"),
				fields: []string{"Name",
					"Available", "AvailableImagesCount", // these two latter depend on the same intermediate result, as above
					"FilteredImagesCount", "LatestFiltered", // as above, similarly
				},
			},
			want: Container{
				Name:                 "container-ordered2",
				Available:            []image.Info{newSemver, currentSemver, oldSemver, testImage},
				AvailableImagesCount: 4,
				LatestFiltered:       newSemver,
				FilteredImagesCount:  3,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewContainer(tt.args.name, justSlice(tt.args.images), tt.args.currentImage, tt.args.tagPattern, tt.args.fields)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}
