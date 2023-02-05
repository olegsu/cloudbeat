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

package elb

import (
	"fmt"
	"regexp"

	"github.com/elastic/cloudbeat/config"
	"github.com/elastic/cloudbeat/resources/fetching"
	"github.com/elastic/cloudbeat/resources/providers/awslib"
	"github.com/elastic/elastic-agent-libs/logp"
	"k8s.io/client-go/kubernetes"
)

type Option func(e *ElbFetcher)

func WithLogger(log *logp.Logger) Option {
	return func(e *ElbFetcher) {
		e.log = log
	}
}

func WithFetcherConfig(c *config.Config) Option {
	return func(e *ElbFetcher) {
		cfg := ElbFetcherConfig{}
		if err := config.UnpackInto(c, fetching.ElbType, &cfg); err != nil {
			panic(err)
		}
		e.cfg = cfg
	}
}

func WithResourceChannel(ch chan fetching.ResourceInfo) Option {
	return func(e *ElbFetcher) {
		e.resourceCh = ch
	}
}

func WtihELBProvider(p awslib.ElbLoadBalancerDescriber) Option {
	return func(e *ElbFetcher) {
		e.elbProvider = p
	}
}

func WithKubeClient(k kubernetes.Interface) Option {
	return func(e *ElbFetcher) {
		e.kubeClient = k
	}
}

func WithRegexMatcher(region string) Option {
	return func(e *ElbFetcher) {
		f := fmt.Sprintf(elbRegexTemplate, region)
		e.lbRegexMatchers = append(e.lbRegexMatchers, regexp.MustCompile((f)))
	}
}

func WithCloudIdentity(i *awslib.Identity) Option {
	return func(e *ElbFetcher) {
		e.cloudIdentity = i
	}
}
