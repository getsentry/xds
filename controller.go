package main

import (
	"os"

	consul "github.com/hashicorp/consul/api"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	consulClient *consul.Client
	k8sClient    *kubernetes.Clientset

	configStore *ConfigStore
	epStore     *EpStore
}

func NewController(
	consulClient *consul.Client,
	k8sClient *kubernetes.Clientset,
) *Controller {
	namespace := v1.NamespaceDefault

	c := &Controller{
		consulClient: consulClient,
		k8sClient:    k8sClient,
	}

	c.configStore = NewConfigStore(k8sClient, os.Getenv("XDS_CONFIGMAP"))
	if err := c.configStore.Init(); err != nil {
		panic(err)
	}

	c.epStore = NewEpStore(k8sClient, namespace, c.configStore)
	if err := c.epStore.Init(); err != nil {
		panic(err)
	}
	return c
}

func (c *Controller) Run() {
	go c.configStore.Run()
	go c.epStore.Run()
}
