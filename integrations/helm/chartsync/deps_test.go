package chartsync

import (
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
)

func Test_updateDependencies(t *testing.T) {
	helmhome, err := ioutil.TempDir("", "flux-helm")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(helmhome)
	cmd := exec.Command("helm", "init", "--client-only", "--home", helmhome)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	type args struct {
		chartDir string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Chart without dependencies",
			args: args{
				chartDir: "test/chart-without-deps",
			},
			wantErr: false,
		},
		{
			name: "non-existent chart",
			args: args{
				chartDir: "test/folder-doesnt-exist",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := updateDependencies(tt.args.chartDir, helmhome); (err != nil) != tt.wantErr {
				t.Errorf("updateDependencies() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
