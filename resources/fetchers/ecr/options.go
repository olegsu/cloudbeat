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

package ecr

import (
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/elastic/cloudbeat/config"
	"github.com/elastic/cloudbeat/resources/fetching"
	"github.com/elastic/cloudbeat/resources/providers/awslib"
	"github.com/elastic/elastic-agent-libs/logp"
	"k8s.io/client-go/kubernetes"
)

const (
	// PrivateRepoRegexTemplate should identify images with an ecr regex template
	// <account-id>.dkr.ecr.<region>.amazonaws.com/<repository-name>
	PrivateRepoRegexTemplate = "^%s\\.dkr\\.ecr\\.([-\\w]+)\\.amazonaws\\.com\\/([-\\w\\.\\/]+)[:,@]?"
)

type Option func(e *EcrFetcher)

func WithLogger(log *logp.Logger) Option {
	return func(e *EcrFetcher) {
		e.log = log
	}
}

func WithFetcherConfig(c *config.Config) Option {
	return func(e *EcrFetcher) {
		cfg := EcrFetcherConfig{}
		if err := config.UnpackInto(c, fetching.EcrType, &cfg); err != nil {
			panic(err)
		}
		e.cfg = cfg
	}
}

func WithKubeClient(k kubernetes.Interface) Option {
	return func(e *EcrFetcher) {
		e.kubeClient = k
	}
}

func WithPodDescriber(awsAccountID string, ecrRepositoryDescriber awslib.EcrRepositoryDescriber) Option {
	return func(e *EcrFetcher) {
		privateRepoRegex := fmt.Sprintf(PrivateRepoRegexTemplate, awsAccountID)

		pd := PodDescriber{
			FilterRegex: regexp.MustCompile(privateRepoRegex),
			Provider:    ecrRepositoryDescriber,
		}
		e.PodDescriber = pd
	}
}

func WithResourceChannel(ch chan fetching.ResourceInfo) Option {
	return func(e *EcrFetcher) {
		e.resourceCh = ch
	}
}

func WithAWSConfig(cfg aws.Config) Option {
	return func(e *EcrFetcher) {
		e.awsConfig = cfg
	}
}
