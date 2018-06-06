package releasesync

import (
	"fmt"
	"os"
	"testing"

	proto "github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	"k8s.io/helm/pkg/helm"
	hapi_chart "k8s.io/helm/pkg/proto/hapi/chart"
	hapi_release "k8s.io/helm/pkg/proto/hapi/release"

	"github.com/go-kit/kit/log"
	ifv1 "github.com/weaveworks/flux/apis/helm.integrations.flux.weave.works/v1alpha2"
	helmgit "github.com/weaveworks/flux/integrations/helm/git"
	"github.com/weaveworks/flux/integrations/helm/release"
	chartrelease "github.com/weaveworks/flux/integrations/helm/release"
)

type installReq struct {
	checkoutDir string
	releaseName string
	fhr         ifv1.FluxHelmRelease
	action      chartrelease.Action
	opts        chartrelease.InstallOptions
}

type installResult struct {
	release hapi_release.Release
	err     error
}

type install struct {
	installReq
	installResult
}

type mockReleaser struct {
	current    map[string][]chartrelease.DeployInfo
	deployed   map[string]*hapi_release.Release
	configSync *helmgit.Checkout
	installs   []install
}

func (r *mockReleaser) GetCurrent() (map[string][]chartrelease.DeployInfo, error) {
	if r.current == nil {
		return nil, fmt.Errorf("failed to fetch current releases")
	}
	return r.current, nil
}

func (r *mockReleaser) GetDeployedRelease(name string) (*hapi_release.Release, error) {
	if _, present := r.deployed[name]; !present {
		return nil, fmt.Errorf("no release hamed %q", name)
	}
	return r.deployed[name], nil
}

func (r *mockReleaser) ConfigSync() *helmgit.Checkout {
	return r.configSync
}

func (r *mockReleaser) Install(checkout *helmgit.Checkout,
	releaseName string,
	fhr ifv1.FluxHelmRelease,
	action chartrelease.Action,
	opts chartrelease.InstallOptions) (*hapi_release.Release, error) {
	req := installReq{
		checkoutDir: checkout.Dir,
		releaseName: releaseName,
		fhr:         fhr,
		action:      action,
		opts:        opts}
	cmpopts := cmp.AllowUnexported(installReq{})
	for _, i := range r.installs {
		if cmp.Equal(i.installReq, req, cmpopts) {
			return &i.installResult.release, i.installResult.err
		}
	}
	return nil, fmt.Errorf("unexpected request: %+v", req)
}

func makeCurRel(ns string, relNames ...string) map[string]map[string]struct{} {
	m := make(map[string]map[string]struct{})
	m[ns] = make(map[string]struct{})
	for _, relName := range relNames {
		m[ns][relName] = struct{}{}
	}
	return m
}

func mergeCurRels(a, b map[string]map[string]struct{}) map[string]map[string]struct{} {
	m := make(map[string]map[string]struct{})
	for ns := range a {
		m[ns] = a[ns]
	}
	for ns := range b {
		if _, present := m[ns]; present {
			panic("ns '" + ns + "' present in both a and b")
		}
		m[ns] = b[ns]
	}
	return m
}

func makeCustRes(ns string, relNames ...string) map[string]map[string]ifv1.FluxHelmRelease {
	m := make(map[string]map[string]ifv1.FluxHelmRelease)
	m[ns] = make(map[string]ifv1.FluxHelmRelease)
	for _, relName := range relNames {
		m[ns][relName] = ifv1.FluxHelmRelease{}
	}
	return m
}

func mergeCustRes(a, b map[string]map[string]ifv1.FluxHelmRelease) map[string]map[string]ifv1.FluxHelmRelease {
	m := make(map[string]map[string]ifv1.FluxHelmRelease)
	for ns := range a {
		m[ns] = a[ns]
	}
	for ns := range b {
		if _, present := m[ns]; present {
			panic("ns '" + ns + "' present in both a and b")
		}
		m[ns] = b[ns]
	}
	return m
}

func TestAddDeletedReleasesToSync(t *testing.T) {
	var zeromap = make(map[string][]chartRelease)
	var tests = []struct {
		msg             string
		currentReleases map[string]map[string]struct{}
		customResources map[string]map[string]ifv1.FluxHelmRelease
		want            map[string][]chartRelease
	}{
		{
			msg:             "no-op, zero resources",
			currentReleases: makeCurRel("ns1", "r1"),
			customResources: make(map[string]map[string]ifv1.FluxHelmRelease),
			want:            zeromap,
		},
		{
			msg:             "no-op, equality",
			currentReleases: makeCurRel("ns1", "r1"),
			customResources: makeCustRes("ns1", "r1"),
			want:            zeromap,
		},
		{
			msg:             "add missing release",
			currentReleases: makeCurRel("ns1"),
			customResources: makeCustRes("ns1", "r1"),
			want: map[string][]chartRelease{"ns1": []chartRelease{
				chartRelease{releaseName: "r1", action: release.InstallAction}}},
		},
		{
			msg:             "add missing release new namespace",
			currentReleases: makeCurRel("ns1"),
			customResources: makeCustRes("ns2", "r1"),
			want: map[string][]chartRelease{"ns2": []chartRelease{
				chartRelease{releaseName: "r1", action: release.InstallAction}}},
		},
		{
			msg: "add missing releases multi namespace",
			currentReleases: mergeCurRels(makeCurRel("ns1"),
				makeCurRel("ns2", "r2")),
			customResources: mergeCustRes(makeCustRes("ns1", "r1"),
				makeCustRes("ns2", "r2", "r3")),
			want: map[string][]chartRelease{
				"ns1": []chartRelease{chartRelease{releaseName: "r1", action: release.InstallAction}},
				"ns2": []chartRelease{chartRelease{releaseName: "r3", action: release.InstallAction}},
			},
		},
	}

	opts := cmp.AllowUnexported(chartRelease{})
	rs := New(log.NewLogfmtLogger(os.Stdout), nil)
	for i, test := range tests {
		var got = make(map[string][]chartRelease)
		err := rs.addDeletedReleasesToSync(got, test.currentReleases, test.customResources)
		if err != nil {
			t.Errorf("%d %s: got error: %v", i, test.msg, err)
		}
		if diff := cmp.Diff(got, test.want, opts); diff != "" {
			t.Errorf("%d %s: diff (-got +want)\n%s", i, test.msg, diff)
		}

	}
}

func config(vals map[string]string) *hapi_chart.Config {
	pv := make(map[string]*hapi_chart.Value)
	for k, v := range vals {
		pv[k] = &hapi_chart.Value{Value: v}
	}

	c := &hapi_chart.Config{Values: pv}
	// Marshalling to get c.Raw populated
	data, _ := proto.Marshal(c)
	_ = proto.Unmarshal(data, c)
	return c
}

func relvals(name string, vals string) *hapi_release.Release {
	rel := helm.ReleaseMock(&helm.MockReleaseOptions{Name: name})
	rel.Config.Raw = vals
	return rel
}

func relchart(name string, chartname string, chartver string, tmplname string) *hapi_release.Release {
	return helm.ReleaseMock(&helm.MockReleaseOptions{Name: name, Chart: &hapi_chart.Chart{
		Metadata: &hapi_chart.Metadata{
			Name:    chartname,
			Version: chartver,
		},
		Templates: []*hapi_chart.Template{
			{Name: tmplname, Data: []byte(helm.MockManifest)},
		},
	}})
}

func TestAddExistingReleasesToSync(t *testing.T) {
	var zeromap = make(map[string][]chartRelease)
	var tests = []struct {
		msg             string
		currentReleases map[string]map[string]struct{}
		customResources map[string]map[string]ifv1.FluxHelmRelease
		want            map[string][]chartRelease
		releaser        chartrelease.Releaser
		wanterror       error
	}{
		{
			msg:             "no-op, zero resources",
			currentReleases: makeCurRel("ns1", "r1"),
			customResources: make(map[string]map[string]ifv1.FluxHelmRelease),
			want:            zeromap,
		},
		{
			msg:             "no-op, no overlap",
			currentReleases: makeCurRel("ns1", "r1"),
			customResources: mergeCustRes(
				makeCustRes("ns1", "r2"),
				makeCustRes("ns2", "r1")),
			want: zeromap,
		},
		{
			msg:             "get deployed release fails",
			currentReleases: makeCurRel("ns1", "r1"),
			customResources: makeCustRes("ns1", "r1"),
			releaser:        &mockReleaser{},
			wanterror:       fmt.Errorf("no release hamed %q", "r1"),
		},
		{
			msg:             "dry-run install fails",
			currentReleases: makeCurRel("ns1", "r1"),
			customResources: makeCustRes("ns1", "r1"),
			releaser: &mockReleaser{
				configSync: &helmgit.Checkout{Dir: "dir"},
				deployed: map[string]*hapi_release.Release{
					"r1": relvals("r1", `k1: "v1"`),
				},
				installs: []install{
					dryinst("r1", *relvals("", ""), fmt.Errorf("dry-run failed")),
				},
			},
			wanterror: fmt.Errorf("dry-run failed"),
		},
		{
			msg: "r1 vals changed, r2 unchanged",
			currentReleases: mergeCurRels(
				makeCurRel("ns1", "r1"),
				makeCurRel("ns2", "r2")),
			customResources: mergeCustRes(
				makeCustRes("ns1", "r1"),
				makeCustRes("ns2", "r2")),
			releaser: &mockReleaser{
				configSync: &helmgit.Checkout{Dir: "dir"},
				deployed: map[string]*hapi_release.Release{
					"r1": relvals("r1", `k1: "v1"`),
					"r2": relvals("r2", `k1: "v1"`),
				},
				installs: []install{
					dryinst("r1", *relvals("r1", `k1: "v2"`), nil),
					dryinst("r2", *relvals("r2", `k1: "v1"`), nil),
				},
			},
			want: map[string][]chartRelease{"ns1": []chartRelease{
				chartRelease{releaseName: "r1", action: release.Action("UPDATE")},
			}},
		},
		{
			msg: "r1/r2/r3 charts changed, r4 unchanged",
			currentReleases: mergeCurRels(mergeCurRels(
				makeCurRel("ns1", "r1"),
				makeCurRel("ns2", "r2")),
				makeCurRel("ns3", "r3", "r4")),
			customResources: mergeCustRes(mergeCustRes(
				makeCustRes("ns1", "r1"),
				makeCustRes("ns2", "r2")),
				makeCustRes("ns3", "r3", "r4")),
			releaser: &mockReleaser{
				configSync: &helmgit.Checkout{Dir: "dir"},
				deployed: map[string]*hapi_release.Release{
					"r1": relchart("r1", "c1", "v0.1.0", "templates/foo.tpl"),
					"r2": relchart("r2", "c2", "v0.1.0", "templates/bar.tpl"),
					"r3": relchart("r3", "c3", "v0.1.0", "templates/baz.tpl"),
					"r4": relchart("r4", "c4", "v0.1.0", "templates/qux.tpl"),
				},
				installs: []install{
					dryinst("r1", *relchart("r1", "c-", "v0.1.0", "templates/foo.tpl"), nil),
					dryinst("r2", *relchart("r2", "c2", "v0.1.1", "templates/bar.tpl"), nil),
					dryinst("r3", *relchart("r3", "c3", "v0.1.0", "templates/foo.tpl"), nil),
					dryinst("r4", *relchart("r4", "c4", "v0.1.0", "templates/qux.tpl"), nil),
				},
			},
			want: map[string][]chartRelease{
				"ns1": []chartRelease{chartRelease{releaseName: "r1", action: release.Action("UPDATE")}},
				"ns2": []chartRelease{chartRelease{releaseName: "r2", action: release.Action("UPDATE")}},
				"ns3": []chartRelease{chartRelease{releaseName: "r3", action: release.Action("UPDATE")}},
			},
		},
	}

	opts := cmp.AllowUnexported(chartRelease{})
	for i, test := range tests {
		rs := New(log.NewLogfmtLogger(os.Stdout), test.releaser)
		var got = make(map[string][]chartRelease)
		err := rs.addExistingReleasesToSync(got, test.currentReleases, test.customResources)
		if fmt.Sprintf("%v", err) != fmt.Sprintf("%v", test.wanterror) {
			t.Errorf("%d %s: got error %q, want error %q", i, test.msg, err, test.wanterror)
		}
		if test.wanterror != nil {
			continue
		}
		if diff := cmp.Diff(got, test.want, opts); diff != "" {
			t.Errorf("%d %s: diff (-got +want)\n%s", i, test.msg, diff)
		}

	}
}

func dryinst(relname string, rel hapi_release.Release, err error) install {
	return install{
		installReq{
			checkoutDir: "dir",
			releaseName: relname + "-temp",
			action:      "CREATE",
			opts:        chartrelease.InstallOptions{DryRun: true},
		},
		installResult{rel, err},
	}
}
