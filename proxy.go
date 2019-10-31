package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type proxy struct {
	bootstrapData *bootstrapData
	reverseProxy  *httputil.ReverseProxy
	readClusters  bool
	readListeners bool
	readEndpoints map[string]struct{}
}

func newProxy(upstreamURLRaw string, bootstrapData *bootstrapData) (*proxy, error) {
	upstreamURL, err := url.Parse(upstreamURLRaw)
	if err != nil {
		return nil, err
	}

	return &proxy{
		bootstrapData: bootstrapData,
		reverseProxy:  httputil.NewSingleHostReverseProxy(upstreamURL),
		readEndpoints: make(map[string]struct{}),
	}, nil
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.URL.Path {
	case "/v2/discovery:endpoints":
		p.handleEDS(w, req)
	case "/v2/discovery:listeners":
		p.handleLDS(w, req)
	case "/v2/discovery:clusters":
		p.handleCDS(w, req)
	case "/healthz":
		if len(p.readEndpoints) == len(p.bootstrapData.Endpoints) {
			http.Error(w, "ok", 200)
		} else {
			http.Error(w, "bootstrapping", 500)
		}
	default:
		http.Error(w, "not found", 404)
	}
}

func (p *proxy) handleEDS(w http.ResponseWriter, req *http.Request) {
	if len(p.readEndpoints) == len(p.bootstrapData.Endpoints) {
		p.reverseProxy.ServeHTTP(w, req)
		return
	}

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

	log.Printf("serving request for '%s' endpoints from bootstrap data", dr.ResourceNames[0])
	p.readEndpoints[dr.ResourceNames[0]] = struct{}{}
	w.Write(p.bootstrapData.Endpoints[dr.ResourceNames[0]])
}

func (p *proxy) handleLDS(w http.ResponseWriter, req *http.Request) {
	if p.readListeners {
		p.reverseProxy.ServeHTTP(w, req)
		return
	}

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

	log.Printf("serving listeners request from bootstrap data")
	w.Write(p.bootstrapData.Listeners)
}

func (p *proxy) handleCDS(w http.ResponseWriter, req *http.Request) {
	if p.readClusters {
		p.reverseProxy.ServeHTTP(w, req)
		return
	}

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

	log.Printf("serving clusters request from boostrap data")
	w.Write(p.bootstrapData.Clusters)
}
