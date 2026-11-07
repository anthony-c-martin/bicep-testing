package biceprpcclient

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestManagementGroupSnapshotVersionGate(t *testing.T) {
	client := &Client{version: "0.46.1"}
	_, err := client.GetSnapshot(context.Background(), GetSnapshotRequest{
		Path: "unused", Metadata: SnapshotMetadata{ManagementGroupID: "platform"},
	})
	if err == nil || !strings.Contains(err.Error(), "0.47.16") {
		t.Fatalf("expected version error, got %v", err)
	}
	_, err = client.GetSnapshot(context.Background(), GetSnapshotRequest{
		Path: "unused", Metadata: SnapshotMetadata{ManagementGroupID: " "},
	})
	if err == nil || !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("expected invalid-input error, got %v", err)
	}
	data, err := json.Marshal(SnapshotMetadata{ManagementGroupID: "platform"})
	if err != nil || string(data) != `{"managementGroupId":"platform"}` {
		t.Fatalf("metadata = %s, %v", data, err)
	}
}

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		actual   string
		minimum  string
		expected bool
	}{
		{actual: "0.46.1", minimum: "0.36.1", expected: true},
		{actual: "0.36.1", minimum: "0.36.1", expected: true},
		{actual: "0.25.3", minimum: "0.36.1", expected: false},
		{actual: "0.46.1+abc", minimum: "0.46.1", expected: true},
	}
	for _, test := range tests {
		if actual := versionAtLeast(test.actual, test.minimum); actual != test.expected {
			t.Errorf("versionAtLeast(%q, %q) = %t, want %t", test.actual, test.minimum, actual, test.expected)
		}
	}
}
