# PowerShell

The PowerShell module provides commands for testing the predicted resources, outputs, and diagnostics of a Bicep deployment without deploying to Azure.

## Requirements

- PowerShell 7.6 or later
- .NET 10 SDK or later when building from source
- A `.bicepparam` entry point for the Bicep deployment under test

## Build

The module has not yet been published to the PowerShell Gallery. Build its runtime payload from a local checkout:

```powershell
./packages/powershell/build.ps1
Import-Module ./packages/powershell/BicepTest/BicepTest.psd1
```

## Usage

Create a tester, evaluate a snapshot, and dispose the tester when the test completes:

```powershell
$tester = New-BicepTester -BicepVersion '0.43.1'
try {
    $snapshot = $tester | Get-BicepSnapshot `
        -Path 'infra/main.bicepparam' `
        -SubscriptionId '00000000-0000-0000-0000-000000000000' `
        -ResourceGroup 'my-resource-group' `
        -Location 'eastus' `
        -DeploymentName 'my-deployment'

    $storageAccounts = $snapshot.PredictedResources |
        Where-Object Type -eq 'Microsoft.Storage/storageAccounts'
    $storageAccounts.Properties.GetProperty('allowBlobPublicAccess').GetBoolean() |
        Should -BeFalse
}
finally {
    $tester | Remove-BicepTester
}
```

`New-BicepTester` downloads and reuses the requested Bicep CLI version. Snapshot tests do not require Azure credentials or an Azure subscription.

## Live deployment tests

Use `Start-BicepTestDeployment` with an Azure `TokenCredential` when a test needs deployed resources or service behavior:

```powershell
$deployment = $tester | Start-BicepTestDeployment `
    -Credential $credential `
    -Path 'infra/main.bicepparam' `
    -SubscriptionId $subscriptionId `
    -ResourceGroup $resourceGroup `
    -StackName "storage-test-$([guid]::NewGuid().ToString('N'))"

try {
    $deployment.Resources.Type | Should -Contain 'Microsoft.Storage/storageAccounts'
    Invoke-WebRequest $deployment.Outputs['endpoint'].GetString() |
        Select-Object -ExpandProperty StatusCode |
        Should -Be 200
}
finally {
    $deployment | Remove-BicepTestDeployment
}
```

Deployment results expose normalized outputs and managed resource IDs/types. Removal is idempotent and deletes the Deployment Stack and all resources it manages. Live tests require Azure credentials, an existing resource group, and deployment/deletion permissions.

## Sample

See the runnable [Pester sample](../samples/powershell/BicepTest.Sample.Tests.ps1) for a complete consumer test using the shared example infrastructure.

## Public API

The complete exported PowerShell API is available in [`api/powershell/BicepTest.txt`](../api/powershell/BicepTest.txt).