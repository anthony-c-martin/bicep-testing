package sample_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	biceptesting "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing"
)

func TestSnapshots(t *testing.T) {
	session, err := biceptesting.NewSession(context.Background(), "0.47.16")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close session: %v", err)
		}
	})
	resourceGroup := biceptesting.SnapshotMetadata{
		SubscriptionID: "28c9069e-23e8-47d2-b640-00d2e0f09616",
		ResourceGroup:  "sample-rg",
		Location:       "eastus",
	}

	t.Run("storage and Key Vault meet the security baseline", func(t *testing.T) {
		// Context is unnecessary when assertions only depend on explicit parameter values.
		snapshot := takeSnapshot(t, session, "security-baseline/secure.bicepparam", biceptesting.SnapshotMetadata{})
		storage := requireResource(t, snapshot, "Microsoft.Storage/storageAccounts", "orderssecurestore")
		vault := requireResource(t, snapshot, "Microsoft.KeyVault/vaults", "orders-secure-kv")

		if storage.Properties["allowBlobPublicAccess"] != false || storage.Properties["allowSharedKeyAccess"] != false ||
			storage.Properties["minimumTlsVersion"] != "TLS1_2" || storage.Properties["publicNetworkAccess"] != "Disabled" ||
			storage.Properties["supportsHttpsTrafficOnly"] != true {
			t.Errorf("storage must disable public/shared-key access and require HTTPS with TLS 1.2: %v", storage.Properties)
		}
		if vault.Properties["enablePurgeProtection"] != true || vault.Properties["enableRbacAuthorization"] != true ||
			vault.Properties["publicNetworkAccess"] != "Disabled" || vault.Properties["softDeleteRetentionInDays"] != float64(90) {
			t.Errorf("vault must use RBAC, disable public access and retain deleted secrets for 90 days: %v", vault.Properties)
		}
	})

	t.Run("development uses inexpensive storage without an audit account", func(t *testing.T) {
		snapshot := takeSnapshot(t, session, "environment-topology/dev.bicepparam", resourceGroup)
		storage, err := snapshot.ResourcesOfType("Microsoft.Storage/storageAccounts")
		if err != nil {
			t.Fatal(err)
		}
		if len(storage) != 1 {
			t.Errorf("development storage accounts = %d, want 1", len(storage))
		}
		primary := requireResource(t, snapshot, "Microsoft.Storage/storageAccounts", "ordersdevprimary")
		if snapshotValue(t, primary.AdditionalProperties, "/sku/name") != "Standard_LRS" {
			t.Error("development storage must use the inexpensive Standard_LRS SKU")
		}
		if snapshotValue(t, primary.AdditionalProperties, "/tags/environment") != "dev" ||
			snapshotValue(t, primary.AdditionalProperties, "/tags/dataClassification") != "internal" {
			t.Error("development storage must be tagged as dev/internal")
		}
		if snapshot.Outputs["primaryStorageId"] != primary.ID || snapshot.Outputs["auditStorageId"] != nil {
			t.Error("development must output its primary account ID and no audit account")
		}
	})

	t.Run("production uses resilient primary and audit storage", func(t *testing.T) {
		snapshot := takeSnapshot(t, session, "environment-topology/prod.bicepparam", resourceGroup)
		storage, err := snapshot.ResourcesOfType("Microsoft.Storage/storageAccounts")
		if err != nil {
			t.Fatal(err)
		}
		if len(storage) != 2 {
			t.Errorf("production storage accounts = %d, want 2", len(storage))
		}
		primary := requireResource(t, snapshot, "Microsoft.Storage/storageAccounts", "ordersprodprimary")
		audit := requireResource(t, snapshot, "Microsoft.Storage/storageAccounts", "ordersprodaudit")
		if snapshotValue(t, primary.AdditionalProperties, "/sku/name") != "Standard_ZRS" ||
			snapshotValue(t, audit.AdditionalProperties, "/sku/name") != "Standard_GRS" {
			t.Error("production must use zone-redundant primary storage and geo-redundant audit storage")
		}
		if snapshotValue(t, primary.AdditionalProperties, "/tags/environment") != "prod" ||
			snapshotValue(t, primary.AdditionalProperties, "/tags/dataClassification") != "confidential" {
			t.Error("production storage must be tagged as prod/confidential")
		}
		if snapshot.Outputs["primaryStorageId"] != primary.ID || snapshot.Outputs["auditStorageId"] != audit.ID {
			t.Error("production outputs must identify the primary and audit accounts")
		}
	})

	t.Run("private storage connects to the data subnet and private DNS", func(t *testing.T) {
		snapshot := takeSnapshot(t, session, "private-network/main.bicepparam", resourceGroup)
		network := requireResource(t, snapshot, "Microsoft.Network/virtualNetworks", "orders-vnet")
		appSubnet := requireResource(t, snapshot, "Microsoft.Network/virtualNetworks/subnets", "orders-vnet/app")
		dataSubnet := requireResource(t, snapshot, "Microsoft.Network/virtualNetworks/subnets", "orders-vnet/data")
		endpoint := requireResource(t, snapshot, "Microsoft.Network/privateEndpoints", "orders-storage-pe")
		storage := requireResource(t, snapshot, "Microsoft.Storage/storageAccounts", "ordersprivatestore")
		appNsg := requireResource(t, snapshot, "Microsoft.Network/networkSecurityGroups", "orders-app-nsg")
		dataNsg := requireResource(t, snapshot, "Microsoft.Network/networkSecurityGroups", "orders-data-nsg")
		dnsLink := requireResource(t, snapshot, "Microsoft.Network/privateDnsZones/virtualNetworkLinks",
			"privatelink.blob.core.windows.net/orders-vnet-link")

		if !reflect.DeepEqual(snapshotValue(t, network.Properties, "/addressSpace/addressPrefixes"), []any{"10.42.0.0/16"}) ||
			appSubnet.Properties["addressPrefix"] != "10.42.1.0/24" || dataSubnet.Properties["addressPrefix"] != "10.42.2.0/24" {
			t.Error("the virtual network and subnets must use the planned address ranges")
		}
		if snapshotValue(t, appSubnet.Properties, "/networkSecurityGroup/id") != appNsg.ID ||
			snapshotValue(t, dataSubnet.Properties, "/networkSecurityGroup/id") != dataNsg.ID ||
			dataSubnet.Properties["privateEndpointNetworkPolicies"] != "Disabled" {
			t.Error("each subnet must use its NSG and the data subnet must support private endpoints")
		}
		if snapshotValue(t, endpoint.Properties, "/subnet/id") != dataSubnet.ID ||
			snapshotValue(t, endpoint.Properties, "/privateLinkServiceConnections/0/properties/privateLinkServiceId") != storage.ID ||
			!reflect.DeepEqual(snapshotValue(t, endpoint.Properties, "/privateLinkServiceConnections/0/properties/groupIds"), []any{"blob"}) {
			t.Error("the blob private endpoint must connect the storage account to the data subnet")
		}
		if snapshotValue(t, dnsLink.Properties, "/virtualNetwork/id") != network.ID {
			t.Error("private DNS must be linked to the workload virtual network")
		}
		if snapshotValue(t, snapshot.Outputs, "/networkIds/virtualNetworkId") != network.ID ||
			snapshotValue(t, snapshot.Outputs, "/networkIds/appSubnetId") != appSubnet.ID ||
			snapshotValue(t, snapshot.Outputs, "/networkIds/dataSubnetId") != dataSubnet.ID ||
			snapshotValue(t, snapshot.Outputs, "/networkIds/privateEndpointId") != endpoint.ID {
			t.Error("network outputs must identify the resources consumers need to reference")
		}
	})

	t.Run("platform policy audits resources missing a cost-center tag", func(t *testing.T) {
		snapshot := takeSnapshot(t, session, "snapshot-scopes/management-group.bicepparam",
			biceptesting.SnapshotMetadata{ManagementGroupID: "sample-platform", Location: "eastus"})
		policy := requireResource(t, snapshot, "Microsoft.Authorization/policyDefinitions", "require-cost-center")
		scope, err := biceptesting.GetResourceScope(policy.ID)
		if err != nil {
			t.Fatal(err)
		}
		if scope.ManagementGroupID != "sample-platform" {
			t.Error("the cost-center policy must belong to the platform management group")
		}
		if snapshotValue(t, policy.Properties, "/policyRule/if/field") != "tags[costCenter]" ||
			snapshotValue(t, policy.Properties, "/policyRule/if/exists") != "false" ||
			snapshotValue(t, policy.Properties, "/policyRule/then/effect") != "audit" {
			t.Error("the policy must audit resources that do not have a cost-center tag")
		}
		if snapshot.Outputs["policyId"] != policy.ID {
			t.Error("the policy output must identify the policy consumers can assign")
		}
	})
}

func takeSnapshot(t *testing.T, session *biceptesting.Session, relativePath string, metadata biceptesting.SnapshotMetadata) *biceptesting.SnapshotResult {
	t.Helper()
	snapshot, err := session.Snapshot(context.Background(), filepath.Join("..", "infra", relativePath), metadata)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Diagnostics) != 0 {
		t.Fatalf("Bicep diagnostics: %v", snapshot.Diagnostics)
	}
	return snapshot
}

func requireResource(t *testing.T, snapshot *biceptesting.SnapshotResult, resourceType, name string) biceptesting.SnapshotResource {
	t.Helper()
	resource, err := snapshot.RequireResource(biceptesting.ResourceSelector{Type: resourceType, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return resource
}

func snapshotValue(t *testing.T, document any, pointer string) any {
	t.Helper()
	value, err := biceptesting.GetSnapshotValue(document, pointer)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
