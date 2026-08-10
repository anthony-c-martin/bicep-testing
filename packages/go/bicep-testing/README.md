# Bicep Testing Framework for Go

Test the resources, outputs, and diagnostics produced by a Bicep deployment without deploying to Azure. The package can also run opt-in integration tests against a real Azure Deployment Stack when an assertion requires live resources.

This is an independent, non-official project.

## Requirements

- Go 1.25 or later
- A `.bicepparam` entry point for the deployment under test

## Installation

```sh
go get github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing@v0.1.4
```

## Snapshot testing

Create a session for the test and register cleanup immediately:

```go
package infra_test

import (
	"context"
	"testing"

	biceptesting "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing"
)

func TestStorageAccountsDisablePublicAccess(t *testing.T) {
	ctx := context.Background()
	session, err := biceptesting.NewSession(ctx, "0.46.1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("close session: %v", err)
		}
	})

	snapshot, err := session.Snapshot(ctx, "infra/main.bicepparam", biceptesting.SnapshotMetadata{
		TenantID:       "00000000-0000-0000-0000-000000000000",
		SubscriptionID: "00000000-0000-0000-0000-000000000000",
		ResourceGroup:  "example-rg",
		Location:       "eastus",
		DeploymentName: "example-deployment",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(snapshot.Diagnostics) != 0 {
		t.Fatalf("got %d diagnostics, want none", len(snapshot.Diagnostics))
	}
	for _, resource := range snapshot.PredictedResources {
		if resource.Type == "Microsoft.Storage/storageAccounts" &&
			resource.Properties["allowBlobPublicAccess"] != false {
			t.Errorf("storage account %q allows public blob access", resource.Name)
		}
	}
}
```

`NewSession()` downloads the requested Bicep CLI version into `~/.bicep/bin` and reuses it on later runs. The session owns a Bicep process, so always call `Close()`. Every operation accepts a `context.Context` for cancellation and deadlines.

Snapshot tests run locally. The subscription, tenant, resource group, location, and deployment values provide evaluation context only; they do not need to exist, and no Azure credentials are required.

## Snapshot results

`Snapshot()` returns:

- `PredictedResources`: resources and resolved properties predicted for the deployment
- `Outputs`: resolved deployment outputs
- `Diagnostics`: Bicep compilation warnings and errors

## Live deployment testing

Use `Deploy()` only when a test must inspect a real Azure resource or service response:

```go
deployment, err := session.Deploy(ctx, credential, biceptesting.DeployOptions{
	FilePath:       "infra/main.bicepparam",
	SubscriptionID: subscriptionID,
	ResourceGroup:  resourceGroup,
	StackName:      fmt.Sprintf("bicep-test-%d", time.Now().UnixNano()),
})
if err != nil {
	t.Fatal(err)
}
t.Cleanup(func() {
	if err := deployment.Teardown(context.Background()); err != nil {
		t.Errorf("tear down deployment: %v", err)
	}
})
```

Live tests require an `azcore.TokenCredential`, an existing resource group, and permission to create and delete Deployment Stacks and their managed resources. `Teardown()` deletes the stack and all resources it manages. Concurrent calls share an active deletion, successful cleanup is idempotent, and a failed deletion can be retried. Use a unique stack name and register teardown immediately after deployment.

Lower-level Bicep integrations can use the separately versioned [bicep-rpc-client module](https://github.com/anthony-c-martin/bicep-testing/tree/main/packages/go/bicep-rpc-client).

## More information

- [Runnable Go snapshot and deployment samples](https://github.com/anthony-c-martin/bicep-testing/tree/main/samples/go)
- [Exported API](https://github.com/anthony-c-martin/bicep-testing/blob/main/api/go/biceptesting.txt)
- [Project repository](https://github.com/anthony-c-martin/bicep-testing)