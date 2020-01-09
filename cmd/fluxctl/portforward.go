package main

import (
	"fmt"
	"strings"

	portforward "github.com/justinbarrick/go-k8s-portforward"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Attempt to create PortForwards to fluxes that match the label selectors until a Flux
// is found or an error is returned.
func tryPortforwards(context string, ns string, selectors ...metav1.LabelSelector) (p *portforward.PortForward, err error) {
	message := fmt.Sprintf("No pod found in namespace %q using the following selectors:", ns)

	for _, selector := range selectors {
		p, err = tryPortforward(context, ns, selector)
		if err == nil {
			return
		}

		if !strings.Contains(err.Error(), "Could not find running pod for selector") {
			return
		} else {
			message = fmt.Sprintf("%s\n  %s", message, metav1.FormatLabelSelector(&selector))
		}
	}
	message = fmt.Sprintf("%s\n\nMake sure Flux is running in namespace %q.\n"+
		"If Flux is running in another different namespace, please supply it to --k8s-fwd-ns.", message, ns)
	if err != nil {
		err = errors.New(message)
	}

	return
}

// Attempt to create a portforward in the namespace for the provided LabelSelector
func tryPortforward(kubeConfigContext string, ns string, selector metav1.LabelSelector) (*portforward.PortForward, error) {
	portforwarder := &portforward.PortForward{
		Namespace:       ns,
		Labels:          selector,
		DestinationPort: 3030,
	}

	var configOverrides clientcmd.ConfigOverrides
	if kubeConfigContext != "" {
		configOverrides.CurrentContext = kubeConfigContext
	}

	var err error
	portforwarder.Config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&configOverrides,
	).ClientConfig()

	if err != nil {
		return portforwarder, errors.Wrap(err, "Could not load kubernetes configuration file")
	}

	portforwarder.Clientset, err = kubernetes.NewForConfig(portforwarder.Config)
	if err != nil {
		return portforwarder, errors.Wrap(err, "Could not create kubernetes client")
	}

	err = portforwarder.Start()
	if err != nil {
		return portforwarder, err
	}

	return portforwarder, nil
}
