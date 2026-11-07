package biceptesting

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func sharedInfra(relative string) string {
	return filepath.Join("..", "..", "..", "samples", "infra", relative)
}

func sharedTestData(relative string) string {
	return filepath.Join("..", "..", "testdata", relative)
}

func TestSnapshotHelpersSharedCases(t *testing.T) {
	data, err := os.ReadFile(sharedTestData("snapshot-helpers.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Scopes     []map[string]string
		InvalidIDs []string `json:"invalidIds"`
		Document   any
		Pointers   []struct {
			Pointer string
			Value   any
		}
		InvalidPointers []string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, item := range cases.Scopes {
		scope, err := GetResourceScope(item["id"])
		want := ResourceScope{SubscriptionID: item["subscriptionId"], ResourceGroup: item["resourceGroup"], ManagementGroupID: item["managementGroupId"]}
		if err != nil || scope != want {
			t.Errorf("GetResourceScope(%q) = %+v, %v; want %+v", item["id"], scope, err, want)
		}
	}
	for _, id := range cases.InvalidIDs {
		if _, err := GetResourceScope(id); err == nil {
			t.Errorf("GetResourceScope(%q) did not fail", id)
		}
	}
	for _, item := range cases.Pointers {
		value, err := GetSnapshotValue(cases.Document, item.Pointer)
		if err != nil || !reflect.DeepEqual(value, item.Value) {
			t.Errorf("GetSnapshotValue(%q) = %v, %v; want %v", item.Pointer, value, err, item.Value)
		}
	}
	for _, pointer := range cases.InvalidPointers {
		if _, err := GetSnapshotValue(cases.Document, pointer); err == nil {
			t.Errorf("GetSnapshotValue(%q) did not fail", pointer)
		}
	}
	value, err := GetSnapshotValue(cases.Document, "")
	if err != nil || !reflect.DeepEqual(value, cases.Document) {
		t.Fatal("empty pointer did not preserve the document")
	}
	empty := &SnapshotResult{}
	if resources, err := empty.ResourcesOfType("Missing/type"); err != nil || len(resources) != 0 {
		t.Fatalf("empty selection = %v, %v", resources, err)
	}
	resource := SnapshotResource{ID: "/id", Type: "Test/type", Name: "test"}
	duplicate := &SnapshotResult{PredictedResources: []SnapshotResource{resource, resource}}
	for _, item := range []struct {
		Snapshot *SnapshotResult
		Selector ResourceSelector
	}{
		{empty, ResourceSelector{ID: "/missing"}},
		{duplicate, ResourceSelector{ID: "/ID"}},
		{duplicate, ResourceSelector{Type: "test/TYPE", Name: "test"}},
		{duplicate, ResourceSelector{Type: "Test/type", Name: "TEST"}},
		{empty, ResourceSelector{}},
		{duplicate, ResourceSelector{ID: "/id", Type: "Test/type", Name: "test"}},
	} {
		if _, err := item.Snapshot.RequireResource(item.Selector); err == nil {
			t.Errorf("RequireResource(%+v) did not fail", item.Selector)
		}
	}
	if _, err := empty.ResourcesOfType(""); err == nil {
		t.Fatal("empty type did not fail")
	}
}

func TestSnapshotsEveryScopeAndOptionalContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	session, err := NewSession(ctx, "0.47.16")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	}()
	snapshot, err := session.Snapshot(ctx, sharedInfra("snapshot-scopes/management-group.bicepparam"),
		SnapshotMetadata{ManagementGroupID: "platform", Location: "eastus", DeploymentName: "offline-test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Diagnostics) != 0 || snapshot.Outputs["scopeId"] != "/providers/Microsoft.Management/managementGroups/platform" ||
		snapshot.Outputs["deploymentName"] != "offline-test" {
		t.Fatalf("unexpected management-group snapshot: %+v", snapshot)
	}
	policies, err := snapshot.ResourcesOfType("microsoft.authorization/POLICYDEFINITIONS")
	if err != nil || len(policies) != 1 {
		t.Fatalf("policies = %v, %v", policies, err)
	}
	policy, err := snapshot.RequireResource(ResourceSelector{Type: "Microsoft.Authorization/policyDefinitions", Name: "require-cost-center"})
	if err != nil {
		t.Fatal(err)
	}
	if byID, err := snapshot.RequireResource(ResourceSelector{ID: policy.ID}); err != nil || byID.ID != policy.ID {
		t.Fatal("ID selection failed")
	}
	if scope, err := GetResourceScope(policy.ID); err != nil || scope.ManagementGroupID != "platform" {
		t.Fatal("scope selection failed")
	}
	if value, err := GetSnapshotValue(policy.Properties, "/policyRule/then/effect"); err != nil || value != "audit" {
		t.Fatal("property selection failed")
	}
	for _, item := range []struct {
		Path     string
		Metadata SnapshotMetadata
		Output   string
		Value    string
	}{
		{sharedTestData("snapshot-scopes/subscription.bicepparam"), SnapshotMetadata{SubscriptionID: subscriptionID}, "scopeId", "/subscriptions/" + subscriptionID},
		{sharedTestData("snapshot-scopes/tenant.bicepparam"), SnapshotMetadata{TenantID: tenantID}, "tenantId", tenantID},
	} {
		result, err := session.Snapshot(ctx, item.Path, item.Metadata)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Diagnostics) != 0 || result.Outputs[item.Output] != item.Value {
			t.Fatalf("unexpected snapshot: %+v", result)
		}
	}
	minimal, err := session.Snapshot(ctx, sharedInfra("security-baseline/secure.bicepparam"), SnapshotMetadata{})
	if err != nil || len(minimal.Diagnostics) != 0 {
		t.Fatalf("minimal snapshot = %+v, %v", minimal, err)
	}
}
