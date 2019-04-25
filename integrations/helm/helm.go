package helm

import (
	"fmt"
	"time"

	"github.com/go-kit/kit/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8shelm "k8s.io/helm/pkg/helm"
	rls "k8s.io/helm/pkg/proto/hapi/services"
	"k8s.io/helm/pkg/tlsutil"
)

const (
	GitOperationTimeout = 30 * time.Second
)

type TillerOptions struct {
	Host        string
	Port        string
	Namespace   string
	TLSVerify   bool
	TLSEnable   bool
	TLSKey      string
	TLSCert     string
	TLSCACert   string
	TLSHostname string
}

// Helm struct provides access to helm client
type Helm struct {
	logger log.Logger
	Host   string
	*k8shelm.Client
}

// newClient creates a new helm client
func newClient(kubeClient *kubernetes.Clientset, opts TillerOptions) (*k8shelm.Client, string, error) {
	host, err := tillerHost(kubeClient, opts)
	if err != nil {
		return &k8shelm.Client{}, "", err
	}

	//host = "tiller-deploy.kube-system:44134"

	options := []k8shelm.Option{k8shelm.Host(host)}
	if opts.TLSVerify || opts.TLSEnable {
		tlsopts := tlsutil.Options{
			KeyFile:            opts.TLSKey,
			CertFile:           opts.TLSCert,
			InsecureSkipVerify: true,
		}
		if opts.TLSVerify {
			tlsopts.CaCertFile = opts.TLSCACert
			tlsopts.InsecureSkipVerify = false
		}
		if opts.TLSHostname != "" {
			tlsopts.ServerName = opts.TLSHostname
		}
		tlscfg, err := tlsutil.ClientConfig(tlsopts)
		if err != nil {
			return nil, "", err
		}
		options = append(options, k8shelm.WithTLS(tlscfg))
	}

	return k8shelm.NewClient(options...), host, nil
}

func ClientSetup(logger log.Logger, kubeClient *kubernetes.Clientset, tillerOpts TillerOptions) *k8shelm.Client {
	var helmClient *k8shelm.Client
	var host string
	var err error
	for {
		helmClient, host, err = newClient(kubeClient, tillerOpts)
		if err != nil {
			logger.Log("error", fmt.Sprintf("error creating helm client: %s", err.Error()))
			time.Sleep(20 * time.Second)
			continue
		}
		version, err := GetTillerVersion(helmClient, host)
		if err != nil {
			logger.Log("warning", "unable to connect to Tiller", "err", err, "host", host, "options", fmt.Sprintf("%+v", tillerOpts))
			time.Sleep(20 * time.Second)
			continue
		}
		logger.Log("info", "connected to Tiller", "version", version, "host", host, "options", fmt.Sprintf("%+v", tillerOpts))
		break
	}
	return helmClient
}

// GetTillerVersion retrieves tiller version
func GetTillerVersion(cl *k8shelm.Client, h string) (string, error) {
	var v *rls.GetVersionResponse
	var err error
	voption := k8shelm.VersionOption(k8shelm.Host(h))
	if v, err = cl.GetVersion(voption); err != nil {
		return "", fmt.Errorf("error getting tiller version: %v", err)
	}

	return v.GetVersion().String(), nil
}

// TODO ... set up based on the tiller existing in the cluster, if no ops given
func tillerHost(kubeClient *kubernetes.Clientset, opts TillerOptions) (string, error) {
	if opts.Host == "" || opts.Port == "" {
		ts, err := kubeClient.CoreV1().Services(opts.Namespace).Get("tiller-deploy", metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s.%s:%v", ts.Name, ts.Namespace, ts.Spec.Ports[0].Port), nil
	}

	return fmt.Sprintf("%s:%s", opts.Host, opts.Port), nil
}
