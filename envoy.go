package main

import (
	"log"
	"os"
	"os/exec"

	"github.com/google/shlex"
)

func runEnvoy(args string) error {
	argParts, err := shlex.Split(args)
	if err != nil {
		return err
	}

	envoyCommand := exec.Command("envoy", argParts...)
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
