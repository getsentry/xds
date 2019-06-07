package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/hashicorp/consul/command/flags"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/go-homedir"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

// K8SConfig returns a *restclient.Config for initializing a K8S client.
// This configuration first attempts to load a local kubeconfig if a
// path is given. If that doesn't work, then in-cluster auth is used.
func K8SConfig(path string) (*rest.Config, error) {
	// Get the configuration. This can come from multiple sources. We first
	// try kubeconfig it is set directly, then we fall back to in-cluster
	// auth. Finally, we try the default kubeconfig path.
	kubeconfig := path
	if kubeconfig == "" {
		// If kubeconfig is empty, let's first try the default directory.
		// This is must faster than trying in-cluster auth so we try this
		// first.
		dir, err := homedir.Dir()
		if err != nil {
			return nil, fmt.Errorf("error retrieving home directory: %s", err)
		}
		kubeconfig = filepath.Join(dir, ".kube", "config")
	}

	// First try to get the configuration from the kubeconfig value
	config, configErr := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if configErr != nil {
		configErr = fmt.Errorf("error loading kubeconfig: %s", configErr)

		// kubeconfig failed, fall back and try in-cluster config. We do
		// this as the fallback since this makes network connections and
		// is much slower to fail.
		var err error
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, multierror.Append(configErr, fmt.Errorf(
				"error loading in-cluster config: %s", err))
		}
	}

	return config, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var kubeconfig string

	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.StringVar(&kubeconfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	consulFlags := &flags.HTTPFlags{}
	flags.Merge(flagset, consulFlags.ClientFlags())
	flagset.Parse(os.Args[1:])

	consulClient, err := consulFlags.APIClient()
	if err != nil {
		log.Println(err)
		klog.Fatal(err)
	}

	k8sConfig, err := K8SConfig(kubeconfig)
	if err != nil {
		log.Println(err)
		klog.Fatal(err)
	}

	k8sClient, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		log.Println(err)
		klog.Fatal(err)
	}

	c := NewController(consulClient, k8sClient)
	c.Run()

	log.Println("ready.")

	listen := os.Getenv("XDS_LISTEN")
	if listen == "" {
		listen = "127.0.0.1:5000"
	}
	http.ListenAndServe(listen, &xDSHandler{c})
}
