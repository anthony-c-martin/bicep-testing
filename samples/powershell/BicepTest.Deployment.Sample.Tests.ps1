BeforeAll {
    $parametersPath = Join-Path $PSScriptRoot '../infra/live-storage/main.bicepparam'
    Import-Module AnthonyCMartin.BicepTesting -RequiredVersion 0.1.7 -Force

    function Get-AzureResourceResponse([string] $ResourceId) {
        Invoke-AzRestMethod -Path "${ResourceId}?api-version=2023-05-01" -Method GET
    }

    function Get-SampleValidation(
        $Session,
        [string] $StackName,
        [bool] $IncludeAuditStorage
    ) {
        $Session | Test-BicepTestDeployment `
            -Path $parametersPath `
            -SubscriptionId $env:AZURE_SUBSCRIPTION_ID `
            -ResourceGroup $env:AZURE_RESOURCE_GROUP `
            -StackName $StackName `
            -ParameterOverrides @{
                resourcePrefix = $env:BICEP_TEST_RESOURCE_PREFIX
                includeAuditStorage = $IncludeAuditStorage
            }
    }

    function Start-SampleDeployment(
        $Session,
        [string] $StackName,
        [bool] $IncludeAuditStorage
    ) {
        $Session | Start-BicepTestDeployment `
            -Path $parametersPath `
            -SubscriptionId $env:AZURE_SUBSCRIPTION_ID `
            -ResourceGroup $env:AZURE_RESOURCE_GROUP `
            -StackName $StackName `
            -ParameterOverrides @{
                resourcePrefix = $env:BICEP_TEST_RESOURCE_PREFIX
                includeAuditStorage = $IncludeAuditStorage
            }
    }
}

Describe 'Real-world Bicep deployments' -Skip:(
    -not $env:AZURE_SUBSCRIPTION_ID -or
    -not $env:AZURE_RESOURCE_GROUP -or
    -not $env:BICEP_TEST_STACK_NAME -or
    -not $env:BICEP_TEST_RESOURCE_PREFIX) {
    BeforeAll {
        $session = New-BicepTestSession -BicepVersion '0.46.1'
    }

    AfterAll {
        $session | Remove-BicepTestSession
    }

    It 'redeploys identical inputs without changing resource identity or security' {
        $deployment = $null
        try {
            $deployment = Start-SampleDeployment -Session $session `
                -StackName "$env:BICEP_TEST_STACK_NAME-idempotency" -IncludeAuditStorage $false
            $deployment.Succeeded | Should -BeTrue
            $primaryStorageId = $deployment.Outputs['primaryStorageId'].GetString()

            $deployment = Start-SampleDeployment -Session $session `
                -StackName "$env:BICEP_TEST_STACK_NAME-idempotency" -IncludeAuditStorage $false
            $deployment.Succeeded | Should -BeTrue
            $deployment.Outputs['primaryStorageId'].GetString() | Should -Be $primaryStorageId
            $response = Get-AzureResourceResponse $primaryStorageId
            $response.StatusCode | Should -Be 200
            $storage = $response.Content | ConvertFrom-Json
            $storage.properties.allowBlobPublicAccess | Should -BeFalse
            $storage.properties.minimumTlsVersion | Should -Be 'TLS1_2'
            $storage.properties.publicNetworkAccess | Should -Be 'Disabled'
        }
        finally {
            if ($deployment) { $deployment | Remove-BicepTestDeployment }
        }
    }

    It 'keeps deployed storage compliant with the security baseline in Azure' {
        $deployment = $null
        try {
            $validation = Get-SampleValidation `
                -Session $session `
                -StackName "$env:BICEP_TEST_STACK_NAME-secure" `
                -IncludeAuditStorage $false
            $validation.IsValid | Should -BeTrue

            $deployment = Start-SampleDeployment `
                -Session $session `
                -StackName "$env:BICEP_TEST_STACK_NAME-secure" `
                -IncludeAuditStorage $false
            $deployment.Succeeded | Should -BeTrue
            $primaryStorageId = $deployment.Outputs['primaryStorageId'].GetString()
            $response = Get-AzureResourceResponse $primaryStorageId
            $storage = $response.Content | ConvertFrom-Json

            $response.StatusCode | Should -Be 200
            $storage.properties.allowBlobPublicAccess | Should -BeFalse
            $storage.properties.allowSharedKeyAccess | Should -BeFalse
            $storage.properties.minimumTlsVersion | Should -Be 'TLS1_2'
            $storage.properties.publicNetworkAccess | Should -Be 'Disabled'
            $storage.properties.supportsHttpsTrafficOnly | Should -BeTrue
        }
        finally {
            if ($deployment) {
                $deployment | Remove-BicepTestDeployment
            }
        }

    }

    It 'removes the audit account from Azure when audit storage is disabled' {
        $deployment = $null
        try {
            $validation = Get-SampleValidation `
                -Session $session `
                -StackName $env:BICEP_TEST_STACK_NAME `
                -IncludeAuditStorage $true
            $validation.IsValid | Should -BeTrue

            $deployment = Start-SampleDeployment `
                -Session $session `
                -StackName $env:BICEP_TEST_STACK_NAME `
                -IncludeAuditStorage $true
            $deployment.Succeeded | Should -BeTrue
            $primaryStorageId = $deployment.Outputs['primaryStorageId'].GetString()
            $auditStorageId = $deployment.Outputs['auditStorageId'].GetString()

            $validation = Get-SampleValidation `
                -Session $session `
                -StackName $env:BICEP_TEST_STACK_NAME `
                -IncludeAuditStorage $false
            $validation.IsValid | Should -BeTrue

            $deployment = Start-SampleDeployment `
                -Session $session `
                -StackName $env:BICEP_TEST_STACK_NAME `
                -IncludeAuditStorage $false
            $deployment.Succeeded | Should -BeTrue
            $deployment.Outputs['primaryStorageId'].GetString() | Should -Be $primaryStorageId
            (Get-AzureResourceResponse $primaryStorageId).StatusCode | Should -Be 200
            (Get-AzureResourceResponse $auditStorageId).StatusCode | Should -Be 404
        }
        finally {
            if ($deployment) {
                $deployment | Remove-BicepTestDeployment
            }
        }

    }
}
