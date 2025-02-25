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

package fetchers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"cloud.google.com/go/asset/apiv1/assetpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/elastic/cloudbeat/resources/fetching"
	"github.com/elastic/cloudbeat/resources/fetching/cycle"
	"github.com/elastic/cloudbeat/resources/providers/gcplib"
	"github.com/elastic/cloudbeat/resources/providers/gcplib/inventory"
	"github.com/elastic/cloudbeat/resources/utils/testhelper"
)

type GcpPoliciesFetcherTestSuite struct {
	suite.Suite

	resourceCh chan fetching.ResourceInfo
}

func TestGcpPoliciesFetcherTestSuite(t *testing.T) {
	s := new(GcpPoliciesFetcherTestSuite)

	suite.Run(t, s)
}

func (s *GcpPoliciesFetcherTestSuite) SetupTest() {
	s.resourceCh = make(chan fetching.ResourceInfo, 50)
}

func (s *GcpPoliciesFetcherTestSuite) TearDownTest() {
	close(s.resourceCh)
}

func (s *GcpPoliciesFetcherTestSuite) TestFetcher_Fetch_Success() {
	ctx := context.Background()
	mockInventoryService := &inventory.MockServiceAPI{}
	fetcher := GcpPoliciesFetcher{
		log:        testhelper.NewLogger(s.T()),
		resourceCh: s.resourceCh,
		provider:   mockInventoryService,
	}

	mockInventoryService.On("ListProjectsAncestorsPolicies", mock.Anything).Return(
		[]*inventory.ProjectPoliciesAsset{
			{
				Ecs: &fetching.EcsGcp{
					Provider:         "gcp",
					ProjectId:        "a",
					ProjectName:      "a",
					OrganizationId:   "a",
					OrganizationName: "a",
				},
				Policies: []*inventory.ExtendedGcpAsset{
					{Asset: &assetpb.Asset{}},
				},
			},
		}, nil,
	)

	err := fetcher.Fetch(ctx, cycle.Metadata{})
	s.Require().NoError(err)
	results := testhelper.CollectResources(s.resourceCh)

	s.Len(results, 1)
}

func (s *GcpPoliciesFetcherTestSuite) TestFetcher_Fetch_Error() {
	ctx := context.Background()
	mockInventoryService := &inventory.MockServiceAPI{}
	fetcher := GcpPoliciesFetcher{
		log:        testhelper.NewLogger(s.T()),
		resourceCh: s.resourceCh,
		provider:   mockInventoryService,
	}

	mockInventoryService.On("ListProjectsAncestorsPolicies", mock.Anything).Return(nil, errors.New("api call error"))

	err := fetcher.Fetch(ctx, cycle.Metadata{})
	s.Require().Error(err)
}

func TestGcpPoliciesAsset_GetMetadata(t *testing.T) {
	tests := []struct {
		name     string
		resource GcpPoliciesAsset
		want     fetching.ResourceMetadata
		wantErr  bool
	}{
		{
			name: "retrieve successfully project  policies asset",
			resource: GcpPoliciesAsset{
				Type:    fetching.ProjectManagement,
				subType: fetching.GcpPolicies,
				Asset: &inventory.ProjectPoliciesAsset{
					Ecs: &fetching.EcsGcp{
						ProjectId:        projectId,
						ProjectName:      "a",
						OrganizationId:   "a",
						OrganizationName: "a",
					},
					Policies: []*inventory.ExtendedGcpAsset{},
				},
			},
			want: fetching.ResourceMetadata{
				ID:      fmt.Sprintf("%s-%s", fetching.GcpPolicies, projectId),
				Name:    fmt.Sprintf("%s-%s", fetching.GcpPolicies, projectId),
				Type:    fetching.ProjectManagement,
				SubType: fetching.GcpPolicies,
				Region:  gcplib.GlobalRegion,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.resource.GetMetadata()

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGcpPolicies_GetElasticCommonData(t *testing.T) {
	type fields struct {
		Type    string
		subType string
		Asset   *inventory.ProjectPoliciesAsset
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string]any
	}{
		{
			name: "verify elastic common data",
			fields: fields{
				Type:    fetching.ProjectManagement,
				subType: fetching.GcpPolicies,
				Asset: &inventory.ProjectPoliciesAsset{
					Ecs: &fetching.EcsGcp{
						ProjectId:        projectId,
						ProjectName:      "a",
						OrganizationId:   "a",
						OrganizationName: "a",
					},
					Policies: []*inventory.ExtendedGcpAsset{},
				},
			},
			want: map[string]any{
				"cloud": map[string]any{
					"provider": "gcp",
					"account": map[string]any{
						"id":   projectId,
						"name": "a",
					},
					"Organization": map[string]any{
						"id":   "a",
						"name": "a",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GcpPoliciesAsset{
				Type:    tt.fields.Type,
				subType: tt.fields.subType,
				Asset:   tt.fields.Asset,
			}

			got, err := g.GetElasticCommonData()

			require.NoError(t, err)
			assert.Equalf(t, tt.want, got, "GetElasticCommonData()")
		})
	}
}
