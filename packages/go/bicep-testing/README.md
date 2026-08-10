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
		TenantID:       "ddbe463a-0554-485d-b589-0b17d60cd38b",
		SubscriptionID: "28c9069e-23e8-47d2-b640-00d2e0f09616",
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

Use `LiveSession` only when a test must inspect a real Azure resource or service response:

```go
credential, err := azidentity.NewDefaultAzureCredential(nil)
if err != nil {
	t.Fatal(err)
}

live, err := biceptesting.NewLiveSession(ctx, "0.46.1", credential)
if err != nil {
	t.Fatal(err)
}
t.Cleanup(func() {
	if err := live.Close(); err != nil {
		t.Errorf("close live session: %v", err)
	}
})

options := biceptesting.DeployOptions{
	FilePath:       "infra/main.bicepparam",
	SubscriptionID: subscriptionID,
	ResourceGroup:  resourceGroup,
	ParameterOverrides: map[string]json.RawMessage{
		"env": json.RawMessage(`"integration"`),
	},
}

validation, err := live.Validate(ctx, options)
if err != nil {
	t.Fatal(err)
}
if !validation.IsValid {
	t.Fatalf("validation failed: %s", validation.ErrorMessage)
}

deployment, err := live.Deploy(ctx, options)
if err != nil {
	t.Fatal(err)
}
defer func() {
	if err := deployment.Teardown(context.Background()); err != nil {
		t.Errorf("tear down deployment: %v", err)
	}
})
if !deployment.Succeeded {
	t.Fatalf("deployment failed: %s", deployment.ErrorMessage)
}
```

`Session` remains offline-only and still provides `Snapshot()`. `LiveSession` owns both a credential and an offline session, forwards `Snapshot()`, and adds `Validate()` and `Deploy()`.

`DeployOptions` supports three scopes:

- resource group: set `SubscriptionID` and `ResourceGroup` (`Location` optional)
- subscription: set `SubscriptionID` and `Location`
- management group: set `ManagementGroupID` and `Location`

`FilePath` is always required. `StackName` is optional and defaults to a unique `bicep-test-...` value. `ParameterOverrides` is optional.

`Validate()` returns `IsValid`, validated `Resources`, `CorrelationID`, and an optional `Error` with `ErrorCode`, `ErrorMessage`, and raw service JSON.

`Deploy()` returns an `error` only for pre-submission failures (invalid options, compilation errors, or client construction failures). After Azure submission, failures are returned as a non-nil `DeployResult` with `Succeeded == false`, `Error`, and a working `Teardown()` handle.

Live tests require Azure credentials and permission to validate, create, and delete Deployment Stacks and their managed resources at the selected scope. `Teardown()` deletes the stack and all resources it manages, treats `404` as already cleaned up, shares one active deletion across concurrent callers, is idempotent after success, and allows retries after failed deletion attempts.

Lower-level Bicep integrations can use the separately versioned [bicep-rpc-client module](https://github.com/anthony-c-martin/bicep-testing/tree/main/packages/go/bicep-rpc-client).

## More information

- [Runnable Go snapshot and deployment samples](https://github.com/anthony-c-martin/bicep-testing/tree/main/samples/go)
- [Exported API](https://github.com/anthony-c-martin/bicep-testing/blob/main/api/go/biceptesting.txt)
- [Project repository](https://github.com/anthony-c-martin/bicep-testing)