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
	readEndpoints bool
}

func newProxy(upstreamURLRaw string, bootstrapData *bootstrapData) (*proxy, error) {
	log.Printf("Running in proxy mode (upstream is '%s')", upstreamURLRaw)
	upstreamURL, err := url.Parse(upstreamURLRaw)
	if err != nil {
		return nil, err
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	return &proxy{
		bootstrapData: bootstrapData,
		reverseProxy:  reverseProxy,
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
		if p.readEndpoints {
			http.Error(w, "ok", 200)
		} else {
			http.Error(w, "bootstrapping", 500)
		}
	default:
		http.Error(w, "not found", 404)
	}
}

func (p *proxy) handleEDS(w http.ResponseWriter, req *http.Request) {
	if p.readEndpoints {
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

	p.bootstrapData.endpointsLock.Lock()
	defer p.bootstrapData.endpointsLock.Unlock()
	if _, exists := p.bootstrapData.Endpoints[dr.ResourceNames[0]]; exists {
		log.Printf("serving request for '%s' endpoints from bootstrap data", dr.ResourceNames[0])
		w.Write(p.bootstrapData.Endpoints[dr.ResourceNames[0]])
		delete(p.bootstrapData.Endpoints, dr.ResourceNames[0])
		if len(p.bootstrapData.Endpoints) == 0 {
			p.readEndpoints = true
		}
	} else {
		log.Printf("eds failed, endpoints for that cluster already read")
		http.Error(w, "unavailable bootstrap data", 500)
	}
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
