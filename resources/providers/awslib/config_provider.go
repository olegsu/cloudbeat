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

package awslib

import (
	"context"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/elastic/beats/v7/x-pack/libbeat/common/aws"
	logp "github.com/elastic/elastic-agent-libs/logp"
)

type ConfigProvider struct {
	MetadataProvider MetadataProvider
}

type ConfigProviderAPI interface {
	InitializeAWSConfig(ctx context.Context, cfg aws.ConfigAWS) (*awssdk.Config, error)
}

func InitializeAWSConfig(ctx context.Context, cfg aws.ConfigAWS) (awssdk.Config, error) {
	return aws.InitializeAWSConfig(cfg)
}

func GetCurrentRegion(ctx context.Context, log *logp.Logger, meta MetadataProvider) string {
	doc, err := meta.GetMetadata(ctx)
	if err != nil {
		log.Warnf("failed to request metadata, setting %s as regions, err: %v", DefaultRegion, err)
		return DefaultRegion
	}
	return doc.Region
}
