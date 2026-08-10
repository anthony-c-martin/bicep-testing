@{
    RootModule = 'AnthonyCMartin.BicepTesting.psm1'
    ModuleVersion = '0.1.4'
    GUID = 'ba8897af-e89f-4657-a30d-d1c9e9816070'
    Author = 'Anthony Martin'
    Description = 'Test Bicep infrastructure by evaluating deployment snapshots locally.'
    PowerShellVersion = '7.6'
    CompatiblePSEditions = @('Core')
    FunctionsToExport = @(
        'Get-BicepSnapshot'
        'New-BicepTestSession'
        'Remove-BicepTestDeployment'
        'Remove-BicepTestSession'
        'Start-BicepTestDeployment'
    )
    CmdletsToExport = @()
    VariablesToExport = @()
    AliasesToExport = @()
    PrivateData = @{
        PSData = @{
            LicenseUri = 'https://github.com/anthony-c-martin/bicep-testing/blob/main/LICENSE'
            ProjectUri = 'https://github.com/anthony-c-martin/bicep-testing'
            Tags = @('Bicep', 'Testing', 'InfrastructureAsCode')
        }
    }
}