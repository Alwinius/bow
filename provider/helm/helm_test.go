package helm

import (
	"reflect"
	"testing"

	"github.com/alwinius/keel/approvals"
	"github.com/alwinius/keel/cache/memory"
	"github.com/alwinius/keel/extension/notification"
	"github.com/alwinius/keel/internal/policy"
	"github.com/alwinius/keel/types"
	"github.com/ghodss/yaml"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/proto/hapi/chart"
	hapi_release5 "k8s.io/helm/pkg/proto/hapi/release"
	rls "k8s.io/helm/pkg/proto/hapi/services"
)

func approver() *approvals.DefaultManager {
	cache := memory.NewMemoryCache()

	return approvals.New(cache)
}

type fakeSender struct {
	sentEvent types.EventNotification
}

func (s *fakeSender) Configure(cfg *notification.Config) (bool, error) {
	return true, nil
}

func (s *fakeSender) Send(event types.EventNotification) error {
	s.sentEvent = event
	return nil
}

type fakeImplementer struct {
	listReleasesResponse *rls.ListReleasesResponse

	// updated info
	updatedRlsName string
	updatedChart   *chart.Chart
	updatedOptions []helm.UpdateOption
}

func (i *fakeImplementer) ListReleases(opts ...helm.ReleaseListOption) (*rls.ListReleasesResponse, error) {
	return i.listReleasesResponse, nil
}

func (i *fakeImplementer) UpdateReleaseFromChart(rlsName string, chart *chart.Chart, opts ...helm.UpdateOption) (*rls.UpdateReleaseResponse, error) {
	i.updatedRlsName = rlsName
	i.updatedChart = chart
	i.updatedOptions = opts

	return &rls.UpdateReleaseResponse{
		Release: &hapi_release5.Release{
			Version: 2,
		},
	}, nil
}

// helper function to generate keel configuration
func testingConfigYaml(cfg *KeelChartConfig) (vals chartutil.Values, err error) {
	root := &Root{Keel: *cfg}
	bts, err := yaml.Marshal(root)
	if err != nil {
		return nil, err
	}

	return chartutil.ReadValues(bts)
}

func TestGetChartPolicy(t *testing.T) {

	chartVals := `
name: al Rashid
where:
  city: Basrah
  title: caliph
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.1.0

keel:  
  policy: all  
  trigger: poll  
  images:
    - repository: image.repository
      tag: image.tag

`

	fakeImpl := &fakeImplementer{
		listReleasesResponse: &rls.ListReleasesResponse{
			Releases: []*hapi_release5.Release{
				&hapi_release5.Release{
					Name: "release-1",
					Chart: &chart.Chart{
						Values:   &chart.Config{Raw: chartVals},
						Metadata: &chart.Metadata{Name: "app-x"},
					},
					Config: &chart.Config{Raw: ""},
				},
			},
		},
	}

	releases, err := fakeImpl.ListReleases()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	policyFound := false

	for _, release := range releases.Releases {

		vals, err := values(release.Chart, release.Config)
		if err != nil {
			t.Fatalf("failed to get values: %s", err)
		}

		cfg, err := getKeelConfig(vals)
		if err != nil {
			t.Errorf("failed to get image paths: %s", err)
		}

		if cfg.Plc.Name() == policy.SemverPolicyTypeAll.String() {
			policyFound = true
		}
	}

	if !policyFound {
		t.Errorf("policy not found")
	}
}

func TestGetChartPolicyFromProm(t *testing.T) {

	fakeImpl := &fakeImplementer{
		listReleasesResponse: &rls.ListReleasesResponse{
			Releases: []*hapi_release5.Release{
				&hapi_release5.Release{
					Name: "release-1",
					Chart: &chart.Chart{
						Values:   &chart.Config{Raw: promChartValues},
						Metadata: &chart.Metadata{Name: "app-x"},
					},
					Config: &chart.Config{Raw: ""},
				},
			},
		},
	}

	releases, err := fakeImpl.ListReleases()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	policyFound := false

	for _, release := range releases.Releases {

		vals, err := values(release.Chart, release.Config)
		if err != nil {
			t.Fatalf("failed to get values: %s", err)
		}

		cfg, err := getKeelConfig(vals)
		if err != nil {
			t.Errorf("failed to get image paths: %s", err)
		}

		if cfg.Plc.Name() == policy.SemverPolicyTypeAll.String() {
			policyFound = true
		}
	}

	if !policyFound {
		t.Errorf("policy not found")
	}
}

func TestGetTrackedReleases(t *testing.T) {

	chartVals := `
name: chart-x
where:
  city: kaunas
  title: hmm
image:
  repository: gcr.io/v2-namespace/bye-world
  tag: 1.1.0

image2:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.2.0 

keel:  
  policy: all  
  trigger: poll  
  images:
    - repository: image.repository
      tag: image.tag

`

	fakeImpl := &fakeImplementer{
		listReleasesResponse: &rls.ListReleasesResponse{
			Releases: []*hapi_release5.Release{
				&hapi_release5.Release{
					Name: "release-1",
					Chart: &chart.Chart{
						Values:   &chart.Config{Raw: chartVals},
						Metadata: &chart.Metadata{Name: "app-x"},
					},
					Config: &chart.Config{Raw: ""},
				},
			},
		},
	}

	prov := NewProvider(fakeImpl, &fakeSender{}, approver())

	tracked, _ := prov.TrackedImages()

	if tracked[0].Image.Remote() != "gcr.io/v2-namespace/bye-world:1.1.0" {
		t.Errorf("unexpected image: %s", tracked[0].Image.Remote())
	}
}

func TestGetTrackedReleasesWithoutKeelConfig(t *testing.T) {

	chartVals := `
name: chart-x
where:
  city: kaunas
  title: hmm
image:
  repository: gcr.io/v2-namespace/bye-world
  tag: 1.1.0

image2:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.2.0 

`

	fakeImpl := &fakeImplementer{
		listReleasesResponse: &rls.ListReleasesResponse{
			Releases: []*hapi_release5.Release{
				&hapi_release5.Release{
					Name: "release-1",
					Chart: &chart.Chart{
						Values:   &chart.Config{Raw: chartVals},
						Metadata: &chart.Metadata{Name: "app-x"},
					},
					Config: &chart.Config{Raw: ""},
				},
			},
		},
	}

	prov := NewProvider(fakeImpl, &fakeSender{}, approver())

	tracked, _ := prov.TrackedImages()

	if len(tracked) != 0 {
		t.Errorf("didn't expect to find any tracked releases, found: %d", len(tracked))
	}

}

func TestGetTrackedReleasesTotallyNonStandard(t *testing.T) {

	chartVals := `
name: chart-x
where:
  city: kaunas
  title: hmm
ihavemyownstandard:
  repo: gcr.io/v2-namespace/bye-world
  version: 1.1.0

image2:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.2.0 

keel:  
  policy: all  
  trigger: poll  
  images:
    - repository: ihavemyownstandard.repo
      tag: ihavemyownstandard.version

`

	fakeImpl := &fakeImplementer{
		listReleasesResponse: &rls.ListReleasesResponse{
			Releases: []*hapi_release5.Release{
				&hapi_release5.Release{
					Name: "release-1",
					Chart: &chart.Chart{
						Values:   &chart.Config{Raw: chartVals},
						Metadata: &chart.Metadata{Name: "app-x"},
					},
					Config: &chart.Config{Raw: ""},
				},
			},
		},
	}

	prov := NewProvider(fakeImpl, &fakeSender{}, approver())

	tracked, _ := prov.TrackedImages()

	if tracked[0].Image.Remote() != "gcr.io/v2-namespace/bye-world:1.1.0" {
		t.Errorf("unexpected image: %s", tracked[0].Image.Remote())
	}
}

func TestGetTriggerFromConfig(t *testing.T) {
	vals, err := testingConfigYaml(&KeelChartConfig{Trigger: types.TriggerTypePoll, Policy: "all"})
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}

	cfg, err := getKeelConfig(vals)
	if err != nil {
		t.Fatalf("failed to get image paths: %s", err)
	}

	if cfg.Trigger != types.TriggerTypePoll {
		t.Errorf("invalid trigger: %s", cfg.Trigger)
	}
}

func TestGetPolicyFromConfig(t *testing.T) {
	vals, err := testingConfigYaml(&KeelChartConfig{Policy: "all"})
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}

	cfg, err := getKeelConfig(vals)
	if err != nil {
		t.Errorf("failed to get image paths: %s", err)
	}

	// if cfg.Policy != types.PolicyTypeAll {
	if cfg.Plc.Name() != policy.SemverPolicyTypeAll.String() {
		t.Errorf("invalid policy: %s", cfg.Policy)
	}
}

func TestGetImagesFromConfig(t *testing.T) {
	vals, err := testingConfigYaml(&KeelChartConfig{Policy: "all", Images: []ImageDetails{
		ImageDetails{
			RepositoryPath: "repopath",
			TagPath:        "tagpath",
		},
	}})
	if err != nil {
		t.Fatalf("Failed to load testdata: %s", err)
	}

	cfg, err := getKeelConfig(vals)
	if err != nil {
		t.Errorf("failed to get image paths: %s", err)
	}

	if cfg.Images[0].RepositoryPath != "repopath" {
		t.Errorf("invalid repo path: %s", cfg.Images[0].RepositoryPath)
	}

	if cfg.Images[0].TagPath != "tagpath" {
		t.Errorf("invalid tag path: %s", cfg.Images[0].TagPath)
	}
}

func TestUpdateRelease(t *testing.T) {
	// imp := NewHelmImplementer("192.168.99.100:30083")

	chartVals := `
name: al Rashid
where:
  city: Basrah
  title: caliph
image:
  repository: karolisr/webhook-demo
  tag: 0.0.10

keel:  
  policy: all  
  trigger: poll  
  images:
    - repository: image.repository
      tag: image.tag

`
	myChart := &chart.Chart{
		Values: &chart.Config{Raw: chartVals},
	}

	fakeImpl := &fakeImplementer{
		listReleasesResponse: &rls.ListReleasesResponse{
			Releases: []*hapi_release5.Release{
				&hapi_release5.Release{
					Name:   "release-1",
					Chart:  myChart,
					Config: &chart.Config{Raw: ""},
				},
			},
		},
	}

	provider := NewProvider(fakeImpl, &fakeSender{}, approver())

	err := provider.processEvent(&types.Event{
		Repository: types.Repository{
			Name: "karolisr/webhook-demo",
			Tag:  "0.0.11",
		},
	})
	if err != nil {
		t.Errorf("failed to process event, error: %s", err)
	}

	// checking updated release
	if fakeImpl.updatedChart != myChart {
		t.Errorf("wrong chart updated")
	}

	if fakeImpl.updatedRlsName != "release-1" {
		t.Errorf("unexpected release updated: %s", fakeImpl.updatedRlsName)
	}
}

var pollingValues = `
name: al Rashid
where:
  city: Basrah
  title: caliph
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.1.0

keel:  
  policy: all  
  trigger: poll 
  pollSchedule: "@every 12m" 
  images:
    - repository: image.repository
      tag: image.tag

`

func TestGetPollingSchedule(t *testing.T) {
	vals, _ := chartutil.ReadValues([]byte(pollingValues))

	cfg, err := getKeelConfig(vals)
	if err != nil {
		t.Errorf("failed to get config: %s", err)
	}

	if cfg.PollSchedule != "@every 12m" {
		t.Errorf("unexpected polling schedule: %s", cfg.PollSchedule)
	}
}

func Test_getKeelConfig(t *testing.T) {

	var valuesBasicStr = `
name: al Rashid
where:
  city: Basrah
  title: caliph
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.1.0

keel:  
  policy: all  
  images:
    - repository: image.repository
      tag: image.tag

`
	valuesBasic, _ := chartutil.ReadValues([]byte(valuesBasicStr))

	var valuesChannelsStr = `
name: al Rashid
where:
  city: Basrah
  title: caliph
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.1.0

keel:
  policy: all
  notificationChannels:
    - chan1
    - chan2
  images:
    - repository: image.repository
      tag: image.tag

`
	valuesChannels, _ := chartutil.ReadValues([]byte(valuesChannelsStr))

	var valuesPollStr = `
name: al Rashid
where:
  city: Basrah
  title: caliph
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.1.0

keel:  
  policy: major  
  trigger: poll
  pollSchedule: "@every 30m"
  images:
    - repository: image.repository
      tag: image.tag
      imagePullSecret: such-secret
`
	valuesPoll, _ := chartutil.ReadValues([]byte(valuesPollStr))

	type args struct {
		vals chartutil.Values
	}
	tests := []struct {
		name    string
		args    args
		want    *KeelChartConfig
		wantErr bool
	}{
		{
			name: "correct config",
			args: args{vals: valuesBasic},
			want: &KeelChartConfig{
				Policy:  "all",
				Trigger: types.TriggerTypeDefault,
				Images: []ImageDetails{
					ImageDetails{RepositoryPath: "image.repository", TagPath: "image.tag"},
				},
				Plc: policy.NewSemverPolicy(policy.SemverPolicyTypeAll),
			},
		},
		{
			name: "custom notification channels",
			args: args{vals: valuesChannels},
			want: &KeelChartConfig{
				Policy:               "all",
				Trigger:              types.TriggerTypeDefault,
				NotificationChannels: []string{"chan1", "chan2"},
				Images: []ImageDetails{
					ImageDetails{RepositoryPath: "image.repository", TagPath: "image.tag"},
				},
				Plc: policy.NewSemverPolicy(policy.SemverPolicyTypeAll),
			},
		},
		{
			name: "correct polling config",
			args: args{vals: valuesPoll},
			want: &KeelChartConfig{
				Policy:       "major",
				Trigger:      types.TriggerTypePoll,
				PollSchedule: "@every 30m",
				Images: []ImageDetails{
					ImageDetails{RepositoryPath: "image.repository", TagPath: "image.tag", ImagePullSecret: "such-secret"},
				},
				Plc: policy.NewSemverPolicy(policy.SemverPolicyTypeMajor),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getKeelConfig(tt.args.vals)
			if (err != nil) != tt.wantErr {
				t.Errorf("getKeelConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getKeelConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetChartMatchTag(t *testing.T) {

	chartVals := `
name: al Rashid
where:
  city: Basrah
  title: caliph
image:
  repository: gcr.io/v2-namespace/hello-world
  tag: 1.1.0

keel:  
  policy: all  
  trigger: poll
  matchTag: true
  images:
    - repository: image.repository
      tag: image.tag

`

	fakeImpl := &fakeImplementer{
		listReleasesResponse: &rls.ListReleasesResponse{
			Releases: []*hapi_release5.Release{
				&hapi_release5.Release{
					Name: "release-1",
					Chart: &chart.Chart{
						Values:   &chart.Config{Raw: chartVals},
						Metadata: &chart.Metadata{Name: "app-x"},
					},
					Config: &chart.Config{Raw: ""},
				},
			},
		},
	}

	releases, err := fakeImpl.ListReleases()
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	policyFound := false

	for _, release := range releases.Releases {

		vals, err := values(release.Chart, release.Config)
		if err != nil {
			t.Fatalf("failed to get values: %s", err)
		}

		cfg, err := getKeelConfig(vals)
		if err != nil {
			t.Errorf("failed to get image paths: %s", err)
		}

		if cfg.Plc.Name() == policy.SemverPolicyTypeAll.String() {
			policyFound = true
		}
		if !cfg.MatchTag {
			t.Errorf("expected to find 'matchTag' == true ")
		}
	}

	if !policyFound {
		t.Errorf("policy not found")
	}
}
