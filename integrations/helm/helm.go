package helm

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8shelm "k8s.io/helm/pkg/helm"
	rls "k8s.io/helm/pkg/proto/hapi/services"
)

type TillerOptions struct {
	IP        string
	Port      string
	Namespace string
}

// Helm struct provides access to helm client
type Helm struct {
	logger log.Logger
	Host   string
	*k8shelm.Client
}

// NewClient creates a new helm client
func newClient(kubeClient *kubernetes.Clientset, opts TillerOptions) (*k8shelm.Client, error) {
	host, err := tillerHost(kubeClient, opts)
	if err != nil {
		return &k8shelm.Client{}, err
	}

	return k8shelm.NewClient(k8shelm.Host(host)), nil
}

func ClientSetup(logger log.Logger, kubeClient *kubernetes.Clientset, tillerOpts TillerOptions) *k8shelm.Client {
	var helmClient *k8shelm.Client
	var err error
	for {
		helmClient, err = newClient(kubeClient, tillerOpts)
		if err != nil {
			logger.Log("error", fmt.Sprintf("Error creating helm client: %v", err))
			time.Sleep(20 * time.Second)
			continue
		}
		logger.Log("info", "Helm client set up")
		break
	}
	return helmClient
}

// GetTillerVersion retrieves tiller version
func GetTillerVersion(cl k8shelm.Client, h string) (string, error) {
	var v *rls.GetVersionResponse
	var err error
	voption := k8shelm.VersionOption(k8shelm.Host(h))
	if v, err = cl.GetVersion(voption); err == nil {
		return "", fmt.Errorf("error getting tiller version: %v", err)
	}

	return v.GetVersion().String(), nil
}

// TODO ... set up based on the tiller existing in the cluster, if no ops given
func tillerHost(kubeClient *kubernetes.Clientset, opts TillerOptions) (string, error) {
	var ts *corev1.Service
	var err error
	var ip string
	var port string

	if opts.IP == "" {
		ts, err = kubeClient.CoreV1().Services(opts.Namespace).Get("tiller-deploy", metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		ip = ts.Spec.ClusterIP
		port = fmt.Sprintf("%v", ts.Spec.Ports[0].Port)
	}

	if opts.IP != "" {
		ip = opts.IP
	}
	if opts.Port != "" {
		port = fmt.Sprintf("%v", opts.Port)
	}

	return fmt.Sprintf("%s:%s", ip, port), nil
}
