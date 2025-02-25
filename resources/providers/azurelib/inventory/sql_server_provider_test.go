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

package inventory

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

type encryptionProtectorFn func() ([]armsql.EncryptionProtectorsClientListByServerResponse, error)
type auditingPoliciesFn func() (armsql.ServerBlobAuditingPoliciesClientGetResponse, error)
type transparentDataEncryptionFn func(dbName string) ([]armsql.TransparentDataEncryptionsClientListByDatabaseResponse, error)
type databaseFn func() ([]armsql.DatabasesClientListByServerResponse, error)

func mockAssetSQLEncryptionProtector(f encryptionProtectorFn) SQLProviderAPI {
	wrapper := &sqlAzureClientWrapper{
		AssetSQLEncryptionProtector: func(_ context.Context, _, _, _ string, _ *arm.ClientOptions, _ *armsql.EncryptionProtectorsClientListByServerOptions) ([]armsql.EncryptionProtectorsClientListByServerResponse, error) {
			return f()
		},
	}

	return &sqlProvider{
		log:    logp.NewLogger("mock_asset_sql_encryption_protector"),
		client: wrapper,
	}
}

func mockAssetSQLBlobAuditingPolicies(f auditingPoliciesFn) SQLProviderAPI {
	wrapper := &sqlAzureClientWrapper{
		AssetSQLBlobAuditingPolicies: func(ctx context.Context, subID, resourceGroup, sqlServerName string, clientOptions *arm.ClientOptions, options *armsql.ServerBlobAuditingPoliciesClientGetOptions) (armsql.ServerBlobAuditingPoliciesClientGetResponse, error) {
			return f()
		},
	}

	return &sqlProvider{
		log:    logp.NewLogger("mock_asset_sql_encryption_protector"),
		client: wrapper,
	}
}

func mockAssetSQLTransparentDataEncryption(tdesFn transparentDataEncryptionFn, dbsFn databaseFn) SQLProviderAPI {
	wrapper := &sqlAzureClientWrapper{
		AssetSQLDatabases: func(_ context.Context, _, _, _ string, _ *arm.ClientOptions, _ *armsql.DatabasesClientListByServerOptions) ([]armsql.DatabasesClientListByServerResponse, error) {
			return dbsFn()
		},
		AssetSQLTransparentDataEncryptions: func(_ context.Context, _, _, _, dbName string, _ *arm.ClientOptions, _ *armsql.TransparentDataEncryptionsClientListByDatabaseOptions) ([]armsql.TransparentDataEncryptionsClientListByDatabaseResponse, error) {
			return tdesFn(dbName)
		},
	}

	return &sqlProvider{
		log:    logp.NewLogger("mock_asset_sql_encryption_protector"),
		client: wrapper,
	}
}

func TestListSQLEncryptionProtector(t *testing.T) {
	tcs := map[string]struct {
		apiMockCall    func() ([]armsql.EncryptionProtectorsClientListByServerResponse, error)
		expectError    bool
		expectedAssets []AzureAsset
	}{
		"Error on calling api": {
			apiMockCall: func() ([]armsql.EncryptionProtectorsClientListByServerResponse, error) {
				return nil, errors.New("error")
			},
			expectError:    true,
			expectedAssets: nil,
		},
		"No Encryption Protector Response": {
			apiMockCall: func() ([]armsql.EncryptionProtectorsClientListByServerResponse, error) {
				return nil, nil
			},
			expectError:    false,
			expectedAssets: nil,
		},
		"Response with encryption protectors in different pages": {
			apiMockCall: func() ([]armsql.EncryptionProtectorsClientListByServerResponse, error) {
				return wrapEpResponse(
					wrapEpResult(
						epAzure("id1", armsql.ServerKeyTypeAzureKeyVault),
						epAzure("id2", armsql.ServerKeyTypeAzureKeyVault),
					),
					wrapEpResult(
						epAzure("id3", armsql.ServerKeyTypeServiceManaged),
					),
				), nil
			},
			expectError: false,
			expectedAssets: []AzureAsset{
				epAsset("id1", "AzureKeyVault"),
				epAsset("id2", "AzureKeyVault"),
				epAsset("id3", "ServiceManaged"),
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			p := mockAssetSQLEncryptionProtector(tc.apiMockCall)
			got, err := p.ListSQLEncryptionProtector(context.Background(), "subId", "resourceGroup", "sqlServerInstanceName")

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.ElementsMatch(t, tc.expectedAssets, got)
		})
	}
}

func TestGetSQLBlobAuditingPolicies(t *testing.T) {
	tcs := map[string]struct {
		apiMockCall    func() (armsql.ServerBlobAuditingPoliciesClientGetResponse, error)
		expectError    bool
		expectedAssets []AzureAsset
	}{
		"Error on calling api": {
			apiMockCall: func() (armsql.ServerBlobAuditingPoliciesClientGetResponse, error) {
				return armsql.ServerBlobAuditingPoliciesClientGetResponse{}, errors.New("error")
			},
			expectError:    true,
			expectedAssets: nil,
		},
		"Response with blob auditing policy": {
			apiMockCall: func() (armsql.ServerBlobAuditingPoliciesClientGetResponse, error) {
				return armsql.ServerBlobAuditingPoliciesClientGetResponse{
					ServerBlobAuditingPolicy: armsql.ServerBlobAuditingPolicy{
						ID:   to.Ptr("id1"),
						Name: to.Ptr("policy"),
						Type: to.Ptr("audit-policy"),
						Properties: &armsql.ServerBlobAuditingPolicyProperties{
							State:                        to.Ptr(armsql.BlobAuditingPolicyStateEnabled),
							IsAzureMonitorTargetEnabled:  to.Ptr(true),
							IsDevopsAuditEnabled:         to.Ptr(false),
							IsManagedIdentityInUse:       to.Ptr(true),
							IsStorageSecondaryKeyInUse:   to.Ptr(true),
							QueueDelayMs:                 to.Ptr(int32(100)),
							RetentionDays:                to.Ptr(int32(90)),
							StorageAccountAccessKey:      to.Ptr("access-key"),
							StorageAccountSubscriptionID: to.Ptr("sub-id"),
							StorageEndpoint:              nil,
							AuditActionsAndGroups:        []*string{to.Ptr("a"), to.Ptr("b")},
						},
					},
				}, nil
			},
			expectError: false,
			expectedAssets: []AzureAsset{
				{
					Id:             "id1",
					Name:           "policy",
					DisplayName:    "",
					Location:       "global",
					ResourceGroup:  "resourceGroup",
					SubscriptionId: "subId",
					Type:           "audit-policy",
					TenantId:       "",
					Sku:            nil,
					Identity:       nil,
					Properties: map[string]any{
						"state":                        "Enabled",
						"isAzureMonitorTargetEnabled":  true,
						"isDevopsAuditEnabled":         false,
						"isManagedIdentityInUse":       true,
						"isStorageSecondaryKeyInUse":   true,
						"queueDelayMs":                 int32(100),
						"retentionDays":                int32(90),
						"storageAccountAccessKey":      "access-key",
						"storageAccountSubscriptionID": "sub-id",
						"storageEndpoint":              "",
						"auditActionsAndGroups":        []string{"a", "b"},
					},
					Extension: nil,
				},
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			p := mockAssetSQLBlobAuditingPolicies(tc.apiMockCall)
			got, err := p.GetSQLBlobAuditingPolicies(context.Background(), "subId", "resourceGroup", "sqlServerInstanceName")

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.ElementsMatch(t, tc.expectedAssets, got)
		})
	}
}

func TestListSqlTransparentDataEncryptions(t *testing.T) {
	tcs := map[string]struct {
		tdeFn          transparentDataEncryptionFn
		dbFn           databaseFn
		expectError    bool
		expectedAssets []AzureAsset
	}{
		"Error on fetching databases": {
			dbFn: func() ([]armsql.DatabasesClientListByServerResponse, error) {
				return nil, errors.New("error")
			},
			expectError:    true,
			expectedAssets: nil,
		},
		"Error on fetching 1 transparent data encryption (out of 3)": {
			dbFn: func() ([]armsql.DatabasesClientListByServerResponse, error) {
				return wrapDBResponse(
					[]string{"db1"},
					[]string{"db2", "db3"},
				), nil
			},
			tdeFn: func(dbName string) ([]armsql.TransparentDataEncryptionsClientListByDatabaseResponse, error) {
				if dbName == "db2" {
					return nil, errors.New("error")
				}
				return wrapTdeResponse(
					wrapTdeResult(tdeAzure(dbName+"-tde1", armsql.TransparentDataEncryptionStateEnabled)),
				), nil
			},
			expectError: true,
			expectedAssets: []AzureAsset{
				tdeAsset("db1-tde1", "db1", "Enabled"),
				tdeAsset("db3-tde1", "db3", "Enabled"),
			},
		},
		"Response of 3 dbs with multiple tdes (in different pages)": {
			dbFn: func() ([]armsql.DatabasesClientListByServerResponse, error) {
				return wrapDBResponse(
					[]string{"db1"},
					[]string{"db2", "db3"},
				), nil
			},
			tdeFn: func(dbName string) ([]armsql.TransparentDataEncryptionsClientListByDatabaseResponse, error) {
				if dbName == "db1" {
					return wrapTdeResponse(
						wrapTdeResult(tdeAzure(dbName+"-tde1", armsql.TransparentDataEncryptionStateEnabled)),
						wrapTdeResult(tdeAzure(dbName+"-tde2", armsql.TransparentDataEncryptionStateDisabled)),
					), nil
				}
				return wrapTdeResponse(
					wrapTdeResult(tdeAzure(dbName+"-tde1", armsql.TransparentDataEncryptionStateEnabled)),
				), nil
			},
			expectError: false,
			expectedAssets: []AzureAsset{
				tdeAsset("db1-tde1", "db1", "Enabled"),
				tdeAsset("db1-tde2", "db1", "Disabled"),
				tdeAsset("db2-tde1", "db2", "Enabled"),
				tdeAsset("db3-tde1", "db3", "Enabled"),
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			p := mockAssetSQLTransparentDataEncryption(tc.tdeFn, tc.dbFn)
			got, err := p.ListSQLTransparentDataEncryptions(context.Background(), "subId", "resourceGroup", "sqlServerInstanceName")

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.ElementsMatch(t, tc.expectedAssets, got)
		})
	}
}

func wrapEpResult(eps ...*armsql.EncryptionProtector) armsql.EncryptionProtectorListResult {
	return armsql.EncryptionProtectorListResult{
		Value: eps,
	}
}

func wrapEpResponse(results ...armsql.EncryptionProtectorListResult) []armsql.EncryptionProtectorsClientListByServerResponse {
	return lo.Map(results, func(r armsql.EncryptionProtectorListResult, index int) armsql.EncryptionProtectorsClientListByServerResponse {
		return armsql.EncryptionProtectorsClientListByServerResponse{
			EncryptionProtectorListResult: r,
		}
	})
}

func epAzure(id string, keyType armsql.ServerKeyType) *armsql.EncryptionProtector {
	return &armsql.EncryptionProtector{
		ID:       to.Ptr(id),
		Name:     to.Ptr("Name " + id),
		Kind:     to.Ptr("azurekeyvault"),
		Location: to.Ptr("eu-west"),
		Type:     to.Ptr("encryptionProtector"),
		Properties: &armsql.EncryptionProtectorProperties{
			ServerKeyType:       to.Ptr(keyType),
			AutoRotationEnabled: to.Ptr(true),
			ServerKeyName:       to.Ptr("key-" + id),
			Subregion:           to.Ptr("eu-west-1"),
		},
	}
}

func epAsset(id, keyType string) AzureAsset {
	return AzureAsset{
		Id:             id,
		Name:           "Name " + id,
		DisplayName:    "",
		Location:       "eu-west",
		ResourceGroup:  "resourceGroup",
		SubscriptionId: "subId",
		Type:           "encryptionProtector",
		TenantId:       "",
		Sku:            nil,
		Identity:       nil,
		Properties: map[string]any{
			"kind":                "azurekeyvault",
			"serverKeyType":       keyType,
			"autoRotationEnabled": true,
			"serverKeyName":       "key-" + id,
			"subregion":           "eu-west-1",
			"thumbprint":          "",
			"uri":                 "",
		},
		Extension: nil,
	}
}

func wrapDBResponse(ids ...[]string) []armsql.DatabasesClientListByServerResponse {
	return lo.Map(ids, func(ids []string, _ int) armsql.DatabasesClientListByServerResponse {
		values := lo.Map(ids, func(id string, _ int) *armsql.Database {
			return &armsql.Database{
				Name: to.Ptr(id),
			}
		})

		return armsql.DatabasesClientListByServerResponse{
			DatabaseListResult: armsql.DatabaseListResult{
				Value: values,
			},
		}
	})
}

func wrapTdeResponse(results ...armsql.LogicalDatabaseTransparentDataEncryptionListResult) []armsql.TransparentDataEncryptionsClientListByDatabaseResponse {
	return lo.Map(results, func(r armsql.LogicalDatabaseTransparentDataEncryptionListResult, _ int) armsql.TransparentDataEncryptionsClientListByDatabaseResponse {
		return armsql.TransparentDataEncryptionsClientListByDatabaseResponse{
			LogicalDatabaseTransparentDataEncryptionListResult: r,
		}
	})
}

func wrapTdeResult(tdes ...*armsql.LogicalDatabaseTransparentDataEncryption) armsql.LogicalDatabaseTransparentDataEncryptionListResult {
	return armsql.LogicalDatabaseTransparentDataEncryptionListResult{
		Value: tdes,
	}
}

func tdeAzure(id string, state armsql.TransparentDataEncryptionState) *armsql.LogicalDatabaseTransparentDataEncryption {
	return &armsql.LogicalDatabaseTransparentDataEncryption{
		ID:   to.Ptr(id),
		Name: to.Ptr("name-" + id),
		Type: to.Ptr("transparentDataEncryption"),
		Properties: &armsql.TransparentDataEncryptionProperties{
			State: to.Ptr(state),
		},
	}
}

func tdeAsset(id, dbName, state string) AzureAsset {
	return AzureAsset{
		Id:             id,
		Name:           "name-" + id,
		DisplayName:    "",
		Location:       "global",
		ResourceGroup:  "resourceGroup",
		SubscriptionId: "subId",
		Type:           "transparentDataEncryption",
		TenantId:       "",
		Sku:            nil,
		Identity:       nil,
		Properties: map[string]any{
			"databaseName": dbName,
			"state":        state,
		},
		Extension: nil,
	}
}
