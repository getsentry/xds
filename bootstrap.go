package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"sync"
)

type bootstrapData struct {
	Clusters      []byte            `json:"clusters"`
	Listeners     []byte            `json:"listeners"`
	Endpoints     map[string][]byte `json:"endpoints"`
	endpointsLock sync.Mutex
}

func readBootstrapData(path string) (*bootstrapData, error) {
	bootstrapFd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer bootstrapFd.Close()

	bootstrapRawData, err := ioutil.ReadAll(bootstrapFd)
	if err != nil {
		return nil, err
	}

	var bsData bootstrapData
	err = json.Unmarshal(bootstrapRawData, &bsData)
	if err != nil {
		return nil, err
	}

	return &bsData, nil
}
