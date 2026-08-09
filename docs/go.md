# Go

The Go package provides helpers for testing the predicted resources, outputs, and diagnostics of a Bicep deployment without deploying to Azure.

## Requirements

- Go 1.24 or later
- A `.bicepparam` entry point for the Bicep deployment under test

## Installation

```sh
go get github.com/anthony-c-martin/bicep-testing/packages/go@v0.1.2
```

## Usage

Create one session for the test, capture the snapshot, and close the Bicep process when the test completes:

```go
package infra_test

import (
	"context"
	"testing"

	biceptesting "github.com/anthony-c-martin/bicep-testing/packages/go"
)

func TestStorageAccountsDisablePublicAccess(t *testing.T) {
	session, err := biceptesting.NewSession(context.Background(), "0.43.1")
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	snapshot, err := session.Snapshot(
		context.Background(),
		"infra/main.bicepparam",
		biceptesting.SnapshotMetadata{
			SubscriptionID: "00000000-0000-0000-0000-000000000000",
			ResourceGroup:  "my-resource-group",
			Location:       "eastus",
			DeploymentName: "my-deployment",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	for _, resource := range snapshot.PredictedResources {
		if resource.Type == "Microsoft.Storage/storageAccounts" &&
			resource.Properties["allowBlobPublicAccess"] != false {
			t.Errorf("storage account %q allows public blob access", resource.Name)
		}
	}
}
```

`biceptesting.NewSession` downloads the requested Bicep CLI version into `~/.bicep/bin` and reuses it on later runs. Snapshot tests do not require Azure credentials or an Azure subscription.

## JSON-RPC client

Lower-level integrations can use the independently versioned `bicep-rpc-client` module:

```sh
go get github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client@v0.1.2
```

```go
import biceprpcclient "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client"

client, err := (biceprpcclient.Factory{}).Initialize(ctx, biceprpcclient.Configuration{
	BicepVersion: "0.46.1",
})
if err != nil {
	return err
}
defer client.Close()

result, err := client.Compile(ctx, biceprpcclient.CompileRequest{Path: "infra/main.bicep"})
```

Set `ExistingCLIPath` to use an existing installation. Typed operations include `Compile`, `CompileParams`, `Format`, `GetMetadata`, `GetFileReferences`, `GetDeploymentGraph`, `GetSnapshot`, and cached `Version`; every operation accepts a `context.Context` for cancellation and deadlines.

## Snapshot result

A snapshot contains:

- `PredictedResources`: resources and resolved properties predicted for the deployment
- `Outputs`: resolved deployment outputs
- `Diagnostics`: compilation warnings and errors

## Live deployment tests

Use `Deploy` with an Azure `azcore.TokenCredential` when a test needs real resources or service behavior:

```go
deployment, err := session.Deploy(ctx, credential, biceptesting.DeployOptions{
	FilePath:       "infra/main.bicepparam",
	SubscriptionID: subscriptionID,
	ResourceGroup:  resourceGroup,
	StackName:      fmt.Sprintf("storage-test-%d", time.Now().UnixNano()),
})
if err != nil {
	t.Fatal(err)
}
defer deployment.Teardown(context.Background())

if deployment.Outputs["endpoint"] == "" {
	t.Fatal("deployment did not return an endpoint")
}
```

The result exposes normalized outputs and managed resource IDs/types. `Teardown` is idempotent and deletes the Deployment Stack and its managed resources. Live tests require Azure credentials, an existing resource group, and deployment/deletion permissions.

## Sample

See the runnable [Go test sample](../samples/go/snapshot_test.go) for a complete consumer test using the shared example infrastructure.

## Public API

The complete exported APIs are available for the [`biceptesting`](../api/go/biceptesting.txt) and [`biceprpcclient`](../api/go/bicep-rpc-client.txt) packages.