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

package beater

import (
	"context"
	"testing"
	"time"

	awssdk_s3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/processors"
	beater_testing "github.com/elastic/cloudbeat/beater/testing"
	"github.com/elastic/cloudbeat/config"
	"github.com/elastic/cloudbeat/dataprovider"
	"github.com/elastic/cloudbeat/evaluator"
	"github.com/elastic/cloudbeat/resources/fetchers"
	"github.com/elastic/cloudbeat/resources/fetchers/s3"
	"github.com/elastic/cloudbeat/resources/fetchersManager"
	"github.com/elastic/cloudbeat/resources/fetching"
	"github.com/elastic/cloudbeat/resources/providers/awslib"
	awslib_s3 "github.com/elastic/cloudbeat/resources/providers/awslib/s3"
	"github.com/elastic/cloudbeat/transformer"
	"github.com/elastic/cloudbeat/uniqueness"
	agentconfig "github.com/elastic/elastic-agent-libs/config"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_Cloudbeat(t *testing.T) {
	cfg := config.Config{
		BundlePath: "/Users/olegsuharevich/workspace/elastic/cloudbeat/bundle.tar.gz",
		Benchmark:  config.CIS_AWS,
		Fetchers: []*agentconfig.C{
			agentconfig.MustNewConfigFrom(mapstr.M{"name": "aws-s3"}),
		},
		Processors: processors.PluginConfig{},
	}
	cloudbeat, err := newTestingCloudbeat(cfg)
	assert.NoError(t, err)
	go func() {
		time.Sleep(time.Second * 3)
		cloudbeat.Stop()
	}()
	err = cloudbeat.Run(newTestingBeat())
	assert.NoError(t, err)
}

func newTestingCloudbeat(cfg config.Config) (*cloudbeat, error) {
	ctx, cancel := context.WithCancel(context.Background())
	log := logp.NewLogger("test")
	resourceChan := make(chan fetching.ResourceInfo)
	leader := uniqueness.MockManager{}
	leader.EXPECT().Run(mock.Anything).Return(nil)

	s3Mock := &awslib_s3.MockClient{}

	s3Mock.EXPECT().ListBuckets(mock.Anything, &awssdk_s3.ListBucketsInput{}).Return(nil, nil)

	reg, err := initRegistry(log, &cfg, resourceChan, &leader, map[string]fetchers.Fetcher{
		fetching.S3Type: s3.NewFetcher(
			s3.WithLogger(log),
			s3.WithFetcherConfig(&cfg),
			s3.WithResourceChannel(resourceChan),
			s3.WithS3Client(awslib_s3.NewProvider(log, map[string]awslib_s3.Client{
				awslib.DefaultRegion: s3Mock,
			})),
		),
	})
	if err != nil {
		cancel()
		return nil, err
	}
	data, err := fetchersManager.NewData(log, time.Second, time.Second*5, reg)
	if err != nil {
		cancel()
		return nil, err
	}

	eval, err := evaluator.NewOpaEvaluator(ctx, log, &cfg)
	if err != nil {
		cancel()
		return nil, err
	}

	commonDataProvider := dataprovider.NewCommonDataProvider(log, &cfg)
	commonData, err := commonDataProvider.FetchCommonData(ctx)
	if err != nil {
		cancel()
		return nil, err
	}

	t := transformer.NewTransformer(log, commonData, "test-index")
	// TODO: cloudbeat should have own cancel function to support defer here
	return &cloudbeat{
		ctx:         ctx,
		cancel:      cancel,
		config:      &cfg,
		log:         log,
		leader:      &leader,
		data:        data,
		evaluator:   eval,
		transformer: t,
		resourceCh:  resourceChan,
	}, nil
}

func newTestingBeat() *beat.Beat {
	p := &beater_testing.MockPipeline{}
	c := &beater_testing.MockClient{}
	p.EXPECT().Connect().Return(c, nil)
	c.EXPECT().PublishAll(mock.Anything)
	return &beat.Beat{
		Publisher: p,
	}
}
