package main

import (
	"os"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	k8sClient *kubernetes.Clientset

	configStore *ConfigStore
	epStore     *EpStore
}

func NewController(
	k8sClient *kubernetes.Clientset,
) *Controller {
	c := &Controller{
		k8sClient: k8sClient,
	}

	c.configStore = NewConfigStore(k8sClient, os.Getenv(XDS_CONFIGMAP_ENV))
	if err := c.configStore.Init(); err != nil {
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
