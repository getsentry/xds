package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
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
	mode             = flag.String("mode", "server", "what mode to run xds in (server / proxy)")
	upstreamProxy    = flag.String("upstream-proxy", "", "upstream proxy (if running in proxy mode)")
	configName       = flag.String("config-name", "", "configmap name to use for xds configuration (if running in server mode)")
	bootstrapDataDir = flag.String("bootstrap-data", "", "bootstrap data directory (if running in proxy mode)")
	serviceNode      = flag.String("service-node", "", "service node name")
	serviceCluster   = flag.String("service-cluster", "", "service cluster name")
	concurrency      = flag.Int("concurrency", 1, "envoy concurrency")
	listen           = flag.String("listen", "", "listen address for web service")
	validate         = flag.String("validate", "", "Path to config map to validate. `-` reads from stdin.")
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

	if *configName == "" {
		*configName = os.Getenv("XDS_CONFIGMAP")
		if *configName == "" {
			log.Fatalf("Must pass -config-name argument or XDS_CONFIGMAP environment variable")
		}
	}

	// synchronously fetches initial state and sets things up
	c := NewController(client, *configName)
	c.Run()
	serveHTTP(&xDSHandler{c})
}

func runProxyMode() {
	bootstrapData, err := readBootstrapData(path.Join(*bootstrapDataDir, "bootstrap.json"))
	if err != nil {
		panic(err)
	}

	proxy, err := newProxy(*upstreamProxy, bootstrapData)
	if err != nil {
		panic(err)
	}

	err = runEnvoy(*serviceNode, *serviceCluster, *upstreamProxy, path.Join(*bootstrapDataDir, "envoy.yaml"), *concurrency)
	if err != nil {
		panic(err)
	}

	serveHTTP(proxy)
}

func runBootstrapMode() {
	values := url.Values{
		"cluster": []string{*serviceCluster},
		"id":      []string{*serviceNode},
	}

	bootstrapURL := fmt.Sprintf("http://%s/bootstrap?%s", *upstreamProxy, values.Encode())
	log.Printf("Downloading bootstrap file from %s", bootstrapURL)

	req, err := http.NewRequest("GET", bootstrapURL, nil)
	if err != nil {
		panic(err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	bootstrapFilePath := path.Join(*bootstrapDataDir, "bootstrap.json")
	out, err := os.Create(bootstrapFilePath)
	if err != nil {
		panic(err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		panic(err)
	}
	log.Printf("Finished downloading bootstrap file to %s", bootstrapFilePath)
}

func serveHTTP(handler http.Handler) {
	log.Println("ready.")

	if *listen == "" {
		*listen = os.Getenv("XDS_LISTEN")
		if *listen == "" {
			log.Fatalf("Must pass -listen argument or XDS_LISTEN environment variable")
		}
	}

	http.ListenAndServe(*listen, handler)
}

func validateConfig(configPath string) {
	log.Printf("Validating: %s\n", configPath)
	cmRaw, err := ReadFileorStdin(configPath)
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

	if *mode == "proxy" || *mode == "bootstrap" {
		if *upstreamProxy == "" {
			log.Fatalf("Must pass 'upstream-proxy'")
		}
		if *bootstrapDataDir == "" {
			log.Fatalf("Must pass 'bootstrap-data-dir'")
		}
		if *serviceNode == "" {
			log.Fatalf("Must pass 'service-node'")
		}
		if *serviceCluster == "" {
			log.Fatalf("Must pass 'service-cluster'")
		}
	}

	switch *mode {
	case "server":
		runServerMode()
	case "proxy":
		runProxyMode()
	case "bootstrap":
		runBootstrapMode()
	default:
		log.Fatalf("Invalid XDS mode (only 'server' and 'proxy' are supported): %s", *mode)
	}

}
