package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
)

// /etc/envoy/envoy.yaml
const ENVOY_BOOTSTRAP_CONFIG = `
static_resources:
  clusters:
  - name: xds_cluster
    type: LOGICAL_DNS
    connect_timeout: 0.5s
    load_assignment:
      cluster_name: xds_cluster
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: %s
                port_value: 80

dynamic_resources:
  lds_config:
    api_config_source:
      api_type: REST
      cluster_names: [xds_cluster]
      refresh_delay: 3600s
      request_timeout: 1s

  cds_config:
    api_config_source:
      api_type: REST
      cluster_names: [xds_cluster]
      refresh_delay: 3600s
      request_timeout: 1s
`

func runEnvoy(serviceNode, serviceCluster, endpoint, envoyConfigPath string, concurrency int) error {
	f, err := os.Create(envoyConfigPath)
	if err != nil {
		return err
	}
	f.WriteString(buildEnvoyBootstrapConfig(endpoint))
	f.Close()

	envoyCommand := exec.Command(
		"envoy",
		"--service-node", serviceNode,
		"--service-cluster", serviceCluster,
		"--concurrency", strconv.Itoa(concurrency),
		"-c", envoyConfigPath,
	)

	envoyCommand.Stdout = os.Stdout
	envoyCommand.Stderr = os.Stderr

	err = envoyCommand.Start()
	if err != nil {
		return err
	}

	go func() {
		envoyCommand.Wait()
		log.Printf("Envoy subprocess exited, closing parent process")
		os.Exit(-1)
	}()

	return nil
}

func buildEnvoyBootstrapConfig(endpoint string) string {
	return fmt.Sprintf(ENVOY_BOOTSTRAP_CONFIG, endpoint)
}
