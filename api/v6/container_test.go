package v6

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/policy"
)

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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewContainer(tt.args.name, tt.args.images, tt.args.currentImage, tt.args.tagPattern, tt.args.fields)
			assert.Equal(t, tt.wantErr, err != nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFilterContainerFields(t *testing.T) {
	testContainer := Container{
		Name:                    "test",
		Current:                 image.Info{ImageID: "123"},
		LatestFiltered:          image.Info{ImageID: "123"},
		Available:               []image.Info{{ImageID: "123"}},
		AvailableError:          "test",
		AvailableImagesCount:    1,
		NewAvailableImagesCount: 2,
		FilteredImagesCount:     3,
		NewFilteredImagesCount:  4,
	}

	type args struct {
		container Container
		fields    []string
	}
	tests := []struct {
		name    string
		args    args
		want    Container
		wantErr bool
	}{
		{
			name: "Default fields",
			args: args{
				container: testContainer,
			},
			want:    testContainer,
			wantErr: false,
		},
		{
			name: "FilterImages",
			args: args{
				container: testContainer,
				fields:    []string{"Name", "Available", "NewAvailableImagesCount", "NewFilteredImagesCount"},
			},
			want: Container{
				Name:                    "test",
				Available:               []image.Info{{ImageID: "123"}},
				NewAvailableImagesCount: 2,
				NewFilteredImagesCount:  4,
			},
			wantErr: false,
		},
		{
			name: "Invalid field",
			args: args{
				container: testContainer,
				fields:    []string{"Invalid"},
			},
			want:    Container{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterContainerFields(tt.args.container, tt.args.fields)
			if (err != nil) != tt.wantErr {
				t.Errorf("FilterContainerFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FilterContainerFields() = %v, want %v", got, tt.want)
			}
		})
	}
}
