package biceprpcclient

import "encoding/json"

// CompileRequest contains the path to a Bicep file.
type CompileRequest struct {
	Path string `json:"path"`
}

// CompileResponse contains a compiled ARM template and diagnostics.
type CompileResponse struct {
	Success     bool             `json:"success"`
	Diagnostics []map[string]any `json:"diagnostics"`
	Contents    string           `json:"contents"`
}

// CompileParamsRequest contains the path and overrides for a Bicep parameters compilation.
type CompileParamsRequest struct {
	Path               string                     `json:"path"`
	ParameterOverrides map[string]json.RawMessage `json:"parameterOverrides"`
}

// CompileParamsResponse contains deployable ARM template and parameter JSON.
type CompileParamsResponse struct {
	Success        bool             `json:"success"`
	Diagnostics    []map[string]any `json:"diagnostics"`
	Template       string           `json:"template"`
	Parameters     string           `json:"parameters"`
	TemplateSpecID string           `json:"templateSpecId,omitempty"`
}

// FormatRequest contains the path to a Bicep file to format.
type FormatRequest struct {
	Path string `json:"path"`
}

// FormatResponse contains formatted Bicep source.
type FormatResponse struct {
	Contents string `json:"contents"`
}

// GetMetadataRequest contains the path to a Bicep file.
type GetMetadataRequest struct {
	Path string `json:"path"`
}

// GetMetadataResponse contains declarations and file metadata.
type GetMetadataResponse struct {
	Parameters []map[string]any `json:"parameters"`
	Outputs    []map[string]any `json:"outputs"`
	Exports    []map[string]any `json:"exports"`
	Metadata   []map[string]any `json:"metadata"`
}

// GetFileReferencesRequest contains the path to a Bicep file.
type GetFileReferencesRequest struct {
	Path string `json:"path"`
}

// GetFileReferencesResponse contains all referenced file paths.
type GetFileReferencesResponse struct {
	FilePaths []string `json:"filePaths"`
}

// GetDeploymentGraphRequest contains the path to a Bicep file.
type GetDeploymentGraphRequest struct {
	Path string `json:"path"`
}

// GetDeploymentGraphResponse contains dependency graph nodes and edges.
type GetDeploymentGraphResponse struct {
	Nodes []map[string]any `json:"nodes"`
	Edges []map[string]any `json:"edges"`
}

// SnapshotMetadata describes the Azure deployment context used to evaluate a snapshot.
type SnapshotMetadata struct {
	TenantID       string `json:"tenantId,omitempty"`
	SubscriptionID string `json:"subscriptionId,omitempty"`
	ResourceGroup  string `json:"resourceGroup,omitempty"`
	Location       string `json:"location,omitempty"`
	DeploymentName string `json:"deploymentName,omitempty"`
}

// GetSnapshotRequest contains the path and deployment context for a Bicep snapshot.
type GetSnapshotRequest struct {
	Path           string                  `json:"path"`
	Metadata       SnapshotMetadata        `json:"metadata"`
	ExternalInputs []SnapshotExternalInput `json:"externalInputs,omitempty"`
}

// SnapshotExternalInput supplies a value for a Bicep external input.
type SnapshotExternalInput struct {
	Kind   string `json:"kind"`
	Config any    `json:"config,omitempty"`
	Value  any    `json:"value"`
}

// GetSnapshotResponse contains the serialized deployment snapshot.
type GetSnapshotResponse struct {
	Snapshot string `json:"snapshot"`
}
