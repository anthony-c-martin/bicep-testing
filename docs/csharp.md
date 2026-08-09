# C#

The C# library provides helpers for testing the predicted resources, outputs, and diagnostics of a Bicep deployment without deploying to Azure.

## Requirements

- .NET 10 or later
- A `.bicepparam` entry point for the Bicep deployment under test

## Installation

The package has not yet been published to NuGet. Until it is released, reference `packages/dotnet/src/BicepTest/BicepTest.csproj` from a local checkout.

## Usage

Create and dispose a session within the test lifetime:

```csharp
await using var session = await BicepTestSession.CreateAsync("0.43.1");
var snapshot = await session.SnapshotAsync(
	"infra/main.bicepparam",
	subscriptionId: "00000000-0000-0000-0000-000000000000",
	resourceGroup: "my-resource-group",
	location: "eastus",
	deploymentName: "my-deployment");

var storageAccounts = snapshot.PredictedResources
	.Where(resource => resource.Type == "Microsoft.Storage/storageAccounts");

foreach (var storageAccount in storageAccounts)
{
	Assert.IsFalse(
		storageAccount.Properties.GetProperty("allowBlobPublicAccess").GetBoolean());
}
```

`BicepTestSession.CreateAsync` downloads and reuses the requested Bicep CLI version. Snapshot tests do not require Azure credentials or an Azure subscription.

## Snapshot result

A snapshot contains:

- `PredictedResources`: resources and resolved properties predicted for the deployment
- `Outputs`: resolved deployment outputs
- `Diagnostics`: compilation warnings and errors

## Live deployment tests

Use `DeployAsync` with an Azure `TokenCredential` when a test must assert against deployed resources or service behavior:

```csharp
await using var deployment = await session.DeployAsync(
	new DefaultAzureCredential(),
	new DeployOptions
	{
		FilePath = "infra/main.bicepparam",
		SubscriptionId = subscriptionId,
		ResourceGroup = resourceGroup,
		StackName = $"storage-test-{Guid.NewGuid():N}",
	});

Assert.IsTrue(deployment.Resources.Any(
	resource => resource.Type == "Microsoft.Storage/storageAccounts"));
var endpoint = deployment.Outputs["endpoint"].GetString();
Assert.AreEqual(HttpStatusCode.OK, (await httpClient.GetAsync(endpoint)).StatusCode);
```

`DeployResult` exposes normalized outputs and managed resource IDs/types. `await using` calls the idempotent teardown and deletes the Deployment Stack and all resources it manages. Live tests require an existing resource group, Azure credentials, and deployment/deletion permissions.

## Sample

See the runnable [MSTest sample](../samples/dotnet/SnapshotTests.cs) for a complete consumer test using the shared example infrastructure.

## Public API

The complete exported C# API is available in [`api/dotnet/PublicAPI.Unshipped.txt`](../api/dotnet/PublicAPI.Unshipped.txt).