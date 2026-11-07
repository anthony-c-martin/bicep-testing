BeforeAll {
    $root = (Resolve-Path (Join-Path $PSScriptRoot '../../..')).Path
    Import-Module (Join-Path $root 'packages/powershell/AnthonyCMartin.BicepTesting/AnthonyCMartin.BicepTesting.psd1') -Force
    $infra = Join-Path $root 'samples/infra'
    $testdata = Join-Path $root 'packages/testdata'
    $cases = Get-Content (Join-Path $testdata 'snapshot-helpers.json') -Raw | ConvertFrom-Json -AsHashtable
}

Describe 'Snapshot helpers' {
    It 'matches shared resource-scope cases and rejects invalid IDs' {
        foreach ($case in $cases.scopes) {
            $scope = Get-BicepResourceScope -Id $case.id
            $scope.SubscriptionId | Should -Be $case.subscriptionId
            $scope.ResourceGroup | Should -Be $case.resourceGroup
            $scope.ManagementGroupId | Should -Be $case.managementGroupId
        }
        foreach ($id in $cases.invalidIds) {
            { Get-BicepResourceScope -Id $id } | Should -Throw
        }
    }

    It 'resolves native JSON values and distinguishes missing paths from null' {
        foreach ($case in $cases.pointers) {
            Get-BicepSnapshotValue -InputObject $cases.document -Pointer $case.pointer | Should -Be $case.value
        }
        foreach ($pointer in $cases.invalidPointers) {
            { Get-BicepSnapshotValue -InputObject $cases.document -Pointer $pointer } | Should -Throw
        }
        $array = Get-BicepSnapshotValue -InputObject $cases.document -Pointer '/a~1b/~0key'
        $array | Should -HaveCount 3
        $array[0] | Should -BeFalse
        $empty = Get-BicepSnapshotValue -InputObject @{ items = @() } -Pointer '/items'
        $empty.Count | Should -Be 0
        $single = Get-BicepSnapshotValue -InputObject @{ items = @($null) } -Pointer '/items'
        $single.Count | Should -Be 1
        $single[0] | Should -BeNullOrEmpty
        Get-BicepSnapshotValue -InputObject $cases.document -Pointer '' | Should -BeOfType [hashtable]
        Get-BicepSnapshotValue -InputObject ([pscustomobject] @{ value = $false }) -Pointer '/value' | Should -BeFalse
    }

    It 'supports every snapshot scope, optional context, and pipeline helpers' {
        $session = New-BicepTestSession -BicepVersion '0.47.16'
        try {
            $snapshot = $session | Get-BicepSnapshot -Path (Join-Path $infra 'snapshot-scopes/management-group.bicepparam') `
                -ManagementGroupId 'platform' -Location 'eastus' -DeploymentName 'offline-test'
            $snapshot.Diagnostics | Should -BeNullOrEmpty
            $snapshot | Get-BicepSnapshotValue -Pointer '/outputs/scopeId' |
                Should -Be '/providers/Microsoft.Management/managementGroups/platform'
            $snapshot | Get-BicepSnapshotValue -Pointer '/outputs/deploymentName' | Should -Be 'offline-test'
            @($snapshot | Get-BicepSnapshotResource -Type 'microsoft.authorization/POLICYDEFINITIONS') | Should -HaveCount 1
            $policy = $snapshot | Get-BicepSnapshotResource -Type 'Microsoft.Authorization/policyDefinitions' -Name 'require-cost-center'
            $snapshot | Get-BicepSnapshotResource -Id $policy.Id.ToUpperInvariant() | Should -Be $policy
            ($policy | Get-BicepResourceScope).ManagementGroupId | Should -Be 'platform'
            $policy | Get-BicepSnapshotValue -Pointer '/properties/policyRule/then/effect' | Should -Be 'audit'
            { $snapshot | Get-BicepSnapshotResource -Id '/missing' } | Should -Throw '*found 0*'
            $duplicate = [AnthonyCMartin.BicepTesting.SnapshotResult] @{ PredictedResources = @($policy, $policy) }
            { $duplicate | Get-BicepSnapshotResource -Id $policy.Id } | Should -Throw '*found 2*'
            { $duplicate | Get-BicepSnapshotResource -Type $policy.Type -Name $policy.Name } | Should -Throw '*found 2*'
            { $duplicate | Get-BicepSnapshotResource -Type $policy.Type -Name $policy.Name.ToUpperInvariant() } | Should -Throw '*found 0*'
            $subscription = $session | Get-BicepSnapshot -Path (Join-Path $testdata 'snapshot-scopes/subscription.bicepparam') `
                -SubscriptionId '28c9069e-23e8-47d2-b640-00d2e0f09616'
            $subscription.Diagnostics | Should -BeNullOrEmpty
            $subscription | Get-BicepSnapshotValue -Pointer '/outputs/scopeId' |
                Should -Be '/subscriptions/28c9069e-23e8-47d2-b640-00d2e0f09616'
            $tenant = $session | Get-BicepSnapshot -Path (Join-Path $testdata 'snapshot-scopes/tenant.bicepparam') `
                -TenantId 'ddbe463a-0554-485d-b589-0b17d60cd38b'
            $tenant.Diagnostics | Should -BeNullOrEmpty
            $tenant | Get-BicepSnapshotValue -Pointer '/outputs/tenantId' | Should -Be 'ddbe463a-0554-485d-b589-0b17d60cd38b'
            ($session | Get-BicepSnapshot -Path (Join-Path $infra 'security-baseline/secure.bicepparam')).Diagnostics | Should -BeNullOrEmpty
        }
        finally { $session | Remove-BicepTestSession }
    }

    It 'rejects management-group context with an older CLI' {
        $session = New-BicepTestSession -BicepVersion '0.46.1'
        try {
            { $session | Get-BicepSnapshot -Path (Join-Path $infra 'snapshot-scopes/management-group.bicepparam') `
                -ManagementGroupId 'platform' } | Should -Throw '*0.47.16*'
            { $session | Get-BicepSnapshot -Path 'unused' -ManagementGroupId ' ' } | Should -Throw
        }
        finally { $session | Remove-BicepTestSession }
    }
}
