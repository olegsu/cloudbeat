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

package monitoring

import (
	"github.com/elastic/cloudbeat/resources/providers/awslib"
	"github.com/elastic/cloudbeat/resources/providers/awslib/cloudtrail"
	"github.com/elastic/cloudbeat/resources/providers/awslib/cloudwatch"
	"github.com/elastic/cloudbeat/resources/providers/awslib/cloudwatch/logs"
	"github.com/elastic/cloudbeat/resources/providers/awslib/securityhub"
	"github.com/elastic/cloudbeat/resources/providers/awslib/sns"
)

type FactoryOption func(e *MonitoringFactory)

func WithIdentityProvider(p *awslib.IdentityProvider) FactoryOption {
	return func(e *MonitoringFactory) {
		e.IdentityProvider = p
	}
}

func WithAwsConfigProvider(p awslib.ConfigProviderAPI) FactoryOption {
	return func(e *MonitoringFactory) {
		e.AwsConfigProvider = p
	}
}

func WithCrossRegionTrailFactory(f *awslib.MultiRegionClientFactory[cloudtrail.Client]) FactoryOption {
	return func(e *MonitoringFactory) {
		e.TrailCrossRegionFactory = f
	}
}

func WithCrossRegionCloudwatchFactory(f *awslib.MultiRegionClientFactory[cloudwatch.Client]) FactoryOption {
	return func(e *MonitoringFactory) {
		e.CloudwatchCrossRegionFactory = f
	}
}

func WithCrossRegionCloudwatchlogsFactory(f *awslib.MultiRegionClientFactory[logs.Client]) FactoryOption {
	return func(e *MonitoringFactory) {
		e.CloudwatchlogsCrossRegionFactory = f
	}
}

func WithCrossRegionSNSFactory(f *awslib.MultiRegionClientFactory[sns.Client]) FactoryOption {
	return func(e *MonitoringFactory) {
		e.SNSCrossRegionFactory = f
	}
}

func WithCrossRegionSecurityhubFacotry(f *awslib.MultiRegionClientFactory[securityhub.Service]) FactoryOption {
	return func(e *MonitoringFactory) {
		e.SecurityhubRegionFactory = f
	}
}
