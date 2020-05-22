package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateHandler405(t *testing.T) {
	req, err := http.NewRequest("GET", "/validate", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc((&xDSHandler{}).handleValidate)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatal("GET method should respond with 405 not allowed.")
	}
}

func TestValidateHandler400(t *testing.T) {
	req, err := http.NewRequest(
		"POST",
		"/validate",
		strings.NewReader("invalid yaml"),
	)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc((&xDSHandler{}).handleValidate)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatal("Invalid yaml should respond with 400.")
	}
}

func TestValidateHandler400InvalidConfigmap(t *testing.T) {
	req, err := http.NewRequest(
		"POST",
		"/validate",
		strings.NewReader("data: {\"listeners\": 1}"),
	)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc((&xDSHandler{}).handleValidate)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatal("Invalid configmap should respond with 400.")
	}
}

func TestValidateHandler200(t *testing.T) {
	req, err := http.NewRequest(
		"POST",
		"/validate",
		strings.NewReader(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: xds
data:
  listeners: |
    - name: foo
      address:
        socket_address:
          address: 0.0.0.0
          port_value: 10001
  clusters: |
    - name: foo
      type: EDS
      connect_timeout: 0.25s
      eds_cluster_config:
        service_name: default/foo
        eds_config:
          api_config_source:
            api_type: REST
            cluster_names: [xds_cluster]
            refresh_delay: 1s
  assignemtns: |
    by-cluster:
      foo:
        listeners:
          - foo
        clusters:
          - foo
    by-node-id:
      foo:
        listeners:
          - foo
        clusters:
          - foo
`),
	)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc((&xDSHandler{}).handleValidate)
	handler.ServeHTTP(rr, req)

	// TODO(michal): Stricter validation for missing data.{listeners,clusters,assignemtns}
	// 				 resp. define schema for the data field of configmap
	if rr.Code != http.StatusOK {
		t.Fatal("Even an empty body is valid configmap.")
	}
}
