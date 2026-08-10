package biceptesting

import (
	"encoding/json"
	"testing"
)

func TestSnapshotResourcePreservesAdditionalProperties(t *testing.T) {
	var resource SnapshotResource
	if err := json.Unmarshal([]byte(`{
		"id":"resource-id",
		"type":"Microsoft.Storage/storageAccounts",
		"name":"storage",
		"apiVersion":"2023-01-01",
		"location":"eastus",
		"properties":{"allowBlobPublicAccess":false},
		"kind":"StorageV2",
		"sku":{"name":"Standard_LRS"}
	}`), &resource); err != nil {
		t.Fatalf("Unmarshal returned an error: %v", err)
	}

	if actual := resource.AdditionalProperties["kind"]; actual != "StorageV2" {
		t.Errorf("kind = %v, want StorageV2", actual)
	}
	sku, ok := resource.AdditionalProperties["sku"].(map[string]any)
	if !ok {
		t.Fatalf("sku has type %T, want map[string]any", resource.AdditionalProperties["sku"])
	}
	if actual := sku["name"]; actual != "Standard_LRS" {
		t.Errorf("sku.name = %v, want Standard_LRS", actual)
	}
	if _, exists := resource.AdditionalProperties["name"]; exists {
		t.Error("AdditionalProperties unexpectedly contains known field name")
	}
}
