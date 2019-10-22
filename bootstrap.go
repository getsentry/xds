package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type bootstrapData struct {
	Clusters  []byte            `json:"clusters"`
	Listeners []byte            `json:"listeners"`
	Endpoints map[string][]byte `json:"endpoints"`
}

func readBootstrapData(rawData []byte) (*bootstrapData, error) {
	var bsData bootstrapData

	err := json.Unmarshal(rawData, &bsData)
	if err != nil {
		return nil, err
	}

	return &bsData, nil
}

func (b *bootstrapData) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/v2/discovery:endpoints":
		b.handleEDS(w, req)
	case "/v2/discovery:listeners":
		b.handleLDS(w, req)
	case "/v2/discovery:clusters":
		b.handleCDS(w, req)
	case "/healthz":
		http.Error(w, "ok", 200)
	default:
		http.Error(w, "not found", 404)
	}
}

func (b *bootstrapData) handleEDS(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	dr, err := readDiscoveryRequest(req)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if len(dr.ResourceNames) != 1 {
		http.Error(w, "must have 1 resource_names", 400)
		return
	}

	// TODO: error checking
	w.Write(b.Endpoints[dr.ResourceNames[0]])
}

func (b *bootstrapData) handleLDS(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	_, err := readDiscoveryRequest(req)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Write(b.Listeners)
}

// Cluster Discovery Service
func (b *bootstrapData) handleCDS(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	_, err := readDiscoveryRequest(req)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), 500)
		return
	}

	w.Write(b.Clusters)
}
