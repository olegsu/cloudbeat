// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package kube

import (
	"sync"

	"github.com/elastic/cloudbeat/resources/fetching"
	"github.com/elastic/cloudbeat/resources/providers"
	"github.com/elastic/elastic-agent-autodiscover/kubernetes"
	"github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	k8s "k8s.io/client-go/kubernetes"
)

type KubeFactory struct {
	Provider *providers.KubernetesProvider
}

type KubeClientProvider func(kubeconfig string, opt kubernetes.KubeClientOptions) (k8s.Interface, error)

func New(options ...FactoryOption) *KubeFactory {
	e := &KubeFactory{}
	for _, opt := range options {
		opt(e)
	}
	return e
}

func (f *KubeFactory) Create(log *logp.Logger, c *config.C, ch chan fetching.ResourceInfo) (fetching.Fetcher, error) {
	log.Debug("Starting KubeFactory.Create")

	cfg := KubeApiFetcherConfig{}
	err := c.Unpack(&cfg)
	if err != nil {
		return nil, err
	}
	// TODO: kube provider should come from options
	return f.CreateFrom(log, cfg, ch, kubernetes.GetKubernetesClient)
}

func (f *KubeFactory) CreateFrom(log *logp.Logger, cfg KubeApiFetcherConfig, ch chan fetching.ResourceInfo, provider KubeClientProvider) (fetching.Fetcher, error) {
	fe := &KubeFetcher{
		log:            log,
		cfg:            cfg,
		clientProvider: provider,
		watcherLock:    &sync.Once{},
		watchers:       make([]kubernetes.Watcher, 0),
		resourceCh:     ch,
	}

	log.Infof("Kube Fetcher created with the following config: Name: %s, Interval: %s, "+
		"Kubeconfig: %s", cfg.Name, cfg.Interval, cfg.KubeConfig)
	return fe, nil
}
