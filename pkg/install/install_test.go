package install

import (
	"strings"
	"testing"

	"github.com/instrumenta/kubeval/kubeval"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func testFillInTemplates(t *testing.T, expectedNoManifests int, params TemplateParameters) map[string][]byte {
	manifests, err := FillInTemplates(params)
	assert.NoError(t, err)
	assert.Len(t, manifests, expectedNoManifests)

	for fileName, contents := range manifests {
		validationResults, err := kubeval.Validate(contents)
		assert.NoError(t, err)
		for _, result := range validationResults {
			if len(result.Errors) > 0 {
				t.Errorf("found problems with manifest %s (Kind %s):\ncontent:\n%s\nerrors: %s",
					fileName,
					result.Kind,
					string(contents),
					result.Errors)
			}
		}
	}

	return manifests
}

// Deployment is defined here to avoid pulling in all the k8s dependencies into the install package
type Deployment struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
					SecurityContext *struct {
					} `yaml:"securityContext"`
				} `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

func unmarshalDeployment(t *testing.T, data []byte) *Deployment {
	deployment := &Deployment{}
	if err := yaml.Unmarshal(data, deployment); err != nil {
		t.Errorf("issue decoding memcache-dep.yaml into a deployment: %#v", err)
	}
	return deployment
}

func TestFillInTemplatesAllParameters(t *testing.T) {
	testFillInTemplates(t, 5, TemplateParameters{
		GitURL:             "git@github.com:fluxcd/flux-get-started",
		GitBranch:          "branch",
		GitPaths:           []string{"dir1", "dir2"},
		GitLabel:           "label",
		GitUser:            "User",
		GitEmail:           "this.is@anemail.com",
		Namespace:          "flux",
		GitReadOnly:        false,
		ManifestGeneration: true,
		AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
		AddSecurityContext: true,
	})
}

func TestFillInTemplatesMissingValues(t *testing.T) {
	testFillInTemplates(t, 5, TemplateParameters{
		GitURL:    "git@github.com:fluxcd/flux-get-started",
		GitBranch: "branch",
		GitPaths:  []string{},
		GitLabel:  "label",
	})
}

func TestFillInTemplatesNoMemcached(t *testing.T) {
	testFillInTemplates(t, 3, TemplateParameters{
		GitURL:                  "git@github.com:fluxcd/flux-get-started",
		GitBranch:               "branch",
		GitPaths:                []string{},
		GitLabel:                "label",
		RegistryDisableScanning: true,
	})
}

func TestTestFillInTemplatesAddSecurityContext(t *testing.T) {
	params := TemplateParameters{
		GitURL:             "git@github.com:fluxcd/flux-get-started",
		GitBranch:          "branch",
		GitPaths:           []string{"dir1", "dir2"},
		GitLabel:           "label",
		GitUser:            "User",
		GitEmail:           "this.is@anemail.com",
		Namespace:          "flux",
		GitReadOnly:        false,
		ManifestGeneration: true,
		AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
		AddSecurityContext: true,
	}

	manifests := testFillInTemplates(t, 5, params)

	memDeploy := manifests["memcache-dep.yaml"]
	deployment := unmarshalDeployment(t, memDeploy)

	if len(deployment.Spec.Template.Spec.Containers) < 1 {
		t.Error("incorrect number of containers in deployment")
	}
	container := deployment.Spec.Template.Spec.Containers[0]
	if container.SecurityContext == nil {
		t.Error("security context not found when there should be one")
	}
}

func TestFillInTemplatesNoSecurityContext(t *testing.T) {
	params := TemplateParameters{
		GitURL:             "git@github.com:fluxcd/flux-get-started",
		GitBranch:          "branch",
		GitPaths:           []string{"dir1", "dir2"},
		GitLabel:           "label",
		GitUser:            "User",
		GitEmail:           "this.is@anemail.com",
		Namespace:          "flux",
		GitReadOnly:        false,
		ManifestGeneration: true,
		AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
		AddSecurityContext: false,
	}

	manifests := testFillInTemplates(t, 5, params)
	memDeploy := manifests["memcache-dep.yaml"]

	deployment := unmarshalDeployment(t, memDeploy)
	if len(deployment.Spec.Template.Spec.Containers) < 1 {
		t.Error("incorrect number of containers in deployment")
	}
	container := deployment.Spec.Template.Spec.Containers[0]
	if container.SecurityContext != nil {
		t.Errorf("security context found when there should be none: %#v", container.SecurityContext)
	}
}

func TestFillInTemplatesConfigFile(t *testing.T) {

	configFile := `config1: configuration1
config2: configuration2
config3: configuration3`

	tests := map[string]struct {
		params              TemplateParameters
		configFileName      string
		configFileNameCheck string
		deploymentFileCheck string
	}{
		"configMap": {
			params: TemplateParameters{
				GitURL:             "git@github.com:fluxcd/flux-get-started",
				GitBranch:          "branch",
				GitPaths:           []string{"dir1", "dir2"},
				GitLabel:           "label",
				GitUser:            "User",
				GitEmail:           "this.is@anemail.com",
				Namespace:          "flux",
				ConfigAsConfigMap:  true,
				AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
			},
			configFileName:      "flux-config.yaml",
			configFileNameCheck: "    config2: config",
			deploymentFileCheck: "name: flux-config",
		},
		"secret": {
			params: TemplateParameters{
				GitURL:             "git@github.com:fluxcd/flux-get-started",
				GitBranch:          "branch",
				GitPaths:           []string{"dir1", "dir2"},
				GitLabel:           "label",
				GitUser:            "User",
				GitEmail:           "this.is@anemail.com",
				Namespace:          "flux",
				ConfigAsConfigMap:  false,
				AdditionalFluxArgs: []string{"arg1=foo", "arg2=bar"},
			},
			configFileName: "flux-config.yaml",
			// the following field value is the base64 encoding of the config file string above
			configFileNameCheck: "  flux-config.yaml: Y29uZmlnMTogY29uZmlndXJhdGlvbjEKY29uZmlnMjogY29uZmlndXJhdGlvbjIKY29uZmlnMzogY29uZmlndXJhdGlvbjM=",
			deploymentFileCheck: "secretName: flux-config",
		},
	}

	for name, test := range tests {
		t.Run(name, func(*testing.T) {
			configContent, err := ConfigContent(strings.NewReader(configFile), test.params.ConfigAsConfigMap)
			if err != nil {
				t.Fatal(err)
			}
			test.params.ConfigFileContent = configContent
			manifests := testFillInTemplates(t, test.params)
			for fileName, contents := range manifests {
				if fileName == test.configFileName {
					assert.Contains(t, string(contents), test.configFileNameCheck)
				}
				if fileName == "flux-deployment.yaml" {
					assert.Contains(t, string(contents), test.deploymentFileCheck)
				}
			}
		})
	}
}
