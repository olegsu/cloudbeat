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

package eks

import (
	"github.com/elastic/cloudbeat/config"
	"github.com/elastic/cloudbeat/resources/fetching"
	"github.com/elastic/cloudbeat/resources/providers/awslib"
	"github.com/elastic/elastic-agent-libs/logp"
)

type Option func(e *EksFetcher)

func WithLogger(log *logp.Logger) Option {
	return func(e *EksFetcher) {
		e.log = log
	}
}

func WithFetcherConfig(c *config.Config) Option {
	return func(e *EksFetcher) {
		cfg := EksFetcherConfig{}
		if err := config.UnpackInto(c, fetching.EksType, &cfg); err != nil {
			panic(err)
		}
		e.cfg = cfg
	}
}

func WithResourceChannel(ch chan fetching.ResourceInfo) Option {
	return func(e *EksFetcher) {
		e.resourceCh = ch
	}
}

func WithEKSProvider(p awslib.EksClusterDescriber) Option {
	return func(e *EksFetcher) {
		e.eksProvider = p
	}
}
