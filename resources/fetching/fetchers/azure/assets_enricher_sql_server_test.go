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
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/elastic/cloudbeat/resources/fetching/cycle"
	"github.com/elastic/cloudbeat/resources/providers/azurelib"
	"github.com/elastic/cloudbeat/resources/providers/azurelib/inventory"
)

type enricherResponse struct {
	assets []inventory.AzureAsset
	err    error
}

func noRes() enricherResponse {
	return enricherResponse{
		assets: nil,
		err:    nil,
	}
}

func assetRes(a ...inventory.AzureAsset) enricherResponse {
	return enricherResponse{
		assets: a,
		err:    nil,
	}
}

func errorRes(err error) enricherResponse {
	return enricherResponse{
		assets: nil,
		err:    err,
	}
}

func TestSQLServerEnricher_Enrich(t *testing.T) {
	tcs := map[string]struct {
		input       []inventory.AzureAsset
		expected    []inventory.AzureAsset
		expectError bool
		epRes       map[string]enricherResponse
		bapRes      map[string]enricherResponse
		tdeRes      map[string]enricherResponse
	}{
		"Some assets have encryption protection, others don't": {
			input: []inventory.AzureAsset{
				mockSQLServer("id1", "serverName1"),
				mockOther("id4"),
				mockSQLServer("id2", "serverName2"),
			},
			expected: []inventory.AzureAsset{
				mockSQLServerWithEncryptionProtectorExtension("id1", "serverName1", map[string]any{
					inventory.ExtensionSQLEncryptionProtectors: []map[string]any{epProps("serverKey1", true)},
				}),
				mockOther("id4"),
				mockSQLServer("id2", "serverName2"),
			},
			epRes: map[string]enricherResponse{
				"serverName1": assetRes(mockEncryptionProtector("ep1", epProps("serverKey1", true))),
				"serverName2": noRes(),
			},
			bapRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": noRes(),
			},
			tdeRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": noRes(),
			},
		},
		"Multiple encryption protectors": {
			input: []inventory.AzureAsset{
				mockSQLServer("id1", "serverName1"),
			},
			expected: []inventory.AzureAsset{
				mockSQLServerWithEncryptionProtectorExtension("id1", "serverName1", map[string]any{
					inventory.ExtensionSQLEncryptionProtectors: []map[string]any{
						epProps("serverKey1", true),
						epProps("serverKey2", false),
					},
				}),
			},
			epRes: map[string]enricherResponse{
				"serverName1": assetRes(
					mockEncryptionProtector("ep1", epProps("serverKey1", true)),
					mockEncryptionProtector("ep2", epProps("serverKey2", false))),
			},
			bapRes: map[string]enricherResponse{
				"serverName1": noRes(),
			},
			tdeRes: map[string]enricherResponse{
				"serverName1": noRes(),
			},
		},
		"Error in one encryption protector": {
			expectError: true,
			input: []inventory.AzureAsset{
				mockSQLServer("id1", "serverName1"),
				mockSQLServer("id2", "serverName2"),
			},
			expected: []inventory.AzureAsset{
				mockSQLServer("id1", "serverName1"),
				mockSQLServerWithEncryptionProtectorExtension("id2", "serverName2", map[string]any{
					inventory.ExtensionSQLEncryptionProtectors: []map[string]any{
						epProps("serverKey1", true),
					},
				}),
			},
			epRes: map[string]enricherResponse{
				"serverName1": errorRes(errors.New("error")),
				"serverName2": assetRes(mockEncryptionProtector("ep1", epProps("serverKey1", true))),
			},
			bapRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": noRes(),
			},
			tdeRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": noRes(),
			},
		},
		"Error in one blob audit policy": {
			expectError: true,
			input: []inventory.AzureAsset{
				mockSQLServer("id1", "serverName1"),
				mockSQLServer("id2", "serverName2"),
			},
			expected: []inventory.AzureAsset{
				mockSQLServer("id1", "serverName1"),
				mockSQLServerWithEncryptionProtectorExtension("id2", "serverName2", map[string]any{
					inventory.ExtensionSQLEncryptionProtectors: []map[string]any{
						epProps("serverKey1", true),
					},
				}),
			},
			epRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": assetRes(mockEncryptionProtector("ep1", epProps("serverKey1", true))),
			},
			bapRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": errorRes(errors.New("error")),
			},
			tdeRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": noRes(),
			},
		},
		"Multiple transparent data encryption": {
			input: []inventory.AzureAsset{
				mockSQLServer("id1", "serverName1"),
				mockSQLServer("id2", "serverName2"),
			},
			expected: []inventory.AzureAsset{
				mockSQLServerWithEncryptionProtectorExtension("id1", "serverName1", map[string]any{
					inventory.ExtensionSQLTransparentDataEncryptions: []map[string]any{
						tdeProps("Enabled"),
						tdeProps("Disabled"),
					},
				}),
				mockSQLServer("id2", "serverName2"),
			},
			epRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": noRes(),
			},
			bapRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": noRes(),
			},
			tdeRes: map[string]enricherResponse{
				"serverName1": assetRes(
					mockTransparentDataEncryption("tde1", tdeProps("Enabled")),
					mockTransparentDataEncryption("tde2", tdeProps("Disabled")),
				),
				"serverName2": noRes(),
			},
		},
		"Error in one transparent data encryption": {
			expectError: true,
			input: []inventory.AzureAsset{
				mockSQLServer("id1", "serverName1"),
				mockSQLServer("id2", "serverName2"),
			},
			expected: []inventory.AzureAsset{
				mockSQLServer("id1", "serverName1"),
				mockSQLServerWithEncryptionProtectorExtension("id2", "serverName2", map[string]any{
					inventory.ExtensionSQLEncryptionProtectors: []map[string]any{epProps("serverKey1", true)},
					inventory.ExtensionSQLBlobAuditPolicy:      bapProps("Enabled"),
				}),
			},
			epRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": assetRes(mockEncryptionProtector("ep1", epProps("serverKey1", true))),
			},
			bapRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": assetRes(mockBlobAuditingPolicies("ep1", bapProps("Enabled"))),
			},
			tdeRes: map[string]enricherResponse{
				"serverName1": noRes(),
				"serverName2": errorRes(errors.New("error")),
			},
		},
		"Encryption Protector,  blob audit policies and transparent data encryption": {
			input: []inventory.AzureAsset{
				mockSQLServer("id1", "serverName1"),
			},
			expected: []inventory.AzureAsset{
				mockSQLServerWithEncryptionProtectorExtension("id1", "serverName1", map[string]any{
					inventory.ExtensionSQLEncryptionProtectors:       []map[string]any{epProps("serverKey1", true)},
					inventory.ExtensionSQLBlobAuditPolicy:            bapProps("Disabled"),
					inventory.ExtensionSQLTransparentDataEncryptions: []map[string]any{tdeProps("Enabled")},
				}),
			},
			epRes: map[string]enricherResponse{
				"serverName1": assetRes(mockEncryptionProtector("ep1", epProps("serverKey1", true))),
			},
			bapRes: map[string]enricherResponse{
				"serverName1": assetRes(mockBlobAuditingPolicies("ep1", bapProps("Disabled"))),
			},
			tdeRes: map[string]enricherResponse{
				"serverName1": assetRes(mockTransparentDataEncryption("tde1", tdeProps("Enabled"))),
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			cmd := cycle.Metadata{}

			provider := azurelib.NewMockProviderAPI(t)
			for serverName, r := range tc.epRes {
				provider.EXPECT().ListSQLEncryptionProtector(mock.Anything, "subId", "group", serverName).Return(r.assets, r.err)
			}

			for serverName, r := range tc.bapRes {
				provider.EXPECT().GetSQLBlobAuditingPolicies(mock.Anything, "subId", "group", serverName).Return(r.assets, r.err)
			}

			for serverName, r := range tc.tdeRes {
				provider.EXPECT().ListSQLTransparentDataEncryptions(mock.Anything, "subId", "group", serverName).Return(r.assets, r.err)
			}

			e := sqlServerEnricher{provider: provider}

			err := e.Enrich(context.Background(), cmd, tc.input)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Equal(t, tc.expected, tc.input)
		})
	}
}

func mockSQLServerWithEncryptionProtectorExtension(id, name string, ext map[string]any) inventory.AzureAsset {
	m := mockSQLServer(id, name)
	m.Extension = ext
	return m
}

func mockSQLServer(id, name string) inventory.AzureAsset {
	return inventory.AzureAsset{
		Id:             id,
		SubscriptionId: "subId",
		ResourceGroup:  "group",
		Name:           name,
		Type:           inventory.SQLServersAssetType,
	}
}

func mockEncryptionProtector(id string, props map[string]any) inventory.AzureAsset {
	return inventory.AzureAsset{
		Id:         id,
		Type:       inventory.SQLServersAssetType + "/encryptionProtector",
		Properties: props,
	}
}

func mockBlobAuditingPolicies(id string, props map[string]any) inventory.AzureAsset {
	return inventory.AzureAsset{
		Id:         id,
		Type:       inventory.SQLServersAssetType + "/blobAuditPolicy",
		Properties: props,
	}
}

func mockTransparentDataEncryption(id string, props map[string]any) inventory.AzureAsset {
	return inventory.AzureAsset{
		Id:         id,
		Type:       inventory.SQLServersAssetType + "/transparentDataEncryption",
		Properties: props,
	}
}

func mockOther(id string) inventory.AzureAsset {
	return inventory.AzureAsset{
		Id:   id,
		Type: "otherType",
	}
}

func epProps(keyName string, rotationEnabled bool) map[string]any {
	return map[string]any{
		"kind":                "azurekeyvault",
		"serverKeyType":       "AzureKeyVault",
		"autoRotationEnabled": rotationEnabled,
		"serverKeyName":       keyName,
		"subregion":           "",
		"thumbprint":          "",
		"uri":                 "",
	}
}

func bapProps(state string) map[string]any {
	return map[string]any{
		"state":                        state,
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
	}
}

func tdeProps(state string) map[string]any {
	return map[string]any{
		"state": state,
	}
}
