package k8sgarden

import (
	"code.cloudfoundry.org/executor/depot/containerstore"
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager/v3"
)

type factory struct {
	client garden.Client
}

var _ containerstore.GardenClientFactory = &factory{}

func NewFactory(client garden.Client) containerstore.GardenClientFactory {
	return &factory{
		client: client,
	}
}

func (f *factory) NewGardenClient(logger lager.Logger, traceID string) garden.Client {
	return f.client
}
