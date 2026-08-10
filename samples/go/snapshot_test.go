package sample_test

import (
	"context"
	"path/filepath"
	"testing"

	biceptesting "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing"
)

func TestSnapshots(t *testing.T) {
	session := newTestSession(t)
	t.Run("environment parameters select topology, SKUs, and tags", func(t *testing.T) {
		testEnvironmentParametersSelectTopologySKUsAndTags(t, session)
	})
	t.Run("security baseline catches weakened parameters", func(t *testing.T) {
		testSecurityBaselineCatchesWeakenedParameters(t, session)
	})
	t.Run("private network references are wired together", func(t *testing.T) {
		testPrivateNetworkReferencesAreWiredTogether(t, session)
	})
}

func testEnvironmentParametersSelectTopologySKUsAndTags(t *testing.T, session *biceptesting.Session) {
	development := takeSnapshot(t, session, "environment-topology/dev.bicepparam")
	production := takeSnapshot(t, session, "environment-topology/prod.bicepparam")

	if len(development.Diagnostics) != 0 || len(development.PredictedResources) != 1 {
		t.Fatalf("development snapshot: %d diagnostics, %d resources", len(development.Diagnostics), len(development.PredictedResources))
	}
	developmentStorage := development.PredictedResources[0]
	if developmentStorage.Name != "ordersdevprimary" || nested(developmentStorage.AdditionalProperties, "sku", "name") != "Standard_LRS" {
		t.Errorf("unexpected development storage: %#v", developmentStorage)
	}
	if nested(developmentStorage.AdditionalProperties, "tags", "environment") != "dev" || development.Outputs["auditStorageId"] != nil {
		t.Error("development tags or conditional audit output did not match")
	}

	if len(production.PredictedResources) != 2 {
		t.Fatalf("production resources = %d, want 2", len(production.PredictedResources))
	}
	if production.PredictedResources[0].Name != "ordersprodprimary" || production.PredictedResources[1].Name != "ordersprodaudit" {
		t.Errorf("unexpected production topology: %#v", production.PredictedResources)
	}
	if nested(production.PredictedResources[0].AdditionalProperties, "sku", "name") != "Standard_ZRS" ||
		nested(production.PredictedResources[0].AdditionalProperties, "tags", "dataClassification") != "confidential" ||
		nested(production.PredictedResources[1].AdditionalProperties, "sku", "name") != "Standard_GRS" {
		t.Error("production SKUs or tags did not match")
	}
}

func testSecurityBaselineCatchesWeakenedParameters(t *testing.T, session *biceptesting.Session) {
	secure := takeSnapshot(t, session, "security-baseline/secure.bicepparam")
	insecure := takeSnapshot(t, session, "security-baseline/insecure.bicepparam")
	secureStorage := resourceByType(t, secure, "Microsoft.Storage/storageAccounts")
	secureVault := resourceByType(t, secure, "Microsoft.KeyVault/vaults")
	insecureStorage := resourceByType(t, insecure, "Microsoft.Storage/storageAccounts")

	if secureStorage.Properties["allowBlobPublicAccess"] != false || secureStorage.Properties["allowSharedKeyAccess"] != false ||
		secureStorage.Properties["minimumTlsVersion"] != "TLS1_2" || secureStorage.Properties["publicNetworkAccess"] != "Disabled" {
		t.Errorf("storage security baseline not applied: %#v", secureStorage.Properties)
	}
	if secureVault.Properties["enablePurgeProtection"] != true || secureVault.Properties["enableRbacAuthorization"] != true ||
		secureVault.Properties["softDeleteRetentionInDays"] != float64(90) {
		t.Errorf("vault security baseline not applied: %#v", secureVault.Properties)
	}
	if insecureStorage.Properties["minimumTlsVersion"] != "TLS1_0" || insecureStorage.Properties["allowBlobPublicAccess"] != true {
		t.Error("weakened fixture did not expose the expected regression")
	}
}

func testPrivateNetworkReferencesAreWiredTogether(t *testing.T, session *biceptesting.Session) {
	snapshot := takeSnapshot(t, session, "private-network/main.bicepparam")
	resources := map[string]biceptesting.SnapshotResource{}
	for _, resource := range snapshot.PredictedResources {
		resources[resource.Name] = resource
	}
	networkIDs := snapshot.Outputs["networkIds"].(map[string]any)
	appProperties := resources["orders-vnet/app"].Properties
	dataProperties := resources["orders-vnet/data"].Properties
	endpointProperties := resources["orders-storage-pe"].Properties
	connection := endpointProperties["privateLinkServiceConnections"].([]any)[0].(map[string]any)["properties"].(map[string]any)

	if nested(resources["orders-vnet"].Properties, "addressSpace", "addressPrefixes") == nil || appProperties["addressPrefix"] != "10.42.1.0/24" {
		t.Error("virtual network address plan did not match")
	}
	if nested(appProperties, "networkSecurityGroup", "id") == nil || dataProperties["privateEndpointNetworkPolicies"] != "Disabled" {
		t.Error("subnet security wiring did not match")
	}
	if nested(endpointProperties, "subnet", "id") != networkIDs["dataSubnetId"] || nested(connection, "privateLinkServiceId") == nil {
		t.Error("private endpoint wiring did not match outputs")
	}
	if nested(resources["privatelink.blob.core.windows.net/orders-vnet-link"].Properties, "virtualNetwork", "id") != networkIDs["virtualNetworkId"] {
		t.Error("private DNS link did not target the virtual network")
	}
}

func newTestSession(t *testing.T) *biceptesting.Session {
	t.Helper()
	session, err := biceptesting.NewSession(context.Background(), "0.46.1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close session: %v", err)
		}
	})
	return session
}

func takeSnapshot(t *testing.T, session *biceptesting.Session, relativePath string) *biceptesting.SnapshotResult {
	t.Helper()
	parametersPath, err := filepath.Abs(filepath.Join("..", "infra", relativePath))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := session.Snapshot(context.Background(), parametersPath, biceptesting.SnapshotMetadata{
		TenantID: "ddbe463a-0554-485d-b589-0b17d60cd38b", SubscriptionID: "28c9069e-23e8-47d2-b640-00d2e0f09616",
		ResourceGroup: "sample-rg", Location: "eastus", DeploymentName: "sample-deployment",
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func resourceByType(t *testing.T, snapshot *biceptesting.SnapshotResult, resourceType string) biceptesting.SnapshotResource {
	t.Helper()
	for _, resource := range snapshot.PredictedResources {
		if resource.Type == resourceType {
			return resource
		}
	}
	t.Fatalf("resource type %s not found", resourceType)
	return biceptesting.SnapshotResource{}
}

func nested(value map[string]any, keys ...string) any {
	var current any = value
	for _, key := range keys {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[key]
	}
	return current
}
