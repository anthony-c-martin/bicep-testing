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

    $libraryPath = Join-Path $repositoryRoot 'packages/powershell/AnthonyCMartin.BicepTesting/lib/net10.0'
    $azureCorePath = Join-Path $libraryPath 'Azure.Core.dll'
    $systemClientModelPath = Join-Path $libraryPath 'System.ClientModel.dll'
    Add-Type -Path $azureCorePath
    Add-Type -Path $systemClientModelPath
    if (-not ('PesterTokenCredential' -as [type])) {
        Add-Type -TypeDefinition @'
using System;
using System.Threading;
using System.Threading.Tasks;
using Azure.Core;

public sealed class PesterTokenCredential : TokenCredential
{
    public override AccessToken GetToken(TokenRequestContext requestContext, CancellationToken cancellationToken)
        => new AccessToken("pester-token", DateTimeOffset.UtcNow.AddMinutes(5));

    public override ValueTask<AccessToken> GetTokenAsync(TokenRequestContext requestContext, CancellationToken cancellationToken)
        => new ValueTask<AccessToken>(GetToken(requestContext, cancellationToken));
}
'@ -ReferencedAssemblies @($azureCorePath, $systemClientModelPath)
    }
}

Describe 'AnthonyCMartin.BicepTesting module' {
    It 'exports only the supported commands' {
        (Get-Command -Module AnthonyCMartin.BicepTesting).Name | Should -Be @(
            'Get-BicepSnapshot'
            'New-BicepLiveTestSession'
            'New-BicepTestSession'
            'Remove-BicepTestDeployment'
            'Remove-BicepTestSession'
            'Start-BicepTestDeployment'
            'Test-BicepTestDeployment'
        )
    }

    It 'creates and disposes a live test session' {
        $session = New-BicepLiveTestSession `
            -BicepVersion '0.46.1' `
            -Credential ([PesterTokenCredential]::new())

        try {
            $session.GetType().FullName | Should -Be 'AnthonyCMartin.BicepTesting.LiveBicepTestSession'
        }
        finally {
            $session | Remove-BicepTestSession
        }
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
            param($Options, $CancellationToken)
            $this.DeployCalls++
            [Threading.Tasks.Task]::FromResult([object] [pscustomobject] @{})
        }

        $result = $session | Start-BicepTestDeployment `
            -Path (Join-Path $repositoryRoot 'samples/infra/main.bicepparam') `
            -SubscriptionId $subscriptionId `
            -ResourceGroup $resourceGroup `
            -StackName 'test-stack' `
            -WhatIf

        $result | Should -BeNullOrEmpty
        $session.DeployCalls | Should -Be 0
    }

    It 'does not expose a credential parameter on Start-BicepTestDeployment' {
        (Get-Command Start-BicepTestDeployment).Parameters.ContainsKey('Credential') | Should -BeFalse
    }

    It 'translates deployment options and parameter overrides for each scope' {
        $deployment = [pscustomobject] @{}
        $session = [pscustomobject] @{ DeployCalls = 0; Options = $null }
        $session | Add-Member -MemberType ScriptMethod -Name DeployAsync -Value {
            param($Options, $CancellationToken)
            $this.DeployCalls++
            $this.Options = $Options
            [Threading.Tasks.Task]::FromResult([object] $deployment)
        }.GetNewClosure()

        $resourceGroupResult = $session | Start-BicepTestDeployment `
            -Path (Join-Path $repositoryRoot 'samples/infra/main.bicepparam') `
            -SubscriptionId $subscriptionId `
            -ResourceGroup $resourceGroup `
            -StackName 'test-rg-stack' `
            -ParameterOverrides @{ environment = 'test' }

        $resourceGroupResult | Should -Be $deployment
        $session.Options.SubscriptionId | Should -Be $subscriptionId
        $session.Options.ResourceGroup | Should -Be $resourceGroup
        $session.Options.Location | Should -BeNullOrEmpty
        $session.Options.StackName | Should -Be 'test-rg-stack'
        $session.Options.ParameterOverrides['environment'].GetString() | Should -Be 'test'

        $subscriptionResult = $session | Start-BicepTestDeployment `
            -Path (Join-Path $repositoryRoot 'samples/infra/main.bicepparam') `
            -SubscriptionId $subscriptionId `
            -Location $location `
            -StackName 'test-sub-stack'

        $subscriptionResult | Should -Be $deployment
        $session.Options.SubscriptionId | Should -Be $subscriptionId
        $session.Options.ResourceGroup | Should -BeNullOrEmpty
        $session.Options.Location | Should -Be $location
        $session.Options.StackName | Should -Be 'test-sub-stack'

        $managementGroupResult = $session | Start-BicepTestDeployment `
            -Path (Join-Path $repositoryRoot 'samples/infra/main.bicepparam') `
            -ManagementGroupId 'contoso-platform' `
            -Location $location

        $managementGroupResult | Should -Be $deployment
        $session.Options.SubscriptionId | Should -BeNullOrEmpty
        $session.Options.ManagementGroupId | Should -Be 'contoso-platform'
        $session.Options.ResourceGroup | Should -BeNullOrEmpty
        $session.Options.Location | Should -Be $location
        $session.Options.StackName | Should -Match '^bicep-test-'

        $session.DeployCalls | Should -Be 3
    }

    It 'invokes validation and translates options for each scope' {
        $validResult = [pscustomobject] @{
            IsValid = $true
            Error = $null
            CorrelationId = 'test-correlation-id'
        }
        $session = [pscustomobject] @{ ValidateCalls = 0; Options = $null }
        $session | Add-Member -MemberType ScriptMethod -Name ValidateAsync -Value {
            param($Options, $CancellationToken)
            $this.ValidateCalls++
            $this.Options = $Options
            [Threading.Tasks.Task]::FromResult([object] $validResult)
        }.GetNewClosure()

        $resourceGroupResult = $session | Test-BicepTestDeployment `
            -Path (Join-Path $repositoryRoot 'samples/infra/main.bicepparam') `
            -SubscriptionId $subscriptionId `
            -ResourceGroup $resourceGroup `
            -StackName 'test-rg-stack' `
            -ParameterOverrides @{ includeAuditStorage = $false }

        $resourceGroupResult | Should -Be $validResult
        $session.Options.SubscriptionId | Should -Be $subscriptionId
        $session.Options.ResourceGroup | Should -Be $resourceGroup
        $session.Options.ParameterOverrides['includeAuditStorage'].GetBoolean() | Should -BeFalse

        $subscriptionResult = $session | Test-BicepTestDeployment `
            -Path (Join-Path $repositoryRoot 'samples/infra/main.bicepparam') `
            -SubscriptionId $subscriptionId `
            -Location $location

        $subscriptionResult | Should -Be $validResult
        $session.Options.SubscriptionId | Should -Be $subscriptionId
        $session.Options.ResourceGroup | Should -BeNullOrEmpty
        $session.Options.Location | Should -Be $location

        $managementGroupResult = $session | Test-BicepTestDeployment `
            -Path (Join-Path $repositoryRoot 'samples/infra/main.bicepparam') `
            -ManagementGroupId 'contoso-platform' `
            -Location $location

        $managementGroupResult | Should -Be $validResult
        $session.Options.SubscriptionId | Should -BeNullOrEmpty
        $session.Options.ManagementGroupId | Should -Be 'contoso-platform'
        $session.Options.Location | Should -Be $location
        $session.ValidateCalls | Should -Be 3
    }

    It 'disposes a session through Remove-BicepTestSession' {
        $session = [pscustomobject] @{ DisposeCalls = 0 }
        $session | Add-Member -MemberType ScriptMethod -Name Dispose -Value {
            $this.DisposeCalls++
        }

        $session | Remove-BicepTestSession

        $session.DisposeCalls | Should -Be 1
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
