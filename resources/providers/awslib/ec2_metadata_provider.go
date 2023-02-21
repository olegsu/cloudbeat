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

	ec2imds "github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

type Ec2Metadata = ec2imds.InstanceIdentityDocument

type Ec2MetadataProvider struct {
	IMDS *ec2imds.Client
}

type MetadataProvider interface {
	GetMetadata(ctx context.Context) (Ec2Metadata, error)
}

func (provider Ec2MetadataProvider) GetMetadata(ctx context.Context) (Ec2Metadata, error) {
	input := &ec2imds.GetInstanceIdentityDocumentInput{}
	if provider.IMDS == nil {
		panic("provider.svc is nil") // TODO: for validation only, should not be merge like this
	}
	identityDocument, err := provider.IMDS.GetInstanceIdentityDocument(ctx, input)
	if err != nil {
		return ec2imds.GetInstanceIdentityDocumentOutput{}.InstanceIdentityDocument, err
	}

	return identityDocument.InstanceIdentityDocument, nil
}
