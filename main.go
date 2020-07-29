package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/http_connection_manager/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/rate_limit/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/redis_proxy/v2"
	_ "github.com/envoyproxy/go-control-plane/envoy/config/filter/network/tcp_proxy/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/go-homedir"
	"sigs.k8s.io/yaml"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	mode              = flag.String("mode", "server", "what mode to run xds in (server / proxy)")
	upstreamProxyURL  = flag.String("upstream-proxy-url", "", "upstream proxy url (if running in proxy mode)")
	configName        = flag.String("config-name", "default/xds", "configmap name to use for xds configuration (if running in server mode)")
	bootstrapDataFile = flag.String("bootstrap-data", "", "bootstrap data file (if running in proxy mode)")
	serviceNode       = flag.String("service-node", "", "service node name")
	serviceCluster    = flag.String("service-cluster", "", "service cluster name")
	concurrency       = flag.Int("concurrency", 1, "envoy concurrency")
	envoyConfigPath   = flag.String("envoy-config-path", "/envoy.yaml", "path to envoy config")
	listen            = flag.String("listen", "0.0.0.0:5000", "listen address for web service")
	validate = flag.String("validate", "", "Path to config map to validate. `-` reads from stdin.")
)

// ReadFileorStdin returns content of file or stdin.
func ReadFileorStdin(filePath string) ([]byte, error) {
	var fileHandler *os.File

	if filePath == "-" {
		fileHandler = os.Stdin
	} else {
		var err error
		fileHandler, err = os.Open(filePath)
		defer fileHandler.Close()
		if err != nil {
			return nil, err
		}
	}

	buf := bytes.NewBuffer(nil)
	io.Copy(buf, fileHandler)

	return buf.Bytes(), nil
}

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

func runServerMode() {
	config, err := K8SConfig()
	if err != nil {
		log.Println(err)
		klog.Fatal(err)
	}

	flag.Parse()

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Println(err)
		klog.Fatal(err)
	}

	// synchronously fetches initial state and sets things up
	c := NewController(client, *configName)
	c.Run()
	serveHTTP(&xDSHandler{c})
}

func runProxyMode() {
	bootstrapData, err := readBootstrapData(*bootstrapDataFile)
	if err != nil {
		panic(err)
	}

	proxy, err := newProxy(*upstreamProxyURL, bootstrapData)
	if err != nil {
		panic(err)
	}

	err = runEnvoy(*serviceNode, *serviceCluster, *envoyConfigPath, *concurrency)
	if err != nil {
		panic(err)
	}

	serveHTTP(proxy)
}

func serveHTTP(handler http.Handler) {
	log.Println("ready.")
	http.ListenAndServe(*listen, handler)
}

func validateConfig(configPath string) {
	log.Printf("Validating: %s\n", *validatePtr)
	cmRaw, err := ReadFileorStdin(*validatePtr)
	if err != nil {
		log.Fatal(err)
	}

	var cm v1.ConfigMap

	err = yaml.UnmarshalStrict(cmRaw, &cm)
	if err != nil {
		log.Fatal(err)
	}

	config := NewConfig()
	if err := config.Load(&cm); err != nil {
		log.Fatal(err)
	}
	log.Println("Configuration is valid.")
}

func main() {
	flag.Parse()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	if *validate != "" {
		validateConfig(*validate)
		return
	}

	switch *mode {
	case "server":
		runServerMode()
	case "proxy":
		if *upstreamProxyURL == "" {
			log.Fatalf("Must pass 'upstream-proxy-url' when running in proxy mode")
		}
		if *bootstrapDataFile == "" {
			log.Fatalf("Must pass 'bootstrap-data' when running in proxy mode")
		}
		if *serviceNode == "" {
			log.Fatalf("Must pass 'service-node' while running in proxy mode")
		}
		if *serviceCluster == "" {
			log.Fatalf("Must pass 'service-cluster' when running in proxy mode")
		}
		runProxyMode()
	default:
		log.Fatalf("Invalid XDS mode (only 'server' and 'proxy' are supported): %s", *mode)
	}

}
