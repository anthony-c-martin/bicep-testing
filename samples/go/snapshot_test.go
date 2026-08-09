package sample_test

import (
	"context"
	"path/filepath"
	"testing"

	biceptesting "github.com/anthony-c-martin/bicep-test/packages/go"
)

func TestInfrastructureHasExpectedResourcesAndNoDiagnostics(t *testing.T) {
	session, err := biceptesting.NewSession(context.Background(), "0.43.1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close session: %v", err)
		}
	})

	parametersPath, err := filepath.Abs(filepath.Join("..", "infra", "main.bicepparam"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := session.Snapshot(context.Background(), parametersPath, biceptesting.SnapshotMetadata{
		TenantID:       "00000000-0000-0000-0000-000000000000",
		SubscriptionID: "00000000-0000-0000-0000-000000000000",
		ResourceGroup:  "sample-rg",
		Location:       "eastus",
		DeploymentName: "sample-deployment",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Diagnostics) != 0 {
		t.Errorf("got %d diagnostics, want none", len(snapshot.Diagnostics))
	}
	if len(snapshot.PredictedResources) != 3 {
		t.Fatalf("got %d resources, want 3", len(snapshot.PredictedResources))
	}

	wantResources := map[string]string{
		"sampleprimary": "Microsoft.Storage/storageAccounts",
		"samplebackup":  "Microsoft.Storage/storageAccounts",
		"samplekv":      "Microsoft.KeyVault/vaults",
	}
	for _, resource := range snapshot.PredictedResources {
		if wantType, ok := wantResources[resource.Name]; !ok || resource.Type != wantType {
			t.Errorf("unexpected resource %q of type %q", resource.Name, resource.Type)
		}
	}
}
