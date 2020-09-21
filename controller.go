package main

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	k8sClient *kubernetes.Clientset

	configStore *ConfigStore
	epStore     *EpStore
}

func NewController(
	k8sClient *kubernetes.Clientset,
	configName string,
) *Controller {
	c := &Controller{
		k8sClient:   k8sClient,
		configStore: NewConfigStore(k8sClient, configName),
	}

	if err := c.configStore.InitFromK8s(); err != nil {
		panic(err)
	}

	c.epStore = NewEpStore(k8sClient, v1.NamespaceDefault, c.configStore)
	if err := c.epStore.Init(); err != nil {
		panic(err)
	}
	return c
}

func (c *Controller) Run() {
	go c.configStore.Run()
	go c.epStore.Run()
}

func (c *Controller) GetEndpoints(cluster string) (*Endpoints, bool) {
	return c.epStore.Get(cluster)
}

func (c *Controller) GetConfigSnapshot() *Config {
	return c.configStore.GetConfigSnapshot()
}
