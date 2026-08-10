# AnthonyCMartin.BicepTesting

Test the resources, outputs, and diagnostics produced by a Bicep deployment without deploying to Azure. The package can also run opt-in integration tests against a real Azure Deployment Stack when an assertion requires live resources.

This is an independent, non-official project.

## Requirements

- .NET 10 or later
- A `.bicepparam` entry point for the deployment under test

## Installation

```sh
dotnet add package AnthonyCMartin.BicepTesting --version 0.1.3
```

## Snapshot testing

Create and asynchronously dispose a session within the test lifetime:

```csharp
using AnthonyCMartin.BicepTesting;

await using var session = await BicepTestSession.CreateAsync("0.46.1");
var snapshot = await session.SnapshotAsync(
    "infra/main.bicepparam",
    tenantId: "00000000-0000-0000-0000-000000000000",
    subscriptionId: "00000000-0000-0000-0000-000000000000",
    resourceGroup: "example-rg",
    location: "eastus",
    deploymentName: "example-deployment");

Assert.IsEmpty(snapshot.Diagnostics);
var storageAccounts = snapshot.PredictedResources.Where(
    resource => resource.Type == "Microsoft.Storage/storageAccounts");
Assert.IsTrue(storageAccounts.Any());
Assert.IsTrue(storageAccounts.All(resource =>
    resource.Properties.GetProperty("allowBlobPublicAccess").GetBoolean() is false));
```

`BicepTestSession.CreateAsync()` downloads the requested Bicep CLI version and reuses it on later runs. The session owns a Bicep process, so use `await using` or call `DisposeAsync()` after the tests finish. Pass the test framework's `CancellationToken` to session creation and operations when available.

Snapshot tests run locally. The subscription, tenant, resource group, location, and deployment values provide evaluation context only; they do not need to exist, and no Azure credentials are required.

## Snapshot results

`SnapshotAsync()` returns:

- `PredictedResources`: resources and resolved properties predicted for the deployment
- `Outputs`: resolved deployment outputs
- `Diagnostics`: Bicep compilation warnings and errors

## Live deployment testing

Use `DeployAsync()` only when a test must inspect a real Azure resource or service response:

```csharp
using Azure.Identity;
using System.Text.Json;

await using var deployment = await session.DeployAsync(
    new DefaultAzureCredential(),
    new DeployOptions
    {
        FilePath = "infra/main.bicepparam",
        SubscriptionId = subscriptionId,
        ResourceGroup = resourceGroup,
        StackName = $"bicep-test-{Guid.NewGuid():N}",
        ParameterOverrides = new Dictionary<string, JsonElement>
        {
            ["env"] = JsonSerializer.SerializeToElement("integration"),
        },
    });

Assert.IsTrue(deployment.Resources.Any(
    resource => resource.Type == "Microsoft.Storage/storageAccounts"));
```

Live tests require Azure credentials, an existing resource group, and permission to create and delete Deployment Stacks and their managed resources. Asynchronous disposal is idempotent and deletes the stack and all resources it manages. Concurrent teardown calls share the same deletion; canceling one caller's wait does not cancel cleanup. Use a unique stack name and keep the result in an `await using` scope.

## More information

- [Runnable MSTest snapshot and deployment samples](https://github.com/anthony-c-martin/bicep-testing/tree/main/samples/dotnet)
- [Exported API](https://github.com/anthony-c-martin/bicep-testing/blob/main/api/dotnet/PublicAPI.Unshipped.txt)
- [Project repository](https://github.com/anthony-c-martin/bicep-testing)