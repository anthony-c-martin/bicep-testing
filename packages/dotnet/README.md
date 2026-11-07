# AnthonyCMartin.BicepTesting

Test the resources, outputs, and diagnostics produced by a Bicep deployment without deploying to Azure. The package can also run opt-in integration tests against a real Azure Deployment Stack when an assertion requires live resources.

This is an independent, non-official project.

## Requirements

- .NET 10 or later
- A `.bicepparam` entry point for the deployment under test

## Installation

```sh
dotnet add package AnthonyCMartin.BicepTesting --version 0.1.6
```

## Snapshot testing

Create and asynchronously dispose a session within the test lifetime:

```csharp
using AnthonyCMartin.BicepTesting;

await using var session = await BicepTestSession.CreateAsync("0.46.1");
var snapshot = await session.SnapshotAsync(
    new SnapshotOptions
    {
        FilePath = "infra/main.bicepparam",
    });

Assert.IsEmpty(snapshot.Diagnostics);
var storageAccounts = snapshot.PredictedResources.Where(
    resource => resource.Type == "Microsoft.Storage/storageAccounts");
Assert.IsTrue(storageAccounts.Any());
Assert.IsTrue(storageAccounts.All(resource =>
    resource.Properties.GetProperty("allowBlobPublicAccess").GetBoolean() is false));
```

`BicepTestSession.CreateAsync()` downloads the requested Bicep CLI version and reuses it on later runs. The session owns a Bicep process, so use `await using` or call `DisposeAsync()` after the tests finish. Pass the test framework's `CancellationToken` to session creation and operations when available.

Snapshot tests run locally. The subscription, tenant, resource group, location, and deployment values provide evaluation context only; they do not need to exist, and no Azure credentials are required.

Only `FilePath` is required in snapshot options. Supply context when assertions
depend on it. `DeploymentName` has a generated default and also affects offline
`deployment().name` evaluation.

### Management-group context and helpers (unreleased)

The development checkout adds `SnapshotOptions.ManagementGroupId`, requiring
Bicep CLI 0.47.16+. Older CLIs throw `NotSupportedException`.

```csharp
await using var session = await BicepTestSession.CreateAsync("0.47.16");
var snapshot = await session.SnapshotAsync(new SnapshotOptions
{
    FilePath = "infra/management-group.bicepparam",
    ManagementGroupId = "example-platform",
    Location = "eastus",
    DeploymentName = "offline-test",
});
Assert.IsEmpty(snapshot.Diagnostics);
Assert.HasCount(1, snapshot.ResourcesOfType("Microsoft.Authorization/policyDefinitions"));
var policy = snapshot.RequireResource("Microsoft.Authorization/policyDefinitions", "require-cost-center");
Assert.AreEqual("example-platform", SnapshotHelpers.GetResourceScope(policy.Id).ManagementGroupId);
Assert.AreEqual("audit", SnapshotHelpers.GetSnapshotValue(policy.Properties, "/policyRule/then/effect").GetString());
```

Use the shared [management-group fixture](../../samples/infra/snapshot-scopes/).
`RequireResource(id)` is the alternative ID selector. Types and IDs match
case-insensitively; names match exactly. Type selection returns an empty array
when absent; required selection throws `InvalidOperationException` for zero or
multiple matches. Empty arguments throw `ArgumentException`.

`GetResourceScope()` returns read-only subscription/resource-group/management-group
components, with `null` for absent fields. It supports tenant, nested, and
extension IDs, rejects malformed IDs, and requires GUID subscription IDs.
Tenant IDs are not encoded in resource IDs.

`GetSnapshotValue()` resolves an RFC 6901 JSON Pointer in a `JsonElement`:
`""` returns the input, `/items/0` selects an array item, and `~0`/`~1` escape
`~`/`/`. The returned `JsonElement` preserves explicit null; missing paths throw
`KeyNotFoundException`, and invalid syntax throws `ArgumentException`. Raw JSON
access and existing result models remain available.

## Snapshot results

`SnapshotAsync()` returns:

- `PredictedResources`: resources and resolved properties predicted for the deployment
- `Outputs`: resolved deployment outputs
- `Diagnostics`: Bicep compilation warnings and errors

## Azure validation

Use `ValidateAsync()` to compile a parameters file and run Azure Deployment Stack preflight validation without creating resources:

```csharp
using Azure.Identity;

await using var session = await LiveBicepTestSession.CreateAsync(
    "0.46.1",
    new DefaultAzureCredential());
var validation = await session.ValidateAsync(
    new DeployOptions
    {
        FilePath = "infra/main.bicepparam",
        SubscriptionId = subscriptionId,
        ResourceGroup = resourceGroup,
    });

Assert.IsTrue(validation.IsValid, validation.ErrorMessage);
```

Validation requires Azure credentials and access to the target scope. The result includes validated resources, a correlation ID, and error code/message when Azure rejects the deployment. Validation and deployment results use the same `OperationError` shape; `Error.RawData` contains the complete ARM error JSON, including nested details and targets, for logging or additional assertions. Validation does not create a Deployment Stack or resources.

## Live deployment testing

Use `DeployAsync()` only when a test must inspect a real Azure resource or service response:

```csharp
using Azure.Identity;
using System.Text.Json;

await using var session = await LiveBicepTestSession.CreateAsync(
    "0.46.1",
    new DefaultAzureCredential());
await using var deployment = await session.DeployAsync(
    new DeployOptions
    {
        FilePath = "infra/main.bicepparam",
        SubscriptionId = subscriptionId,
        ResourceGroup = resourceGroup,
        ParameterOverrides = new Dictionary<string, JsonElement>
        {
            ["env"] = JsonSerializer.SerializeToElement("integration"),
        },
    });

Assert.IsTrue(deployment.Resources.Any(
    resource => resource.Type == "Microsoft.Storage/storageAccounts"));
```

If Azure accepts the operation but deployment later fails or is canceled, `DeployAsync()` returns a result with `Succeeded` set to `false` and structured `Error`, `ErrorCode`, and `ErrorMessage` values. `Error.RawData` contains the complete Azure response body when available. The result still owns cleanup for the named stack and any partially created resources:

```csharp
await using var deployment = await session.DeployAsync(options);
if (!deployment.Succeeded)
{
    Assert.Fail(deployment.ErrorMessage);
}

// Assert on the live resources.
```

Set `SubscriptionId` and `ResourceGroup` for resource-group scope. Set only `SubscriptionId` for subscription scope, or only `ManagementGroupId` for management-group scope. Subscription and management-group scopes also require `Location` because their Deployment Stack metadata is regional.

Live tests require Azure credentials and permission to create and delete Deployment Stacks and their managed resources at the target scope. A unique stack name is generated by default; set `StackName` only when multiple deployments must update the same stack. Asynchronous disposal is idempotent and deletes the stack and all resources it manages. Concurrent teardown calls share the same deletion; canceling one caller's wait does not cancel cleanup. Keep the result in an `await using` scope.

## More information

The [live sample](../../samples/dotnet/LiveTests.cs) includes an opt-in identical
redeployment test with cleanup after both deployments. Reuse one `DeployOptions`
instance (including its generated `StackName`), or set the same name explicitly.
For AI-generated tests, copy the [consumer authoring skill](../../.github/skills/author-bicep-tests/)
into your repository. Automated scenario coverage is not yet available.

- [Runnable MSTest snapshot and deployment samples](https://github.com/anthony-c-martin/bicep-testing/tree/main/samples/dotnet)
- [Exported API](https://github.com/anthony-c-martin/bicep-testing/blob/main/api/dotnet/PublicAPI.txt)
- [Project repository](https://github.com/anthony-c-martin/bicep-testing)