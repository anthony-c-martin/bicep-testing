BeforeAll {
    $repositoryRoot = Resolve-Path (Join-Path $PSScriptRoot '../../..')
    $modulePath = Join-Path $repositoryRoot 'packages/powershell/AnthonyCMartin.BicepTesting/AnthonyCMartin.BicepTesting.psd1'
    Import-Module $modulePath -Force

    $fixturePath = Join-Path $repositoryRoot 'packages/node/test/samples/snapshot/main.bicepparam'
    $tenantId = 'ddbe463a-0554-485d-b589-0b17d60cd38b'
    $subscriptionId = '28c9069e-23e8-47d2-b640-00d2e0f09616'
    $resourceGroup = 'test-rg'
    $location = 'eastus'
    $deploymentName = 'test-deployment'
}

Describe 'AnthonyCMartin.BicepTesting module' {
    It 'exports only the supported commands' {
        (Get-Command -Module AnthonyCMartin.BicepTesting).Name | Should -Be @(
            'Get-BicepSnapshot'
            'New-BicepTestSession'
            'Remove-BicepTestDeployment'
            'Remove-BicepTestSession'
            'Start-BicepTestDeployment'
        )
    }

    It 'matches the reference snapshot behavior' {
        $session = New-BicepTestSession -BicepVersion '0.46.1'
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

    It 'honors WhatIf before starting a deployment' {
        $session = [pscustomobject] @{ DeployCalls = 0 }
        $session | Add-Member -MemberType ScriptMethod -Name DeployAsync -Value {
            $this.DeployCalls++
            [Threading.Tasks.Task]::FromResult([object] [pscustomobject] @{})
        }

        $result = $session | Start-BicepTestDeployment `
            -Credential ([pscustomobject] @{}) `
            -Path (Join-Path $repositoryRoot 'samples/infra/main.bicepparam') `
            -SubscriptionId $subscriptionId `
            -ResourceGroup $resourceGroup `
            -StackName 'test-stack' `
            -WhatIf

        $result | Should -BeNullOrEmpty
        $session.DeployCalls | Should -Be 0
    }

    It 'translates deployment options and parameter overrides' {
        $deployment = [pscustomobject] @{}
        $session = [pscustomobject] @{ DeployCalls = 0; Options = $null }
        $session | Add-Member -MemberType ScriptMethod -Name DeployAsync -Value {
            param($Credential, $Options, $CancellationToken)
            $this.DeployCalls++
            $this.Options = $Options
            [Threading.Tasks.Task]::FromResult([object] $deployment)
        }.GetNewClosure()

        $result = $session | Start-BicepTestDeployment `
            -Credential ([pscustomobject] @{}) `
            -Path (Join-Path $repositoryRoot 'samples/infra/main.bicepparam') `
            -SubscriptionId $subscriptionId `
            -ResourceGroup $resourceGroup `
            -StackName 'test-stack' `
            -ParameterOverrides @{ environment = 'test' }

        $result | Should -Be $deployment
        $session.DeployCalls | Should -Be 1
        $session.Options.SubscriptionId | Should -Be $subscriptionId
        $session.Options.ResourceGroup | Should -Be $resourceGroup
        $session.Options.StackName | Should -Be 'test-stack'
        $session.Options.ParameterOverrides['environment'].GetString() | Should -Be 'test'
    }

    It 'honors WhatIf before removing a deployment' {
        $deployment = [pscustomobject] @{ TeardownCalls = 0 }
        $deployment | Add-Member -MemberType ScriptMethod -Name TeardownAsync -Value {
            $this.TeardownCalls++
            [Threading.Tasks.Task]::CompletedTask
        }

        $deployment | Remove-BicepTestDeployment -WhatIf

        $deployment.TeardownCalls | Should -Be 0
    }

    It 'removes a deployment through its teardown operation' {
        $deployment = [pscustomobject] @{ TeardownCalls = 0 }
        $deployment | Add-Member -MemberType ScriptMethod -Name TeardownAsync -Value {
            $this.TeardownCalls++
            [Threading.Tasks.Task]::CompletedTask
        }

        $deployment | Remove-BicepTestDeployment

        $deployment.TeardownCalls | Should -Be 1
    }
}
