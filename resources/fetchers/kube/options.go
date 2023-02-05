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
	"github.com/elastic/cloudbeat/resources/fetching"
	"github.com/elastic/elastic-agent-autodiscover/kubernetes"
	"github.com/elastic/elastic-agent-libs/logp"
	k8s "k8s.io/client-go/kubernetes"
)

type Option func(e *KubeFetcher)

func WithLogger(log *logp.Logger) Option {
	return func(e *KubeFetcher) {
		e.log = log
	}
}

func WithResourceChannel(ch chan fetching.ResourceInfo) Option {
	return func(e *KubeFetcher) {
		e.resourceCh = ch
	}
}

func WithWatchers(w []kubernetes.Watcher) Option {
	return func(e *KubeFetcher) {
		e.watchers = w
	}
}

func WtihClientProvider(c k8s.Interface) Option {
	return func(e *KubeFetcher) {
		e.client = c
	}
}
