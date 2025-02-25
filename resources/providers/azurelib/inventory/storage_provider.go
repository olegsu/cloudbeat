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
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
	"github.com/elastic/elastic-agent-libs/logp"
	"github.com/samber/lo"

	"github.com/elastic/cloudbeat/resources/fetching/cycle"
	"github.com/elastic/cloudbeat/resources/utils/maps"
	cloudbeat_strings "github.com/elastic/cloudbeat/resources/utils/strings"
)

type storageAccountAzureClientWrapper struct {
	AssetDiagnosticSettings func(ctx context.Context, subID string, options *armmonitor.DiagnosticSettingsClientListOptions) ([]armmonitor.DiagnosticSettingsClientListResponse, error)
	AssetBlobServices       func(ctx context.Context, subID string, clientOptions *arm.ClientOptions, resourceGroup, storageAccountName string, options *armstorage.BlobServicesClientListOptions) ([]armstorage.BlobServicesClientListResponse, error)
}

type StorageAccountProviderAPI interface {
	ListDiagnosticSettingsAssetTypes(ctx context.Context, cycleMetadata cycle.Metadata, subscriptionIDs []string) ([]AzureAsset, error)
	ListStorageAccountBlobServices(ctx context.Context, storageAccounts []AzureAsset) ([]AzureAsset, error)
	ListStorageAccountsBlobDiagnosticSettings(ctx context.Context, storageAccounts []AzureAsset) ([]AzureAsset, error)
	ListStorageAccountsTableDiagnosticSettings(ctx context.Context, storageAccounts []AzureAsset) ([]AzureAsset, error)
	ListStorageAccountsQueueDiagnosticSettings(ctx context.Context, storageAccounts []AzureAsset) ([]AzureAsset, error)
}

type storageAccountProvider struct {
	client                  *storageAccountAzureClientWrapper
	log                     *logp.Logger
	diagnosticSettingsCache *cycle.Cache[[]AzureAsset]
}

func NewStorageAccountProvider(log *logp.Logger, diagnosticSettingsClient *armmonitor.DiagnosticSettingsClient, credentials azcore.TokenCredential) StorageAccountProviderAPI {
	// We wrap the client, so we can mock it in tests
	wrapper := &storageAccountAzureClientWrapper{
		AssetDiagnosticSettings: func(ctx context.Context, resourceURI string, options *armmonitor.DiagnosticSettingsClientListOptions) ([]armmonitor.DiagnosticSettingsClientListResponse, error) {
			pager := diagnosticSettingsClient.NewListPager(resourceURI, options)
			return readPager(ctx, pager)
		},
		AssetBlobServices: func(ctx context.Context, subID string, clientOptions *arm.ClientOptions, resourceGroupName, storageAccountName string, options *armstorage.BlobServicesClientListOptions) ([]armstorage.BlobServicesClientListResponse, error) {
			cl, err := armstorage.NewBlobServicesClient(subID, credentials, clientOptions)
			if err != nil {
				return nil, err
			}
			return readPager(ctx, cl.NewListPager(resourceGroupName, storageAccountName, options))
		},
	}

	return &storageAccountProvider{
		log:                     log,
		client:                  wrapper,
		diagnosticSettingsCache: cycle.NewCache[[]AzureAsset](log),
	}
}

func (p *storageAccountProvider) ListStorageAccountBlobServices(ctx context.Context, storageAccounts []AzureAsset) ([]AzureAsset, error) {
	var assets []AzureAsset

	for _, sa := range storageAccounts {
		responses, err := p.client.AssetBlobServices(ctx, sa.SubscriptionId, nil, sa.ResourceGroup, sa.Name, nil)
		if err != nil {
			return nil, fmt.Errorf("error while fetching azure blob services for storage accounts %s: %w", sa.Id, err)
		}

		blobServices, err := transformBlobServices(responses, sa)
		if err != nil {
			return nil, fmt.Errorf("error while transforming azure blob services for storage accounts %s: %w", sa.Id, err)
		}

		assets = append(assets, blobServices...)
	}

	return assets, nil
}

func transformBlobServices(servicesPages []armstorage.BlobServicesClientListResponse, storageAccount AzureAsset) (_ []AzureAsset, errs error) {
	return lo.FlatMap(servicesPages, func(response armstorage.BlobServicesClientListResponse, _ int) []AzureAsset {
		return lo.Map(response.Value, func(item *armstorage.BlobServiceProperties, _ int) AzureAsset {
			properties, err := maps.AsMapStringAny(item.BlobServiceProperties)
			if err != nil {
				errs = errors.Join(errs, err)
			}

			return AzureAsset{
				Id:             cloudbeat_strings.Dereference(item.ID),
				Name:           cloudbeat_strings.Dereference(item.Name),
				Type:           strings.ToLower(cloudbeat_strings.Dereference(item.Type)),
				ResourceGroup:  storageAccount.ResourceGroup,
				SubscriptionId: storageAccount.SubscriptionId,
				TenantId:       storageAccount.TenantId,
				Properties:     properties,
				Extension: map[string]any{
					ExtensionStorageAccountID:   storageAccount.Id,
					ExtensionStorageAccountName: storageAccount.Name,
				},
			}
		})
	}), errs
}

func (p *storageAccountProvider) ListStorageAccountsBlobDiagnosticSettings(ctx context.Context, storageAccounts []AzureAsset) ([]AzureAsset, error) {
	return p.serviceDiagnosticSettings(ctx, "blobServices/default", storageAccounts)
}

func (p *storageAccountProvider) ListStorageAccountsTableDiagnosticSettings(ctx context.Context, storageAccounts []AzureAsset) ([]AzureAsset, error) {
	return p.serviceDiagnosticSettings(ctx, "tableServices/default", storageAccounts)
}

func (p *storageAccountProvider) ListStorageAccountsQueueDiagnosticSettings(ctx context.Context, storageAccounts []AzureAsset) ([]AzureAsset, error) {
	return p.serviceDiagnosticSettings(ctx, "queueServices/default", storageAccounts)
}

func (p *storageAccountProvider) ListDiagnosticSettingsAssetTypes(ctx context.Context, cycleMetadata cycle.Metadata, subscriptionIDs []string) ([]AzureAsset, error) {
	p.log.Info("Listing Azure Diagnostic Monitor Settings")

	return p.diagnosticSettingsCache.GetValue(ctx, cycleMetadata, func(ctx context.Context) ([]AzureAsset, error) {
		return p.getDiagnosticSettings(ctx, subscriptionIDs)
	})
}

func (p *storageAccountProvider) getDiagnosticSettings(ctx context.Context, subscriptionIDs []string) ([]AzureAsset, error) {
	var assets []AzureAsset

	for _, subID := range subscriptionIDs {
		responses, err := p.client.AssetDiagnosticSettings(ctx, fmt.Sprintf("/subscriptions/%s/", subID), nil)
		if err != nil {
			return nil, err
		}
		a, err := transformDiagnosticSettingsClientListResponses(responses, subID)
		if err != nil {
			return nil, err
		}
		assets = append(assets, a...)
	}

	return assets, nil
}

func (p *storageAccountProvider) serviceDiagnosticSettings(ctx context.Context, serviceIDPostfix string, storageAccounts []AzureAsset) ([]AzureAsset, error) {
	var assets []AzureAsset

	for _, sa := range storageAccounts {
		queueDiagSettings, err := p.storageAccountServiceDiagnosticSettings(ctx, serviceIDPostfix, sa)
		if err != nil {
			return nil, err
		}
		assets = append(assets, queueDiagSettings...)
	}

	return assets, nil
}

func (p *storageAccountProvider) storageAccountServiceDiagnosticSettings(ctx context.Context, idPostfix string, storageAccount AzureAsset) ([]AzureAsset, error) {
	res, err := p.client.AssetDiagnosticSettings(ctx, fmt.Sprintf("%s/%s", storageAccount.Id, idPostfix), nil)
	if err != nil {
		return nil, fmt.Errorf("error while fetching storage account service %s diagnostic settings: %w", idPostfix, err)
	}
	assets, err := transformDiagnosticSettingsClientListResponses(res, storageAccount.SubscriptionId)
	if err != nil {
		return nil, fmt.Errorf("error while transforming storage account service %s diagnostic settings: %w", idPostfix, err)
	}

	for i := range assets {
		(&assets[i]).AddExtension(ExtensionStorageAccountID, storageAccount.Id)
	}

	return assets, nil
}

func transformDiagnosticSettingsClientListResponses(response []armmonitor.DiagnosticSettingsClientListResponse, subID string) ([]AzureAsset, error) {
	var assets []AzureAsset

	for _, settingsCollection := range response {
		for _, v := range settingsCollection.Value {
			if v == nil {
				continue
			}
			a, err := transformDiagnosticSettingsResource(v, subID)
			if err != nil {
				return nil, fmt.Errorf("error parsing azure asset model: %w", err)
			}
			assets = append(assets, a)
		}
	}

	return assets, nil
}

func transformDiagnosticSettingsResource(v *armmonitor.DiagnosticSettingsResource, subID string) (AzureAsset, error) {
	properties, err := maps.AsMapStringAny(v.Properties)
	if err != nil {
		return AzureAsset{}, err
	}

	return AzureAsset{
		Id:             cloudbeat_strings.Dereference(v.ID),
		Name:           cloudbeat_strings.Dereference(v.Name),
		Location:       "global",
		Properties:     properties,
		ResourceGroup:  "",
		SubscriptionId: subID,
		TenantId:       "",
		Type:           cloudbeat_strings.Dereference(v.Type),
	}, nil
}
