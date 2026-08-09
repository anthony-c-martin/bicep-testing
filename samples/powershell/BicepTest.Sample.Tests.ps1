BeforeAll {
    $modulePath = Join-Path $PSScriptRoot '../../packages/powershell/BicepTest/BicepTest.psd1'
    $parametersPath = Join-Path $PSScriptRoot '../infra/main.bicepparam'
    Import-Module $modulePath -Force

    $session = New-BicepTestSession -BicepVersion '0.43.1'
    $snapshot = $session | Get-BicepSnapshot `
        -Path $parametersPath `
        -TenantId '00000000-0000-0000-0000-000000000000' `
        -SubscriptionId '00000000-0000-0000-0000-000000000000' `
        -ResourceGroup 'sample-rg' `
        -Location 'eastus' `
        -DeploymentName 'sample-deployment'
}

AfterAll {
    $session | Remove-BicepTestSession
}

Describe 'Bicep infrastructure' {
    It 'has the expected resources and no diagnostics' {
        $snapshot.Diagnostics | Should -BeNullOrEmpty
        $snapshot.PredictedResources | Should -HaveCount 3
        $snapshot.PredictedResources.Name | Should -Contain 'sampleprimary'
        $snapshot.PredictedResources.Name | Should -Contain 'samplebackup'
        $snapshot.PredictedResources.Name | Should -Contain 'samplekv'
    }
}