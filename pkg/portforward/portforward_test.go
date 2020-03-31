package portforward

// based on https://github.com/justinbarrick/go-k8s-portforward
// licensed under the Apache License 2.0

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newPod(name string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: labels,
			Name:   name,
		},
	}
}

func TestFindPodByLabels(t *testing.T) {
	pf := PortForward{
		Clientset: fake.NewSimpleClientset(
			newPod("mypod1", map[string]string{
				"name": "other",
			}),
			newPod("mypod2", map[string]string{
				"name": "flux",
			}),
			newPod("mypod3", map[string]string{})),
		Labels: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"name": "flux",
			},
		},
	}

	pod, err := pf.findPodByLabels()
	assert.Nil(t, err)
	assert.Equal(t, "mypod2", pod)
}

func TestFindPodByLabelsNoneExist(t *testing.T) {
	pf := PortForward{
		Clientset: fake.NewSimpleClientset(
			newPod("mypod1", map[string]string{
				"name": "other",
			})),
		Labels: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"name": "flux",
			},
		},
	}

	_, err := pf.findPodByLabels()
	assert.NotNil(t, err)
	assert.Equal(t, "Could not find running pod for selector: labels \"name=flux\"", err.Error())
}

func TestFindPodByLabelsMultiple(t *testing.T) {
	pf := PortForward{
		Clientset: fake.NewSimpleClientset(
			newPod("mypod1", map[string]string{
				"name": "flux",
			}),
			newPod("mypod2", map[string]string{
				"name": "flux",
			}),
			newPod("mypod3", map[string]string{})),
		Labels: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"name": "flux",
			},
		},
	}

	_, err := pf.findPodByLabels()
	assert.NotNil(t, err)
	assert.Equal(t, "Ambiguous pod: found more than one pod for selector: labels \"name=flux\"", err.Error())
}

func TestFindPodByLabelsExpression(t *testing.T) {
	pf := PortForward{
		Clientset: fake.NewSimpleClientset(
			newPod("mypod1", map[string]string{
				"name": "lol",
			}),
			newPod("mypod2", map[string]string{
				"name": "fluxd",
			}),
			newPod("mypod3", map[string]string{})),
		Labels: metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "name",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"flux", "fluxd"},
				},
			},
		},
	}

	pod, err := pf.findPodByLabels()
	assert.Nil(t, err)
	assert.Equal(t, "mypod2", pod)
}

func TestFindPodByLabelsExpressionNotFound(t *testing.T) {
	pf := PortForward{
		Clientset: fake.NewSimpleClientset(
			newPod("mypod1", map[string]string{
				"name": "lol",
			}),
			newPod("mypod2", map[string]string{
				"name": "lol",
			}),
			newPod("mypod3", map[string]string{})),
		Labels: metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				metav1.LabelSelectorRequirement{
					Key:      "name",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"flux", "fluxd"},
				},
			},
		},
	}

	_, err := pf.findPodByLabels()
	assert.NotNil(t, err)
	assert.Equal(t, "Could not find running pod for selector: labels \"name in (flux,fluxd)\"", err.Error())
}

func TestGetPodNameNameSet(t *testing.T) {
	pf := PortForward{
		Name: "hello",
	}

	pod, err := pf.getPodName()
	assert.Nil(t, err)
	assert.Equal(t, "hello", pod)
}

func TestGetPodNameNoNameSet(t *testing.T) {
	pf := PortForward{
		Clientset: fake.NewSimpleClientset(
			newPod("mypod", map[string]string{
				"name": "flux",
			})),
		Labels: metav1.LabelSelector{
			MatchLabels: map[string]string{
				"name": "flux",
			},
		},
	}

	pod, err := pf.getPodName()
	assert.Nil(t, err)
	assert.Equal(t, "mypod", pod)
	assert.Equal(t, pf.Name, pod)
}

func TestGetFreePort(t *testing.T) {
	pf := PortForward{}
	port, err := pf.getFreePort()
	assert.Nil(t, err)
	assert.NotZero(t, port)
}

func TestGetListenPort(t *testing.T) {
	pf := PortForward{
		ListenPort: 80,
	}

	port, err := pf.getListenPort()
	assert.Nil(t, err)
	assert.Equal(t, 80, port)
}

func TestGetListenPortRandom(t *testing.T) {
	pf := PortForward{}

	port, err := pf.getListenPort()
	assert.Nil(t, err)
	assert.NotZero(t, port)
	assert.Equal(t, pf.ListenPort, port)
}
