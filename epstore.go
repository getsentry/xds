package main

import (
	"log"
	"reflect"
	"sync"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/endpoint"
	"github.com/gogo/protobuf/types"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type EpStore struct {
	k8sClient *kubernetes.Clientset
	namespace string

	informer cache.SharedIndexInformer
	store    cache.Store

	configStore *ConfigStore

	registry sync.Map
}

type Endpoints struct {
	version string
	data    []byte
}

func NewEpStore(
	k8sClient *kubernetes.Clientset,
	namespace string,
	configStore *ConfigStore,
) *EpStore {
	es := &EpStore{
		k8sClient:   k8sClient,
		namespace:   namespace,
		configStore: configStore,
	}
	infFactory := informers.NewSharedInformerFactoryWithOptions(k8sClient, 0,
		informers.WithNamespace(namespace),
		informers.WithTweakListOptions(func(*metav1.ListOptions) {}))

	es.informer = infFactory.Core().V1().Endpoints().Informer()
	es.store = es.informer.GetStore()
	es.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			config := es.configStore.GetConfigSnapshot()

			key, _ := cache.MetaNamespaceKeyFunc(obj)
			if !config.HasService(key) {
				return
			}

			es.LoadEp(obj.(*v1.Endpoints))
		},
		UpdateFunc: func(old, cur interface{}) {
			config := es.configStore.GetConfigSnapshot()

			key, _ := cache.MetaNamespaceKeyFunc(cur)
			if !config.HasService(key) {
				return
			}
			oep := old.(*v1.Endpoints)
			cep := cur.(*v1.Endpoints)

			if reflect.DeepEqual(cep.Subsets, oep.Subsets) {
				return
			}

			es.LoadEp(cep)
		},
		DeleteFunc: func(obj interface{}) {
			config := es.configStore.GetConfigSnapshot()

			key, _ := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if !config.HasService(key) {
				return
			}

			es.DeleteEp(key)
		},
	})
	return es
}

func (es *EpStore) Init() error {
	config := es.configStore.GetConfigSnapshot()
	eps, err := es.k8sClient.CoreV1().Endpoints(es.namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, ep := range eps.Items {
		if !config.HasService(es.namespace + "/" + ep.GetName()) {
			continue
		}
		es.store.Add(&ep)
		es.LoadEp(&ep)
	}
	return nil
}

func (es *EpStore) Run() {
	es.informer.Run(nil)
}

func validSubset(subset v1.EndpointSubset) bool {
	return len(subset.Ports) == 1
}

func (es *EpStore) LoadEp(ep *v1.Endpoints) {
	epKey := es.namespace + "/" + ep.GetName()
	version := ep.ObjectMeta.ResourceVersion

	// Check if the existing resource version is the same
	if ep, ok := es.registry.Load(epKey); ok && ep.(*Endpoints).version == version {
		return
	}

	// Count how many registrations we need here
	// so that we can correctly allocate what we need
	n := 0
	for _, subset := range ep.Subsets {
		if !validSubset(subset) {
			continue
		}
		n += len(subset.Addresses)
	}

	cla := &v2.ClusterLoadAssignment{
		ClusterName: epKey,
		Endpoints: []*endpoint.LocalityLbEndpoints{{
			LbEndpoints: make([]*endpoint.LbEndpoint, n),
		}},
	}

	n = 0
	for _, subset := range ep.Subsets {
		if !validSubset(subset) {
			continue
		}
		for _, address := range subset.Addresses {

			cla.Endpoints[0].LbEndpoints[n] = &endpoint.LbEndpoint{
				HostIdentifier: &endpoint.LbEndpoint_Endpoint{
					Endpoint: &endpoint.Endpoint{
						Address: &core.Address{
							Address: &core.Address_SocketAddress{
								SocketAddress: &core.SocketAddress{
									Protocol: core.TCP,
									Address:  address.IP,
									PortSpecifier: &core.SocketAddress_PortValue{
										PortValue: uint32(subset.Ports[0].Port),
									},
								},
							},
						},
					},
				},
			}
			log.Printf("%s/%s:%d\n", ep.GetName(), address.IP, subset.Ports[0].Port)
			n++
		}
	}

	r, _ := types.MarshalAny(cla)
	j, _ := structToJSON(&v2.DiscoveryResponse{
		VersionInfo: version,
		Resources:   []*types.Any{r},
	})

	// Write entire DiscoveryResponse into the registry
	es.registry.Store(epKey, &Endpoints{
		version: version,
		data:    j,
	})
}

func (es *EpStore) DeleteEp(key string) {
	log.Println("removing service: " + key)
	es.registry.Delete(key)
}

func (es *EpStore) Get(key string) (*Endpoints, bool) {
	// HACK: Strip an old k8s prefix
	if key[:4] == "k8s:" {
		key = key[4:]
	}
	if ep, ok := es.registry.Load(key); ok {
		return ep.(*Endpoints), true
	}
	return nil, false
}
