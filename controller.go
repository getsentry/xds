package main

import (
	"io/ioutil"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type Controller struct {
	k8sClient *kubernetes.Clientset

	bootstrapData *bootstrapData
	configStore   *ConfigStore
	epStore       *EpStore
}

func NewController(
	k8sClient *kubernetes.Clientset,
) *Controller {
	c := &Controller{
		k8sClient:   k8sClient,
		configStore: NewConfigStore(k8sClient, os.Getenv(XDS_CONFIGMAP_ENV)),
	}

	bootstrapFile := os.Getenv(XDS_BOOTSTRAP_FILE)
	if bootstrapFile != "" {
		bootstrapFd, err := os.Open(bootstrapFile)
		if err != nil {
			panic(err)
		}

		bootstrapRawData, err := ioutil.ReadAll(bootstrapFd)
		if err != nil {
			panic(err)
		}

		bsData, err := readBootstrapData(bootstrapRawData)
		if err != nil {
			panic(err)
		}

		c.bootstrapData = bsData
	} else {
		if err := c.configStore.InitFromK8s(); err != nil {
			panic(err)
		}

		c.epStore = NewEpStore(k8sClient, v1.NamespaceDefault, c.configStore)
		if err := c.epStore.Init(); err != nil {
			panic(err)
		}
	}
	return c
}

func (c *Controller) Run() {
	if c.bootstrapData == nil {
		go c.configStore.Run()
		go c.epStore.Run()
	}
}

func (c *Controller) GetEndpoints(cluster string) (*Endpoints, bool) {
	return c.epStore.Get(cluster)
}

func (c *Controller) GetConfigSnapshot() *Config {
	return c.configStore.GetConfigSnapshot()
}
