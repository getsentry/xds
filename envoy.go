package main

import (
	"io"
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
	envoyStdout, err := envoyCommand.StdoutPipe()
	if err != nil {
		return err
	}
	go passthroughOutput(envoyStdout, os.Stdout)

	envoyStderr, err := envoyCommand.StderrPipe()
	if err != nil {
		return err
	}
	go passthroughOutput(envoyStderr, os.Stderr)

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

func passthroughOutput(source io.ReadCloser, destination *os.File) {
	buffer := make([]byte, 4096)

	for {
		_, err := source.Read(buffer)
		if err != nil {
			return
		}

		_, err = destination.Write(buffer)
		if err != nil {
			return
		}
	}
}
