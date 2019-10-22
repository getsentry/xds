package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/rate_limit/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/redis_proxy/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/go-homedir"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

const XDS_CONFIGMAP_ENV = "XDS_CONFIGMAP"
const XDS_LISTEN_ENV = "XDS_LISTEN"
const XDS_BOOTSTRAP_FILE = "XDS_BOOTSTRAP_FILE"

// K8SConfig returns a *restclient.Config for initializing a K8S client.
// This configuration first attempts to load a local kubeconfig if a
// path is given. If that doesn't work, then in-cluster auth is used.
func K8SConfig() (*rest.Config, error) {
	dir, err := homedir.Dir()
	if err != nil {
		return nil, fmt.Errorf("error retrieving home directory: %s", err)
	}
	kubeconfig := filepath.Join(dir, ".kube", "config")

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

	config, err := K8SConfig()
	if err != nil {
		log.Println(err)
		klog.Fatal(err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println(err)
		klog.Fatal(err)
	}

	// synchronously fetches initial state and sets things up
	c := NewController(client)
	c.Run()

	log.Println("ready.")

	listen := os.Getenv(XDS_LISTEN_ENV)
	if listen == "" {
		listen = "127.0.0.1:5000"
	}
	http.ListenAndServe(listen, &xDSHandler{c})
}
