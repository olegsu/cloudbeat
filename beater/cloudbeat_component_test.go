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

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/elastic/beats/v7/libbeat/beat"
	"github.com/elastic/beats/v7/libbeat/processors"
	beater_testing "github.com/elastic/cloudbeat/beater/testing"
	"github.com/elastic/cloudbeat/config"
	"github.com/elastic/cloudbeat/dataprovider"
	"github.com/elastic/cloudbeat/evaluator"
	"github.com/elastic/cloudbeat/resources/fetchers"
	"github.com/elastic/cloudbeat/resources/fetchers/monitoring"
	"github.com/elastic/cloudbeat/resources/fetchersManager"
	"github.com/elastic/cloudbeat/resources/fetching"
	awscis_monitoring "github.com/elastic/cloudbeat/resources/providers/aws_cis/monitoring"
	"github.com/elastic/cloudbeat/resources/providers/awslib"
	"github.com/elastic/cloudbeat/resources/providers/awslib/cloudtrail"
	"github.com/elastic/cloudbeat/resources/providers/awslib/cloudwatch"
	"github.com/elastic/cloudbeat/resources/providers/awslib/cloudwatch/logs"
	"github.com/elastic/cloudbeat/resources/providers/awslib/securityhub"
	"github.com/elastic/cloudbeat/resources/providers/awslib/sns"
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
			agentconfig.MustNewConfigFrom(mapstr.M{"name": "aws-monitoring"}),
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

	crossRegionCloudtrailProvider := awslib.MultiRegionClientFactory[cloudtrail.Client]{}
	crossRegionCloudwatchProvider := awslib.MultiRegionClientFactory[cloudwatch.Client]{}
	crossRegionCloudwatchlogsProvider := awslib.MultiRegionClientFactory[logs.Client]{}
	crossRegionSNSProvider := awslib.MultiRegionClientFactory[sns.Client]{}

	awscfg := awssdk.Config{}

	mockSecurityhubProvider := securityhub.MockService{}

	reg, err := initRegistry(log, &cfg, resourceChan, &leader, map[string]fetchers.Fetcher{
		fetching.MonitoringType: monitoring.NewFetcher(
			monitoring.WithLogger(log),
			monitoring.WithResourceChannel(resourceChan),
			monitoring.WithFetcherConfig(&cfg),
			monitoring.WithCloudIdentity(&awslib.Identity{Account: awssdk.String("cloudbeat-testing-aws-account")}),
			monitoring.WithSecurityhubProvider(),
			monitoring.WithMonitoringProvider(&awscis_monitoring.Provider{
				Cloudtrail:     cloudtrail.NewProvider(awscfg, log, &crossRegionCloudtrailProvider),
				Cloudwatch:     cloudwatch.NewProvider(log, awscfg, &crossRegionCloudwatchProvider),
				Cloudwatchlogs: logs.NewCloudwatchLogsProvider(log, awscfg, &crossRegionCloudwatchlogsProvider),
				Sns:            sns.NewSNSProvider(log, awscfg, &crossRegionSNSProvider),
				Log:            log,
			}),
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
