# Bicep RPC Client for Go

An independent Go client for programmatically interacting with the [Bicep CLI](https://github.com/Azure/bicep) over JSON-RPC.

## Getting started

### Install the module

```sh
go get github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client@v0.1.0
```

### Initialize the client

`Factory.Initialize` downloads and caches the requested Bicep CLI version, starts its JSON-RPC server, and returns a client. Always close the client to terminate the Bicep process.

```go
package main

import (
	"context"
	"fmt"
	"log"

	biceprpcclient "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client"
)

func main() {
	ctx := context.Background()
	client, err := (biceprpcclient.Factory{}).Initialize(ctx, biceprpcclient.Configuration{})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	version, err := client.Version(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Bicep CLI version: %s\n", version)
}
```

With no configuration, the factory downloads the latest Bicep CLI into `~/.bicep/bin/latest`. The client requires Bicep CLI 0.25.3 or later.

### Pin a specific Bicep version

Set `BicepVersion` without a leading `v`. Versioned CLIs are cached separately under `~/.bicep/bin/v<version>`.

```go
client, err := (biceprpcclient.Factory{}).Initialize(ctx, biceprpcclient.Configuration{
	BicepVersion: "0.46.1",
})
```

### Use an existing Bicep installation

Set `ExistingCLIPath` to skip downloading the CLI. This takes precedence over `BicepVersion` and `CacheRoot`.

```go
client, err := (biceprpcclient.Factory{}).Initialize(ctx, biceprpcclient.Configuration{
	ExistingCLIPath: "/usr/local/bin/bicep",
})
```

### Use a custom cache directory

```go
client, err := (biceprpcclient.Factory{}).Initialize(ctx, biceprpcclient.Configuration{
	BicepVersion: "0.46.1",
	CacheRoot:    "/var/cache/bicep",
})
```

## Available operations

Every operation accepts a `context.Context` for cancellation and deadlines. Calls on a client are safe for concurrent use. Bicep compilation failures are returned in typed response diagnostics; transport and JSON-RPC failures are returned as Go errors.

### Compile

`Compile` compiles a `.bicep` file into an ARM template JSON string.

```go
result, err := client.Compile(ctx, biceprpcclient.CompileRequest{
	Path: "./main.bicep",
})
if err != nil {
	log.Fatal(err)
}
if result.Success {
	fmt.Println(result.Contents)
} else {
	fmt.Printf("diagnostics: %#v\n", result.Diagnostics)
}
```

`CompileResponse` contains `Success`, `Diagnostics`, and `Contents`.

### CompileParams

`CompileParams` compiles a `.bicepparam` file into ARM deployment parameters. `ParameterOverrides` contains JSON-encoded values keyed by parameter name.

```go
result, err := client.CompileParams(ctx, biceprpcclient.CompileParamsRequest{
	Path: "./main.bicepparam",
	ParameterOverrides: map[string]json.RawMessage{
		"environment":  json.RawMessage(`"test"`),
		"instanceCount": json.RawMessage(`2`),
	},
})
if err != nil {
	log.Fatal(err)
}
if result.Success {
	fmt.Println(result.Parameters)
	fmt.Println(result.Template)
	fmt.Println(result.TemplateSpecID)
}
```

`CompileParamsResponse` contains `Success`, `Diagnostics`, `Parameters`, `Template`, and `TemplateSpecID`. `TemplateSpecID` is populated when the parameters file references a template spec.

### Format

`Format` applies the standard Bicep formatter and returns the formatted source. It requires Bicep CLI 0.37.1 or later and does not write the file.

```go
result, err := client.Format(ctx, biceprpcclient.FormatRequest{
	Path: "./main.bicep",
})
if err != nil {
	log.Fatal(err)
}
if err := os.WriteFile("./main.bicep", []byte(result.Contents), 0o644); err != nil {
	log.Fatal(err)
}
```

`FormatResponse` contains `Contents`.

### GetMetadata

`GetMetadata` returns the parameters, outputs, exports, and file-level metadata declared by a Bicep file.

```go
result, err := client.GetMetadata(ctx, biceprpcclient.GetMetadataRequest{
	Path: "./main.bicep",
})
if err != nil {
	log.Fatal(err)
}
for _, parameter := range result.Parameters {
	fmt.Printf("parameter %v\n", parameter["name"])
}
for _, output := range result.Outputs {
	fmt.Printf("output %v\n", output["name"])
}
for _, exported := range result.Exports {
	fmt.Printf("export %v\n", exported["name"])
}
for _, item := range result.Metadata {
	fmt.Printf("metadata %v\n", item["name"])
}
```

`GetMetadataResponse` exposes `Parameters`, `Outputs`, `Exports`, and `Metadata` as JSON-shaped maps so fields added by future Bicep versions remain available.

### GetFileReferences

`GetFileReferences` returns every file referenced by a Bicep file, including modules, loaded files, and the entry point.

```go
result, err := client.GetFileReferences(ctx, biceprpcclient.GetFileReferencesRequest{
	Path: "./main.bicep",
})
if err != nil {
	log.Fatal(err)
}
for _, filePath := range result.FilePaths {
	fmt.Println(filePath)
}
```

`GetFileReferencesResponse` contains `FilePaths`.

### GetDeploymentGraph

`GetDeploymentGraph` returns resource nodes and dependency edges for visualization or graph analysis.

```go
result, err := client.GetDeploymentGraph(ctx, biceprpcclient.GetDeploymentGraphRequest{
	Path: "./main.bicep",
})
if err != nil {
	log.Fatal(err)
}
for _, node := range result.Nodes {
	fmt.Printf("node %v (%v)\n", node["name"], node["type"])
}
for _, edge := range result.Edges {
	fmt.Printf("%v -> %v\n", edge["source"], edge["target"])
}
```

`GetDeploymentGraphResponse` exposes `Nodes` and `Edges` as JSON-shaped maps.

### GetSnapshot

`GetSnapshot` evaluates a `.bicepparam` file using an Azure deployment context and returns a serialized deployment snapshot. It requires Bicep CLI 0.36.1 or later but does not deploy resources or require Azure credentials.

```go
result, err := client.GetSnapshot(ctx, biceprpcclient.GetSnapshotRequest{
	Path: "./main.bicepparam",
	Metadata: biceprpcclient.SnapshotMetadata{
		TenantID:       "00000000-0000-0000-0000-000000000000",
		SubscriptionID: "00000000-0000-0000-0000-000000000000",
		ResourceGroup:  "my-resource-group",
		Location:       "eastus",
		DeploymentName: "my-deployment",
	},
	ExternalInputs: []biceprpcclient.SnapshotExternalInput{
		{Kind: "sys.envVar", Config: "BUILD_ID", Value: "1234"},
	},
})
if err != nil {
	log.Fatal(err)
}
fmt.Println(result.Snapshot)
```

`SnapshotMetadata` contains `TenantID`, `SubscriptionID`, `ResourceGroup`, `Location`, and `DeploymentName`; each field is optional. `SnapshotExternalInput` contains `Kind`, `Config`, and `Value`. `GetSnapshotResponse` contains the JSON snapshot in `Snapshot`.

### Version

`Version` returns the connected Bicep CLI version. The first result is cached for the lifetime of the client.

```go
version, err := client.Version(ctx)
if err != nil {
	log.Fatal(err)
}
fmt.Println(version)
```

## Client lifecycle and errors

`Client.Close` is safe to call more than once. It closes the JSON-RPC connection and terminates the Bicep process.

JSON-RPC server errors are returned as `*RPCError`, which exposes `Code`, `Message`, and raw JSON `Data` in addition to implementing `error`.

```go
var rpcErr *biceprpcclient.RPCError
if errors.As(err, &rpcErr) {
	fmt.Printf("Bicep RPC error %d: %s\n", rpcErr.Code, rpcErr.Message)
}
```

## Low-level installation and initialization

Most applications should use `Factory.Initialize`. The following APIs are available when installation and process startup need to be controlled separately:

- `DownloadURL(ctx, version)` returns the Bicep download URL for the current operating system and architecture. An empty version resolves the latest release.
- `Install(ctx, basePath, version)` downloads Bicep into `basePath`, reuses an existing executable there, and returns its path. An empty version installs the latest release.
- `New(ctx, bicepPath)` starts an existing Bicep executable, connects over a Windows named pipe or Unix domain socket, verifies the CLI version, and returns a `*Client`.

```go
bicepPath, err := biceprpcclient.Install(ctx, "/var/cache/bicep/v0.46.1", "0.46.1")
if err != nil {
	log.Fatal(err)
}
client, err := biceprpcclient.New(ctx, bicepPath)
if err != nil {
	log.Fatal(err)
}
defer client.Close()
```

Automatic downloads support Windows, Linux, and macOS on x64 and Arm64.

## Public API summary

| API | Purpose |
| --- | --- |
| `Configuration`, `Factory.Initialize` | Select, cache, and start a Bicep CLI. |
| `Client`, `New`, `Client.Close` | Own a JSON-RPC connection and Bicep process. |
| `DownloadURL`, `Install` | Resolve and install Bicep CLI binaries. |
| `CompileRequest`, `CompileResponse`, `Client.Compile` | Compile a `.bicep` file. |
| `CompileParamsRequest`, `CompileParamsResponse`, `Client.CompileParams` | Compile a `.bicepparam` file with optional overrides. |
| `FormatRequest`, `FormatResponse`, `Client.Format` | Format Bicep source. |
| `GetMetadataRequest`, `GetMetadataResponse`, `Client.GetMetadata` | Read declarations and metadata. |
| `GetFileReferencesRequest`, `GetFileReferencesResponse`, `Client.GetFileReferences` | List referenced files. |
| `GetDeploymentGraphRequest`, `GetDeploymentGraphResponse`, `Client.GetDeploymentGraph` | Read resource graph nodes and edges. |
| `GetSnapshotRequest`, `SnapshotMetadata`, `SnapshotExternalInput`, `GetSnapshotResponse`, `Client.GetSnapshot` | Evaluate a deployment snapshot. |
| `Client.Version` | Read the connected CLI version. |
| `RPCError` | Inspect a JSON-RPC server error. |

This module is independent and non-official. Its API may change between releases.