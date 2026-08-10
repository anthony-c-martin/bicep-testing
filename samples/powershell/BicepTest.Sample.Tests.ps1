BeforeAll {
    $infraPath = Join-Path $PSScriptRoot '../infra'
    Import-Module AnthonyCMartin.BicepTesting -RequiredVersion 0.1.5 -Force
    $session = New-BicepTestSession -BicepVersion '0.46.1'

    function Get-SampleSnapshot([string] $RelativePath) {
        $session | Get-BicepSnapshot `
            -Path (Join-Path $infraPath $RelativePath) `
            -TenantId 'ddbe463a-0554-485d-b589-0b17d60cd38b' `
            -SubscriptionId '28c9069e-23e8-47d2-b640-00d2e0f09616' `
            -ResourceGroup 'sample-rg' `
            -Location 'eastus' `
            -DeploymentName 'sample-deployment'
    }
}

AfterAll {
    $session | Remove-BicepTestSession
}

Describe 'Real-world Bicep snapshots' {
    It 'selects environment topology, SKUs, and tags' {
        $development = Get-SampleSnapshot 'environment-topology/dev.bicepparam'
        $production = Get-SampleSnapshot 'environment-topology/prod.bicepparam'

        $development.Diagnostics | Should -BeNullOrEmpty
        $development.PredictedResources | Should -HaveCount 1
        $development.PredictedResources[0].Name | Should -Be 'ordersdevprimary'
        $development.PredictedResources[0].AdditionalProperties['sku'].GetProperty('name').GetString() | Should -Be 'Standard_LRS'
        $development.PredictedResources[0].AdditionalProperties['tags'].GetProperty('environment').GetString() | Should -Be 'dev'
        $development.Outputs.auditStorageId | Should -BeNullOrEmpty
        $production.PredictedResources.Name | Should -Be @('ordersprodprimary', 'ordersprodaudit')
        $production.PredictedResources[0].AdditionalProperties['sku'].GetProperty('name').GetString() | Should -Be 'Standard_ZRS'
        $production.PredictedResources[0].AdditionalProperties['tags'].GetProperty('dataClassification').GetString() | Should -Be 'confidential'
        $production.PredictedResources[1].AdditionalProperties['sku'].GetProperty('name').GetString() | Should -Be 'Standard_GRS'
    }

    It 'catches a deliberately weakened security baseline' {
        $secure = Get-SampleSnapshot 'security-baseline/secure.bicepparam'
        $insecure = Get-SampleSnapshot 'security-baseline/insecure.bicepparam'
        $secureStorage = $secure.PredictedResources | Where-Object Type -eq 'Microsoft.Storage/storageAccounts'
        $secureVault = $secure.PredictedResources | Where-Object Type -eq 'Microsoft.KeyVault/vaults'
        $insecureStorage = $insecure.PredictedResources | Where-Object Type -eq 'Microsoft.Storage/storageAccounts'

        $secureStorage.Properties.GetProperty('allowBlobPublicAccess').GetBoolean() | Should -BeFalse
        $secureStorage.Properties.GetProperty('allowSharedKeyAccess').GetBoolean() | Should -BeFalse
        $secureStorage.Properties.GetProperty('minimumTlsVersion').GetString() | Should -Be 'TLS1_2'
        $secureStorage.Properties.GetProperty('publicNetworkAccess').GetString() | Should -Be 'Disabled'
        $secureVault.Properties.GetProperty('enablePurgeProtection').GetBoolean() | Should -BeTrue
        $secureVault.Properties.GetProperty('enableRbacAuthorization').GetBoolean() | Should -BeTrue
        $secureVault.Properties.GetProperty('softDeleteRetentionInDays').GetInt32() | Should -Be 90
        $insecureStorage.Properties.GetProperty('minimumTlsVersion').GetString() | Should -Be 'TLS1_0'
        $insecureStorage.Properties.GetProperty('allowBlobPublicAccess').GetBoolean() | Should -BeTrue
    }

    It 'wires private endpoint, subnet, NSG, and DNS references together' {
        $snapshot = Get-SampleSnapshot 'private-network/main.bicepparam'
        $resources = @{}
        $snapshot.PredictedResources | ForEach-Object { $resources[$_.Name] = $_ }

        $networkIds = $snapshot.Outputs['networkIds']
        $resources['orders-vnet'].Properties.GetProperty('addressSpace').GetProperty('addressPrefixes')[0].GetString() | Should -Be '10.42.0.0/16'
        $resources['orders-vnet/app'].Properties.GetProperty('addressPrefix').GetString() | Should -Be '10.42.1.0/24'
        $resources['orders-vnet/app'].Properties.GetProperty('networkSecurityGroup').GetProperty('id').GetString() | Should -Match '/orders-app-nsg$'
        $resources['orders-vnet/data'].Properties.GetProperty('privateEndpointNetworkPolicies').GetString() | Should -Be 'Disabled'
        $resources['orders-storage-pe'].Properties.GetProperty('subnet').GetProperty('id').GetString() |
            Should -Be $networkIds.GetProperty('dataSubnetId').GetString()
        $connection = $resources['orders-storage-pe'].Properties.GetProperty('privateLinkServiceConnections')[0].GetProperty('properties')
        $connection.GetProperty('groupIds')[0].GetString() | Should -Be 'blob'
        $connection.GetProperty('privateLinkServiceId').GetString() | Should -Match '/storageAccounts/ordersprivatestore$'
        $resources['privatelink.blob.core.windows.net/orders-vnet-link'].Properties.GetProperty('virtualNetwork').GetProperty('id').GetString() |
            Should -Be $networkIds.GetProperty('virtualNetworkId').GetString()
    }
}
