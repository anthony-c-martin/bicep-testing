package biceptest

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	tenantID       = "00000000-0000-0000-0000-000000000000"
	subscriptionID = "00000000-0000-0000-0000-000000000000"
	resourceGroup  = "test-rg"
	location       = "eastus"
	deploymentName = "test-deployment"
)

func TestSnapshotMatchesReferenceBehavior(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	session, err := NewSession(ctx, "0.43.1")
	if err != nil {
		t.Fatalf("NewSession returned an error: %v", err)
	}
	defer session.Close()

	snapshot, err := session.Snapshot(ctx, fixturePath(t), SnapshotMetadata{
		TenantID:       tenantID,
		SubscriptionID: subscriptionID,
		ResourceGroup:  resourceGroup,
		Location:       location,
		DeploymentName: deploymentName,
	})
	if err != nil {
		t.Fatalf("Snapshot returned an error: %v", err)
	}
	if len(snapshot.Diagnostics) != 0 {
		t.Errorf("Snapshot returned diagnostics: %v", snapshot.Diagnostics)
	}

	var storageAccounts, keyVaults []SnapshotResource
	for _, resource := range snapshot.PredictedResources {
		if !strings.EqualFold(resource.Location, location) {
			t.Errorf("resource %q location = %q, want %q", resource.Name, resource.Location, location)
		}
		switch resource.Type {
		case "Microsoft.Storage/storageAccounts":
			storageAccounts = append(storageAccounts, resource)
			if actual := resource.Properties["allowBlobPublicAccess"]; actual != false {
				t.Errorf("storage account %q allowBlobPublicAccess = %v, want false", resource.Name, actual)
			}
			if actual := resource.Properties["minimumTlsVersion"]; actual != "TLS1_2" {
				t.Errorf("storage account %q minimumTlsVersion = %v, want TLS1_2", resource.Name, actual)
			}
		case "Microsoft.KeyVault/vaults":
			keyVaults = append(keyVaults, resource)
			if actual := resource.Properties["enableSoftDelete"]; actual != true {
				t.Errorf("key vault %q enableSoftDelete = %v, want true", resource.Name, actual)
			}
			if actual := resource.Properties["softDeleteRetentionInDays"]; actual != float64(90) {
				t.Errorf("key vault %q softDeleteRetentionInDays = %v, want 90", resource.Name, actual)
			}
		case "Microsoft.Network/virtualNetworks":
			t.Errorf("snapshot unexpectedly contains virtual network %q", resource.Name)
		}
	}
	if len(storageAccounts) != 2 {
		t.Errorf("snapshot contains %d storage accounts, want 2", len(storageAccounts))
	}
	if len(keyVaults) != 1 {
		t.Errorf("snapshot contains %d key vaults, want 1", len(keyVaults))
	}
	assertResourceName(t, storageAccounts, "testprimary")
	assertResourceName(t, storageAccounts, "testbackup")

	expectedID := "/subscriptions/" + subscriptionID + "/resourceGroups/" + resourceGroup + "/providers/Microsoft.Storage/storageAccounts/testprimary"
	if actual := snapshot.Outputs["primaryStorageId"]; actual != expectedID {
		t.Errorf("primaryStorageId = %v, want %q", actual, expectedID)
	}
}

func fixturePath(t *testing.T) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not locate current test file")
	}
	return filepath.Join(filepath.Dir(currentFile), "..", "node", "test", "samples", "snapshot", "main.bicepparam")
}

func assertResourceName(t *testing.T, resources []SnapshotResource, expected string) {
	t.Helper()
	for _, resource := range resources {
		if resource.Name == expected {
			return
		}
	}
	t.Errorf("snapshot does not contain resource named %q", expected)
}
