package release

import (
	"testing"

	"github.com/stretchr/testify/assert"
	flux_v1beta1 "github.com/weaveworks/flux/integrations/apis/flux.weave.works/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/helm/pkg/chartutil"
)

func TestValues(t *testing.T) {
	falseVal := false

	chartValues, _ := chartutil.ReadValues([]byte(`image:
  tag: 1.1.1
valuesDict:
  chart: true`))

	client := fake.NewSimpleClientset(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "release-configmap",
				Namespace: "flux",
			},
			Data: map[string]string{
				"values.yaml": `valuesDict:
  configmap: true`,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "release-secret",
				Namespace: "flux",
			},
			Data: map[string][]byte{
				"values.yaml": []byte(`valuesDict:
  secret: true`),
			},
		},
	)

	valuesFromSource := []flux_v1beta1.ValuesFromSource{
		flux_v1beta1.ValuesFromSource{
			ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "release-configmap",
				},
				Key:      "values.yaml",
				Optional: &falseVal,
			},
			SecretKeyRef:      nil,
			ExternalSourceRef: nil,
			ChartFileRef:      nil,
		},
		flux_v1beta1.ValuesFromSource{
			ConfigMapKeyRef: nil,
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "release-secret",
				},
				Key:      "values.yaml",
				Optional: &falseVal,
			},
			ExternalSourceRef: nil,
			ChartFileRef:      nil,
		}}

	values, err := Values(client.CoreV1(), "flux", "", valuesFromSource, chartValues)
	assert.NoError(t, err)
	assert.Equal(t, "1.1.1", values["image"].(map[string]interface{})["tag"])
	assert.NotNil(t, values["valuesDict"].(map[string]interface{})["chart"])
	assert.NotNil(t, values["valuesDict"].(map[string]interface{})["configmap"])
	assert.NotNil(t, values["valuesDict"].(map[string]interface{})["secret"])
}
