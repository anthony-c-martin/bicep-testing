package biceptesting

import "encoding/json"

// SnapshotResult is the predicted result of evaluating a Bicep parameters file.
type SnapshotResult struct {
	PredictedResources []SnapshotResource `json:"predictedResources"`
	Diagnostics        []string           `json:"diagnostics"`
	Outputs            map[string]any     `json:"outputs"`
}

// SnapshotResource is a resource predicted by a Bicep snapshot.
type SnapshotResource struct {
	ID                   string         `json:"id"`
	Type                 string         `json:"type"`
	Name                 string         `json:"name"`
	APIVersion           string         `json:"apiVersion"`
	Location             string         `json:"location,omitempty"`
	Properties           map[string]any `json:"properties,omitempty"`
	AdditionalProperties map[string]any `json:"-"`
}

// UnmarshalJSON preserves resource fields that are not part of the common snapshot model.
func (resource *SnapshotResource) UnmarshalJSON(data []byte) error {
	type resourceAlias SnapshotResource
	var decoded resourceAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	var additionalProperties map[string]any
	if err := json.Unmarshal(data, &additionalProperties); err != nil {
		return err
	}
	for _, property := range []string{"id", "type", "name", "apiVersion", "location", "properties"} {
		delete(additionalProperties, property)
	}
	decoded.AdditionalProperties = additionalProperties
	*resource = SnapshotResource(decoded)
	return nil
}
