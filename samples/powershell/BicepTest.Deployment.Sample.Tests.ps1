BeforeAll {
    $modulePath = Join-Path $PSScriptRoot '../../packages/powershell/AnthonyCMartin.BicepTesting/AnthonyCMartin.BicepTesting.psd1'
    $parametersPath = Join-Path $PSScriptRoot '../infra/main.bicepparam'
    Import-Module $modulePath -Force
}

Describe 'Bicep infrastructure deployment' -Skip:(
    -not $env:AZURE_SUBSCRIPTION_ID -or
    -not $env:AZURE_RESOURCE_GROUP -or
    -not $env:BICEP_TEST_STACK_NAME -or
    -not $env:BICEP_TEST_RESOURCE_PREFIX) {
    It 'deploys resources and removes them afterward' {
        $session = New-BicepTestSession -BicepVersion '0.43.1'
        try {
            $credential = [Azure.Identity.DefaultAzureCredential]::new()
            $deployment = $session | Start-BicepTestDeployment `
                -Credential $credential `
                -Path $parametersPath `
                -SubscriptionId $env:AZURE_SUBSCRIPTION_ID `
                -ResourceGroup $env:AZURE_RESOURCE_GROUP `
                -StackName $env:BICEP_TEST_STACK_NAME `
                -ParameterOverrides @{ env = $env:BICEP_TEST_RESOURCE_PREFIX }

            $deployment.Resources.Type | Should -Contain 'Microsoft.Storage/storageAccounts'
            $deployment.Outputs['primaryStorageId'].GetString() |
                Should -Match '/providers/Microsoft.Storage/storageAccounts/'
        }
        finally {
            if ($deployment) {
                $deployment | Remove-BicepTestDeployment
            }
            $session | Remove-BicepTestSession
        }
    }
}