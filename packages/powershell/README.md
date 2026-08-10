# AnthonyCMartin.BicepTesting

Test the resources, outputs, and diagnostics produced by a Bicep deployment without deploying to Azure. The module can also run opt-in integration tests against a real Azure Deployment Stack when an assertion requires live resources.

This is an independent, non-official project.

## Requirements

- PowerShell 7.6 or later
- A `.bicepparam` entry point for the deployment under test

## Installation

```powershell
Install-PSResource AnthonyCMartin.BicepTesting -Version 0.1.4 -Repository PSGallery
Import-Module AnthonyCMartin.BicepTesting -RequiredVersion 0.1.4
```

## Snapshot testing

Create one session for the test suite and remove it when the suite completes:

```powershell
BeforeAll {
    $session = New-BicepTestSession -BicepVersion '0.46.1'
    $snapshot = $session | Get-BicepSnapshot `
        -Path 'infra/main.bicepparam' `
        -TenantId 'ddbe463a-0554-485d-b589-0b17d60cd38b' `
        -SubscriptionId '28c9069e-23e8-47d2-b640-00d2e0f09616' `
        -ResourceGroup 'example-rg' `
        -Location 'eastus' `
        -DeploymentName 'example-deployment'
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

## Snapshot results

`Get-BicepSnapshot` returns:

- `PredictedResources`: resources and resolved properties predicted for the deployment
- `Outputs`: resolved deployment outputs
- `Diagnostics`: Bicep compilation warnings and errors

## Live deployment testing

Use `Start-BicepTestDeployment` only when a test must inspect a real Azure resource or service response:

```powershell
$deployment = $session | Start-BicepTestDeployment `
    -Credential $credential `
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
```

Live tests require an Azure `TokenCredential`, an existing resource group, and permission to create and delete Deployment Stacks and their managed resources. `Start-BicepTestDeployment` and `Remove-BicepTestDeployment` support `-WhatIf` and `-Confirm`. Removal is idempotent and deletes the stack and all resources it manages. Use a unique stack name and keep removal in a `finally` block.

## More information

- [Runnable Pester snapshot and deployment samples](https://github.com/anthony-c-martin/bicep-testing/tree/main/samples/powershell)
- [Exported commands](https://github.com/anthony-c-martin/bicep-testing/blob/main/api/powershell/AnthonyCMartin.BicepTesting.txt)
- [Project repository](https://github.com/anthony-c-martin/bicep-testing)