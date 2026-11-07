BeforeAll {
    $infraPath = Join-Path $PSScriptRoot '../infra'
    Import-Module AnthonyCMartin.BicepTesting -RequiredVersion 0.1.7 -Force
    $session = New-BicepTestSession -BicepVersion '0.47.16'

    function Get-ResourceGroupSnapshot([string] $RelativePath) {
        $session | Get-BicepSnapshot `
            -Path (Join-Path $infraPath $RelativePath) `
            -SubscriptionId '28c9069e-23e8-47d2-b640-00d2e0f09616' `
            -ResourceGroup 'sample-rg' `
            -Location 'eastus'
    }
}

AfterAll {
    if ($session) { $session | Remove-BicepTestSession }
}

Describe 'Testing the orders infrastructure without deploying it' {
    It 'keeps storage and Key Vault compliant with the security baseline' {
        # Context is unnecessary when the assertions only depend on explicit parameter values.
        $snapshot = $session | Get-BicepSnapshot -Path (Join-Path $infraPath 'security-baseline/secure.bicepparam')
        $snapshot.Diagnostics | Should -BeNullOrEmpty
        $storage = $snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Storage/storageAccounts' -Name 'orderssecurestore' |
            Get-BicepSnapshotValue -Pointer ''
        $vault = $snapshot | Get-BicepSnapshotResource -Type 'Microsoft.KeyVault/vaults' -Name 'orders-secure-kv' |
            Get-BicepSnapshotValue -Pointer ''

        $storage.properties.allowBlobPublicAccess | Should -BeFalse
        $storage.properties.allowSharedKeyAccess | Should -BeFalse
        $storage.properties.minimumTlsVersion | Should -Be 'TLS1_2'
        $storage.properties.publicNetworkAccess | Should -Be 'Disabled'
        $storage.properties.supportsHttpsTrafficOnly | Should -BeTrue
        $vault.properties.enablePurgeProtection | Should -BeTrue
        $vault.properties.enableRbacAuthorization | Should -BeTrue
        $vault.properties.publicNetworkAccess | Should -Be 'Disabled'
        $vault.properties.softDeleteRetentionInDays | Should -Be 90
    }

    It 'uses inexpensive development storage without an audit account' {
        $snapshot = Get-ResourceGroupSnapshot 'environment-topology/dev.bicepparam'
        $snapshot.Diagnostics | Should -BeNullOrEmpty
        @($snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Storage/storageAccounts') | Should -HaveCount 1
        $primary = $snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Storage/storageAccounts' -Name 'ordersdevprimary' |
            Get-BicepSnapshotValue -Pointer ''

        $primary.sku.name | Should -Be 'Standard_LRS'
        $primary.tags.environment | Should -Be 'dev'
        $primary.tags.dataClassification | Should -Be 'internal'
        $snapshot | Get-BicepSnapshotValue -Pointer '/outputs/primaryStorageId' | Should -Be $primary.id
        $snapshot | Get-BicepSnapshotValue -Pointer '/outputs/auditStorageId' | Should -BeNullOrEmpty
    }

    It 'uses resilient primary and audit storage in production' {
        $snapshot = Get-ResourceGroupSnapshot 'environment-topology/prod.bicepparam'
        $snapshot.Diagnostics | Should -BeNullOrEmpty
        @($snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Storage/storageAccounts') | Should -HaveCount 2
        $primary = $snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Storage/storageAccounts' -Name 'ordersprodprimary' |
            Get-BicepSnapshotValue -Pointer ''
        $audit = $snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Storage/storageAccounts' -Name 'ordersprodaudit' |
            Get-BicepSnapshotValue -Pointer ''

        $primary.sku.name | Should -Be 'Standard_ZRS'
        $audit.sku.name | Should -Be 'Standard_GRS'
        $primary.tags.environment | Should -Be 'prod'
        $primary.tags.dataClassification | Should -Be 'confidential'
        $snapshot | Get-BicepSnapshotValue -Pointer '/outputs/primaryStorageId' | Should -Be $primary.id
        $snapshot | Get-BicepSnapshotValue -Pointer '/outputs/auditStorageId' | Should -Be $audit.id
    }

    It 'connects private storage to the data subnet and private DNS' {
        $snapshot = Get-ResourceGroupSnapshot 'private-network/main.bicepparam'
        $snapshot.Diagnostics | Should -BeNullOrEmpty
        $networkIds = $snapshot | Get-BicepSnapshotValue -Pointer '/outputs/networkIds'
        $network = $snapshot | Get-BicepSnapshotResource -Id $networkIds.virtualNetworkId | Get-BicepSnapshotValue -Pointer ''
        $appSubnet = $snapshot | Get-BicepSnapshotResource -Id $networkIds.appSubnetId | Get-BicepSnapshotValue -Pointer ''
        $dataSubnet = $snapshot | Get-BicepSnapshotResource -Id $networkIds.dataSubnetId | Get-BicepSnapshotValue -Pointer ''
        $endpoint = $snapshot | Get-BicepSnapshotResource -Id $networkIds.privateEndpointId | Get-BicepSnapshotValue -Pointer ''
        $storage = $snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Storage/storageAccounts' -Name 'ordersprivatestore' |
            Get-BicepSnapshotValue -Pointer ''
        $appNsg = $snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Network/networkSecurityGroups' -Name 'orders-app-nsg' |
            Get-BicepSnapshotValue -Pointer ''
        $dataNsg = $snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Network/networkSecurityGroups' -Name 'orders-data-nsg' |
            Get-BicepSnapshotValue -Pointer ''
        $dnsLink = $snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Network/privateDnsZones/virtualNetworkLinks' `
            -Name 'privatelink.blob.core.windows.net/orders-vnet-link' | Get-BicepSnapshotValue -Pointer ''

        $network.properties.addressSpace.addressPrefixes | Should -Be @('10.42.0.0/16')
        $appSubnet.properties.addressPrefix | Should -Be '10.42.1.0/24'
        $appSubnet.properties.networkSecurityGroup.id | Should -Be $appNsg.id
        $dataSubnet.properties.addressPrefix | Should -Be '10.42.2.0/24'
        $dataSubnet.properties.networkSecurityGroup.id | Should -Be $dataNsg.id
        $dataSubnet.properties.privateEndpointNetworkPolicies | Should -Be 'Disabled'
        $endpoint.properties.subnet.id | Should -Be $dataSubnet.id
        $connection = $endpoint.properties.privateLinkServiceConnections[0].properties
        $connection.groupIds | Should -Be @('blob')
        $connection.privateLinkServiceId | Should -Be $storage.id
        $dnsLink.properties.virtualNetwork.id | Should -Be $network.id
    }

    It 'audits resources missing a cost-center tag at the platform management group' {
        $snapshot = $session | Get-BicepSnapshot -Path (Join-Path $infraPath 'snapshot-scopes/management-group.bicepparam') `
            -ManagementGroupId 'sample-platform' -Location 'eastus'
        $snapshot.Diagnostics | Should -BeNullOrEmpty
        $policy = $snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Authorization/policyDefinitions' -Name 'require-cost-center' |
            Get-BicepSnapshotValue -Pointer ''

        (Get-BicepResourceScope -Id $policy.id).ManagementGroupId | Should -Be 'sample-platform'
        $policy.properties.policyRule.if.field | Should -Be 'tags[costCenter]'
        $policy.properties.policyRule.if.exists | Should -Be 'false'
        $policy.properties.policyRule.then.effect | Should -Be 'audit'
        $snapshot | Get-BicepSnapshotValue -Pointer '/outputs/policyId' | Should -Be $policy.id
    }
}
