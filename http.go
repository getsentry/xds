package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	v2 "github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/gogo/protobuf/jsonpb"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

type xDSHandler struct {
	controller *Controller
}

func (h *xDSHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/v2/discovery:endpoints":
		h.handleEDS(w, req)
	case "/v2/discovery:listeners":
		h.handleLDS(w, req)
	case "/v2/discovery:clusters":
		h.handleCDS(w, req)
	case "/config":
		h.handleConfig(w, req)
	case "/validate":
		h.handleValidate(w, req)
	case "/healthz":
		http.Error(w, "ok", 200)
	default:
		http.Error(w, "not found", 404)
	}
}

func (h *xDSHandler) handleValidate(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	var cm v1.ConfigMap

	if body, err := ioutil.ReadAll(req.Body); err != nil {
		http.Error(w, err.Error(), 400)
		return
	} else {
		if err := yaml.UnmarshalStrict(body, &cm); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

	}

	config := NewConfig()
	if err := config.Load(&cm); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	http.Error(w, "ok", 200)
}

// Endpoint Discovery Service
func (h *xDSHandler) handleEDS(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	dr, err := readDiscoveryRequest(req)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), 500)
		return
	}

	if len(dr.ResourceNames) != 1 {
		http.Error(w, "must have 1 resource_names", 400)
		return
	}

	if ep, ok := h.controller.epStore.Get(dr.ResourceNames[0]); ok {
		if ep.version == dr.VersionInfo {
			w.WriteHeader(304)
			return
		}
		w.Write(ep.data)
	} else {
		http.Error(w, "not found", 404)
	}
}

// Listener Discovery Service
func (h *xDSHandler) handleLDS(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	dr, err := readDiscoveryRequest(req)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), 500)
		return
	}

	c := h.controller.configStore.GetConfigSnapshot()
	if c.version == dr.VersionInfo {
		w.WriteHeader(304)
		return
	}

	if b, ok := c.GetListeners(dr.Node); ok {
		w.Write(b)
	} else {
		http.Error(w, "not found", 404)
	}
}

// Cluster Discovery Service
func (h *xDSHandler) handleCDS(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	dr, err := readDiscoveryRequest(req)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), 500)
		return
	}

	c := h.controller.configStore.GetConfigSnapshot()
	if c.version == dr.VersionInfo {
		w.WriteHeader(304)
		return
	}

	if b, ok := c.GetClusters(dr.Node); ok {
		w.Write(b)
	} else {
		http.Error(w, "not found", 404)
	}
}

func (h *xDSHandler) handleConfig(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}

	status := 200
	lastError := ""
	lastUpdate := h.controller.configStore.lastUpdate

	if h.controller.configStore.lastError != nil {
		status = 500
		lastError = h.controller.configStore.lastError.Error()
	}

	j, _ := json.Marshal(struct {
		Version    string    `json:"version"`
		LastError  string    `json:"last_error"`
		LastUpdate time.Time `json:"last_update"`
	}{
		h.controller.configStore.config.version,
		lastError,
		lastUpdate,
	})

	w.WriteHeader(status)
	w.Write(j)
}

func readDiscoveryRequest(req *http.Request) (*v2.DiscoveryRequest, error) {
	var dr v2.DiscoveryRequest
	err := (&jsonpb.Unmarshaler{
		AllowUnknownFields: true,
	}).Unmarshal(req.Body, &dr)
	return &dr, err
}
