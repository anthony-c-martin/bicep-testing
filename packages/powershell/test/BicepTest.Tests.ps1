BeforeAll {
    $repositoryRoot = Resolve-Path (Join-Path $PSScriptRoot '../../..')
    $modulePath = Join-Path $repositoryRoot 'packages/powershell/BicepTest/BicepTest.psd1'
    Import-Module $modulePath -Force

    $fixturePath = Join-Path $repositoryRoot 'packages/node/test/samples/snapshot/main.bicepparam'
    $tenantId = '00000000-0000-0000-0000-000000000000'
    $subscriptionId = '00000000-0000-0000-0000-000000000000'
    $resourceGroup = 'test-rg'
    $location = 'eastus'
    $deploymentName = 'test-deployment'
}

Describe 'BicepTest module' {
    It 'exports only the supported commands' {
        (Get-Command -Module BicepTest).Name | Should -Be @(
            'Get-BicepSnapshot'
            'New-BicepTestSession'
            'Remove-BicepTestDeployment'
            'Remove-BicepTestSession'
            'Start-BicepTestDeployment'
        )
    }

    It 'matches the reference snapshot behavior' {
        $session = New-BicepTestSession -BicepVersion '0.43.1'
        try {
            $snapshot = $session | Get-BicepSnapshot `
                -Path $fixturePath `
                -TenantId $tenantId `
                -SubscriptionId $subscriptionId `
                -ResourceGroup $resourceGroup `
                -Location $location `
                -DeploymentName $deploymentName

            $snapshot.Diagnostics | Should -BeNullOrEmpty

            $storageAccounts = @($snapshot.PredictedResources | Where-Object Type -eq 'Microsoft.Storage/storageAccounts')
            $keyVaults = @($snapshot.PredictedResources | Where-Object Type -eq 'Microsoft.KeyVault/vaults')
            $virtualNetworks = @($snapshot.PredictedResources | Where-Object Type -eq 'Microsoft.Network/virtualNetworks')

            $storageAccounts | Should -HaveCount 2
            $keyVaults | Should -HaveCount 1
            $virtualNetworks | Should -HaveCount 0
            $storageAccounts.Name | Should -Contain 'testprimary'
            $storageAccounts.Name | Should -Contain 'testbackup'

            foreach ($resource in $storageAccounts) {
                $resource.Properties.GetProperty('allowBlobPublicAccess').GetBoolean() | Should -BeFalse
                $resource.Properties.GetProperty('minimumTlsVersion').GetString() | Should -Be 'TLS1_2'
                $resource.Location | Should -Be $location
            }

            $primaryStorageId = $snapshot.Outputs['primaryStorageId'].GetString()
            $primaryStorageId | Should -Be "/subscriptions/$subscriptionId/resourceGroups/$resourceGroup/providers/Microsoft.Storage/storageAccounts/testprimary"
        }
        finally {
            $session | Remove-BicepTestSession
        }
    }
}