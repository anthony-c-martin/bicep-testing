package biceptesting

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var subscriptionIDPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ResourceSelector identifies a resource by ID, or by type and exact name.
type ResourceSelector struct {
	ID   string
	Type string
	Name string
}

// ResourceScope contains the scope components present in an Azure resource ID.
type ResourceScope struct {
	SubscriptionID    string
	ResourceGroup     string
	ManagementGroupID string
}

// ResourcesOfType selects resources using a case-insensitive type comparison.
func (snapshot *SnapshotResult) ResourcesOfType(resourceType string) ([]SnapshotResource, error) {
	if snapshot == nil || strings.TrimSpace(resourceType) == "" {
		return nil, errors.New("snapshot and resource type are required")
	}
	result := []SnapshotResource{}
	for _, resource := range snapshot.PredictedResources {
		if strings.EqualFold(resource.Type, resourceType) {
			result = append(result, resource)
		}
	}
	return result, nil
}

// RequireResource returns exactly one matching resource, or an error for missing or ambiguous matches.
func (snapshot *SnapshotResult) RequireResource(selector ResourceSelector) (SnapshotResource, error) {
	if snapshot == nil {
		return SnapshotResource{}, errors.New("snapshot is required")
	}
	if selector.ID != "" {
		if strings.TrimSpace(selector.ID) == "" || selector.Type != "" || selector.Name != "" {
			return SnapshotResource{}, errors.New("specify either ID or type and name")
		}
	} else if strings.TrimSpace(selector.Type) == "" || strings.TrimSpace(selector.Name) == "" {
		return SnapshotResource{}, errors.New("specify either ID or type and name")
	}
	var result SnapshotResource
	count := 0
	for _, resource := range snapshot.PredictedResources {
		if (selector.ID != "" && strings.EqualFold(resource.ID, selector.ID)) ||
			(selector.ID == "" && strings.EqualFold(resource.Type, selector.Type) && resource.Name == selector.Name) {
			result = resource
			count++
		}
	}
	if count != 1 {
		return SnapshotResource{}, fmt.Errorf("expected exactly one resource matching %+v; found %d", selector, count)
	}
	return result, nil
}

// GetResourceScope parses subscription, resource-group, management-group, and tenant-scoped IDs.
func GetResourceScope(id string) (ResourceScope, error) {
	invalid := func() (ResourceScope, error) {
		return ResourceScope{}, fmt.Errorf("invalid Azure resource ID: %q", id)
	}
	if !strings.HasPrefix(id, "/") || strings.ContainsAny(id, "?#") {
		return invalid()
	}
	parts := strings.Split(id[1:], "/")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return invalid()
		}
	}
	result := ResourceScope{}
	index := 0
	if strings.EqualFold(parts[0], "subscriptions") {
		if len(parts) < 2 || !subscriptionIDPattern.MatchString(parts[1]) {
			return invalid()
		}
		result.SubscriptionID = parts[1]
		index = 2
		if index < len(parts) && strings.EqualFold(parts[index], "resourceGroups") {
			if index+1 >= len(parts) {
				return invalid()
			}
			result.ResourceGroup = parts[index+1]
			index += 2
		}
	} else if !strings.EqualFold(parts[0], "providers") {
		return invalid()
	}
	if len(parts) >= 4 && strings.EqualFold(parts[0], "providers") &&
		strings.EqualFold(parts[1], "Microsoft.Management") && strings.EqualFold(parts[2], "managementGroups") {
		result.ManagementGroupID = parts[3]
	}
	for index < len(parts) {
		if !strings.EqualFold(parts[index], "providers") || index+1 >= len(parts) {
			return invalid()
		}
		index += 2
		start := index
		for index < len(parts) && !strings.EqualFold(parts[index], "providers") {
			index += 2
		}
		if index == start || index > len(parts) {
			return invalid()
		}
	}
	return result, nil
}

// GetSnapshotValue resolves an RFC 6901 JSON Pointer in decoded JSON, preserving explicit nulls.
func GetSnapshotValue(value any, pointer string) (any, error) {
	if pointer == "" {
		return value, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, errors.New("a JSON Pointer must be empty or start with /")
	}
	for _, encoded := range strings.Split(pointer[1:], "/") {
		for index := 0; index < len(encoded); index++ {
			if encoded[index] == '~' {
				index++
				if index >= len(encoded) || (encoded[index] != '0' && encoded[index] != '1') {
					return nil, fmt.Errorf("invalid JSON Pointer escape: %q", pointer)
				}
			}
		}
		key := strings.ReplaceAll(strings.ReplaceAll(encoded, "~1", "/"), "~0", "~")
		switch current := value.(type) {
		case map[string]any:
			var exists bool
			value, exists = current[key]
			if !exists {
				return nil, fmt.Errorf("JSON Pointer not found: %q", pointer)
			}
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || strconv.Itoa(index) != key || index >= len(current) {
				return nil, fmt.Errorf("JSON Pointer not found: %q", pointer)
			}
			value = current[index]
		default:
			return nil, fmt.Errorf("JSON Pointer not found: %q", pointer)
		}
	}
	return value, nil
}
