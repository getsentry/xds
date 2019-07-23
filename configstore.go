package main

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/types"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"
)

const (
	ByNodeIdKeyPrefix  = "n:"
	ByClusterKeyPrefix = "c:"
)

type Config struct {
	version   string
	listeners map[string]*v2.Listener
	clusters  map[string]*v2.Cluster
	rules     *AssignmentRules
	// Set type
	services map[string]struct{}
}

func (c *Config) HasService(name string) bool {
	_, ok := c.services[name]
	return ok
}

func (c *Config) getAssignmentCache(node *core.Node) (*assignmentCache, bool) {
	a, ok := c.rules.cache[ByNodeIdKeyPrefix+node.Id]
	if !ok {
		a, ok = c.rules.cache[ByClusterKeyPrefix+node.Cluster]
	}
	return a, ok
}

func (c *Config) GetListeners(node *core.Node) ([]byte, bool) {
	if cache, ok := c.getAssignmentCache(node); ok {
		return cache.listeners, true
	}
	return nil, false
}

func (c *Config) GetClusters(node *core.Node) ([]byte, bool) {
	if cache, ok := c.getAssignmentCache(node); ok {
		return cache.clusters, true
	}
	return nil, false
}

type Assignment struct {
	Listeners []string `json:"listeners"`
	Clusters  []string `json:"clusters"`
}

type AssignmentRules struct {
	ByNodeId  map[string]*Assignment `json:"by-node-id"`
	ByCluster map[string]*Assignment `json:"by-cluster"`

	cache map[string]*assignmentCache
}

type assignmentCache struct {
	listeners []byte
	clusters  []byte
}

type ConfigStore struct {
	namespace  string
	configName string
	k8sClient  *kubernetes.Clientset

	informer cache.SharedIndexInformer
	store    cache.Store

	config    *Config
	configMap *v1.ConfigMap

	lastUpdate time.Time
	lastError  error
}

func (cs *ConfigStore) Init() error {
	namespace, name := k8sSplitName(cs.configName)
	log.Println("ConfigStore.Init: ", namespace, name, cs.configName)
	cm, err := cs.k8sClient.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		log.Println("ConfigStore.Init wups ", err)
		return err
	}
	cs.store.Add(cm)
	return cs.Load(cm)
}

func (cs *ConfigStore) Run() {
	cs.informer.Run(nil)
}

func (cs *ConfigStore) Load(cm *v1.ConfigMap) error {
	defer func() {
		cs.lastUpdate = time.Now()
	}()
	listeners, err := extractListeners(cm)
	if err != nil {
		return err
	}
	clusters, err := extractClusters(cm)
	if err != nil {
		return err
	}
	config := &Config{
		version:   cm.ObjectMeta.ResourceVersion,
		listeners: make(map[string]*v2.Listener),
		clusters:  make(map[string]*v2.Cluster),
		services:  make(map[string]struct{}),
	}
	for _, listener := range listeners {
		config.listeners[listener.Name] = listener
	}
	for _, cluster := range clusters {
		serviceName := cluster.EdsClusterConfig.ServiceName
		if serviceName[:4] == "k8s:" {
			serviceName = serviceName[4:]
		}
		config.services[serviceName] = struct{}{}
		config.clusters[cluster.Name] = cluster
	}
	assignments, err := extractAssignments(cm)
	if err != nil {
		return err
	}
	config.rules = assignments
	if err := validateConfig(config); err != nil {
		return err
	}
	cs.configMap = cm
	cs.config = config
	return nil
}

func (cs *ConfigStore) GetConfigSnapshot() *Config {
	return cs.config
}

func NewConfigStore(
	k8sClient *kubernetes.Clientset,
	configName string,
) *ConfigStore {
	if configName == "" {
		configName = "default/xds"
	}
	cs := &ConfigStore{
		configName: configName,
		k8sClient:  k8sClient,
	}

	namespace, _ := k8sSplitName(configName)

	infFactory := informers.NewSharedInformerFactoryWithOptions(k8sClient, 0,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(*metav1.ListOptions) {}))

	cs.informer = infFactory.Core().V1().ConfigMaps().Informer()
	cs.store = cs.informer.GetStore()
	cs.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, cur interface{}) {
			key, _ := cache.MetaNamespaceKeyFunc(cur)
			if key != configName {
				return
			}
			if reflect.DeepEqual(old, cur) {
				return
			}

			cs.lastError = cs.Load(cur.(*v1.ConfigMap))
			if cs.lastError != nil {
				log.Println("update failed: ", cs.lastError)
			} else {
				log.Println("update applied")
			}
		},
	})
	return cs
}

func extractListeners(cm *v1.ConfigMap) ([]*v2.Listener, error) {
	// We have to decode our input, which is YAML, so we can iterate
	// over each of them.
	raw, err := unmarshalYAMLSlice([]byte(cm.Data["listeners"]))
	if err != nil {
		return nil, errors.New("listeners: invalid YAML: " + err.Error())
	}
	rv := make([]*v2.Listener, len(raw))
	for i, r := range raw {
		var pb v2.Listener
		if err := convertToPb(r, &pb); err != nil {
			d, _ := yaml.Marshal(r)
			return nil, errors.New(fmt.Sprintf("listeners: index %d: %s:\n\n%s", i, err, d))
		}
		rv[i] = &pb
	}
	return rv, nil
}

func extractClusters(cm *v1.ConfigMap) ([]*v2.Cluster, error) {
	// We have to decode our input, which is YAML, so we can iterate
	// over each of them.
	raw, err := unmarshalYAMLSlice([]byte(cm.Data["clusters"]))
	if err != nil {
		return nil, errors.New("clusters: invalid YAML: " + err.Error())
	}
	rv := make([]*v2.Cluster, len(raw))
	for i, r := range raw {
		var pb v2.Cluster
		if err := convertToPb(r, &pb); err != nil {
			d, _ := yaml.Marshal(r)
			return nil, errors.New(fmt.Sprintf("clusters: index %d: %s:\n\n%s", i, err, d))
		}
		rv[i] = &pb
	}
	return rv, nil
}

func extractAssignments(cm *v1.ConfigMap) (*AssignmentRules, error) {
	var ar AssignmentRules
	err := yaml.Unmarshal([]byte(cm.Data["assignments"]), &ar)
	return &ar, err
}

func validateConfig(config *Config) error {
	config.rules.cache = make(map[string]*assignmentCache)

	for key, assignment := range config.rules.ByNodeId {
		lr := make([]types.Any, len(assignment.Listeners))
		cache := &assignmentCache{}
		for i, name := range assignment.Listeners {
			if listener, ok := config.listeners[name]; !ok {
				return errors.New("missing listener: " + name)
			} else {
				r, _ := types.MarshalAny(listener)
				lr[i] = *r
			}
		}
		cache.listeners, _ = structToJSON(&v2.DiscoveryResponse{
			VersionInfo: config.version,
			Resources:   lr,
		})

		cr := make([]types.Any, len(assignment.Clusters))
		for i, name := range assignment.Clusters {
			if cluster, ok := config.clusters[name]; !ok {
				return errors.New("unknown cluster: " + name)
			} else {
				r, _ := types.MarshalAny(cluster)
				cr[i] = *r
			}
		}
		cache.clusters, _ = structToJSON(&v2.DiscoveryResponse{
			VersionInfo: config.version,
			Resources:   cr,
		})

		config.rules.cache[ByNodeIdKeyPrefix+key] = cache
	}

	for key, assignment := range config.rules.ByCluster {
		lr := make([]types.Any, len(assignment.Listeners))
		cache := &assignmentCache{}
		for i, name := range assignment.Listeners {
			if listener, ok := config.listeners[name]; !ok {
				return errors.New("missing listener: " + name)
			} else {
				r, _ := types.MarshalAny(listener)
				lr[i] = *r
			}
		}
		cache.listeners, _ = structToJSON(&v2.DiscoveryResponse{
			VersionInfo: config.version,
			Resources:   lr,
		})

		cr := make([]types.Any, len(assignment.Clusters))
		for i, name := range assignment.Clusters {
			if cluster, ok := config.clusters[name]; !ok {
				return errors.New("unknown cluster: " + name)
			} else {
				r, _ := types.MarshalAny(cluster)
				cr[i] = *r
			}
		}
		cache.clusters, _ = structToJSON(&v2.DiscoveryResponse{
			VersionInfo: config.version,
			Resources:   cr,
		})

		config.rules.cache[ByClusterKeyPrefix+key] = cache
	}
	return nil
}
