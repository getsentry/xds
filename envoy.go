package main

import (
	"log"
	"os"
	"os/exec"
)

func runEnvoy(serviceNode, serviceCluster, envoyConfigPath string, concurrency int) error {
	envoyCommand := exec.Command(
		"envoy",
		"--service-node", serviceNode,
		"--service-cluster", serviceCluster,
		"--concurrency", string(concurrency),
		"-c", envoyConfigPath,
	)

	envoyCommand.Stdout = os.Stdout
	envoyCommand.Stderr = os.Stderr

	err := envoyCommand.Start()
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
