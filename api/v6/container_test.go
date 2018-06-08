package v6

import (
	"reflect"
	"testing"

	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/update"
)

func TestNewContainer(t *testing.T) {

	testImage := image.Info{ImageID: "test"}

	type args struct {
		name         string
		images       update.ImageInfos
		currentImage image.Info
		tagPattern   string
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
				images:       update.ImageInfos{testImage},
				currentImage: testImage,
				tagPattern:   "*",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewContainer(tt.args.name, tt.args.images, tt.args.currentImage, tt.args.tagPattern, tt.args.fields)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewContainer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewContainer() = %v, want %v", got, tt.want)
			}
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
			name: "Filter",
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
