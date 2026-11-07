# AnthonyCMartin.BicepTesting

Test the resources, outputs, and diagnostics produced by a Bicep deployment without deploying to Azure. The module can also run opt-in integration tests against a real Azure Deployment Stack when an assertion requires live resources.

This is an independent, non-official project.

## Requirements

- PowerShell 7.6 or later
- A `.bicepparam` entry point for the deployment under test

## Installation

```powershell
Install-PSResource AnthonyCMartin.BicepTesting -Version 0.1.6 -Repository PSGallery
Import-Module AnthonyCMartin.BicepTesting -RequiredVersion 0.1.6
```

## Snapshot testing

Create one session for the test suite and remove it when the suite completes:

```powershell
BeforeAll {
    $session = New-BicepTestSession -BicepVersion '0.46.1'
    $snapshot = $session | Get-BicepSnapshot -Path 'infra/main.bicepparam'
}

AfterAll {
    $session | Remove-BicepTestSession
}

Describe 'Bicep infrastructure' {
    It 'disables public blob access' {
        $snapshot.Diagnostics | Should -BeNullOrEmpty
        $storageAccounts = $snapshot.PredictedResources |
            Where-Object Type -eq 'Microsoft.Storage/storageAccounts'
        $storageAccounts | Should -Not -BeNullOrEmpty
        $storageAccounts.Properties.GetProperty('allowBlobPublicAccess').GetBoolean() |
            Should -Not -Contain $true
    }
}
```

`New-BicepTestSession` downloads the requested Bicep CLI version and reuses it on later runs. The session owns a Bicep process, so always call `Remove-BicepTestSession` in `AfterAll` or a `finally` block.

Snapshot tests run locally. The subscription, tenant, resource group, location, and deployment values provide evaluation context only; they do not need to exist, and no Azure credentials are required.

Only the session and path are required. Supply context when assertions depend on
its values. `-DeploymentName` also affects offline `deployment().name` evaluation
and otherwise uses the .NET generated default.

### Management-group context and native helpers (unreleased)

The development checkout adds `-ManagementGroupId` for snapshots, requiring
Bicep CLI 0.47.16+. Older CLIs fail with a terminating error.

```powershell
$session = New-BicepTestSession -BicepVersion '0.47.16'
try {
    $snapshot = $session | Get-BicepSnapshot -Path 'infra/management-group.bicepparam' `
        -ManagementGroupId 'example-platform' -Location 'eastus' -DeploymentName 'offline-test'
    $snapshot.Diagnostics | Should -BeNullOrEmpty
    @($snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Authorization/policyDefinitions') |
        Should -HaveCount 1
    $policy = $snapshot | Get-BicepSnapshotResource `
        -Type 'Microsoft.Authorization/policyDefinitions' -Name 'require-cost-center'
    ($policy | Get-BicepResourceScope).ManagementGroupId | Should -Be 'example-platform'
    $policy | Get-BicepSnapshotValue -Pointer '/properties/policyRule/then/effect' | Should -Be 'audit'
}
finally {
    $session | Remove-BicepTestSession
}
```

Use the shared [management-group fixture](../../samples/infra/snapshot-scopes/).
`Get-BicepSnapshotResource -Id` requires one resource by ID; `-Type -Name` requires
one by type and exact name. Types and IDs match case-insensitively. `-Type` alone
selects any number of resources; wrap it in `@(...)` before asserting count.
Required selection fails with terminating errors for zero/multiple matches;
invalid arguments also fail explicitly.

`Get-BicepResourceScope` accepts an ID or a piped resource with an `Id` property.
It supports subscription, resource-group, management-group, tenant, child, and
extension IDs, returning `null` for absent components. Malformed IDs fail, and
subscription IDs must be GUIDs. Tenant IDs are not encoded in resource IDs.

`Get-BicepSnapshotValue` resolves an RFC 6901 JSON Pointer on a snapshot/resource,
`JsonElement`, or native JSON object. `""` returns the root, `/items/0` indexes
an array, and `~0`/`~1` escape `~`/`/`. Results are native PowerShell values,
including hashtables, arrays, booleans, and `$null`. Missing paths, invalid
syntax, and noncanonical array indices are terminating errors, not null results.
Existing .NET-backed raw results remain unchanged.

## Snapshot results

`Get-BicepSnapshot` returns:

- `PredictedResources`: resources and resolved properties predicted for the deployment
- `Outputs`: resolved deployment outputs
- `Diagnostics`: Bicep compilation warnings and errors

## Live deployment testing

Use a separate live session when a test must inspect real Azure resources or service responses:

```powershell
Connect-AzAccount
$liveSession = New-BicepTestSession -BicepVersion '0.46.1'

try {
    $validation = $liveSession | Test-BicepTestDeployment `
        -Path 'infra/main.bicepparam' `
        -SubscriptionId $env:AZURE_SUBSCRIPTION_ID `
        -ResourceGroup $env:AZURE_RESOURCE_GROUP `
        -StackName "bicep-test-$([guid]::NewGuid().ToString('N'))"

    $validation.IsValid | Should -BeTrue

    $deployment = $liveSession | Start-BicepTestDeployment `
        -Path 'infra/main.bicepparam' `
        -SubscriptionId $env:AZURE_SUBSCRIPTION_ID `
        -ResourceGroup $env:AZURE_RESOURCE_GROUP `
        -StackName "bicep-test-$([guid]::NewGuid().ToString('N'))"

    try {
        $deployment.Resources.Type | Should -Contain 'Microsoft.Storage/storageAccounts'
    }
    finally {
        $deployment | Remove-BicepTestDeployment
    }
}
finally {
    $liveSession | Remove-BicepTestSession
}
```

`New-BicepTestSession` supports both offline snapshots and live tests. It uses the account selected by `Connect-AzAccount` only when validation or deployment needs Azure access. `Test-BicepTestDeployment` returns the underlying `ValidateResult` (including `Error` for service-side failures). `Start-BicepTestDeployment` returns the underlying `DeployResult` (including `Error` for post-submission failures).

Live validation and deployment both support scope-aware parameter sets:

- Resource group scope: `-SubscriptionId -ResourceGroup` with optional `-Location`
- Subscription scope: `-SubscriptionId -Location`
- Management group scope: `-ManagementGroupId -Location`

`-Path` is required in all cases. `-StackName` is optional, and defaults to the .NET generated stack name when omitted. `-ParameterOverrides` accepts a hashtable and is forwarded to deployment parameter values.

Live tests require an Azure PowerShell account with permission to validate, create, and delete Deployment Stacks and their managed resources in the target scope. Authenticate with `Connect-AzAccount` before running them. `Start-BicepTestDeployment` and `Remove-BicepTestDeployment` support `-WhatIf` and `-Confirm`. Removal is idempotent and deletes the stack and all resources it manages.

## More information

The [live samples](../../samples/powershell/BicepTest.Deployment.Sample.Tests.ps1)
include opt-in identical redeployment with cleanup after both operations. Set
the same `-StackName`; an omitted name is generated per operation.
Copy the [consumer authoring skill](../../.github/skills/author-bicep-tests/)
into your repository for AI-generated tests. Automated scenario coverage is not
yet available.

- [Runnable Pester snapshot and deployment samples](https://github.com/anthony-c-martin/bicep-testing/tree/main/samples/powershell)
- [Exported commands](https://github.com/anthony-c-martin/bicep-testing/blob/main/api/powershell/AnthonyCMartin.BicepTesting.txt)
- [Project repository](https://github.com/anthony-c-martin/bicep-testing)