[CmdletBinding()]
param(
    [switch] $Update,
    [switch] $Check
)

$ErrorActionPreference = 'Stop'
if ($Update -eq $Check) {
    throw 'Specify exactly one of -Update or -Check.'
}

$modulePath = Join-Path $PSScriptRoot '../AnthonyCMartin.BicepTesting/AnthonyCMartin.BicepTesting.psd1'
$baselinePath = Join-Path $PSScriptRoot '../../../api/powershell/AnthonyCMartin.BicepTesting.txt'
$commonParameters = @(
    'Debug', 'ErrorAction', 'ErrorVariable', 'InformationAction',
    'InformationVariable', 'OutBuffer', 'OutVariable', 'PipelineVariable',
    'ProgressAction', 'Verbose', 'WarningAction', 'WarningVariable'
)

$module = Import-Module $modulePath -Force -PassThru
$lines = [Collections.Generic.List[string]]::new()
foreach ($command in $module.ExportedCommands.Values | Sort-Object Name) {
    $lines.Add("FUNCTION $($command.Name)")
    $lines.Add('')

    foreach ($parameterSet in $command.ParameterSets | Sort-Object Name) {
        $lines.Add("PARAMETER SET $($parameterSet.Name)")
        $parameters = $parameterSet.Parameters |
            Where-Object Name -NotIn $commonParameters |
            Sort-Object @{ Expression = { if ($_.Position -ge 0) { $_.Position } else { [int]::MaxValue } } }, Name
        foreach ($parameter in $parameters) {
            $attributes = [Collections.Generic.List[string]]::new()
            if ($parameter.IsMandatory) {
                $attributes.Add('Mandatory')
            }
            if ($parameter.Position -ge 0) {
                $attributes.Add("Position=$($parameter.Position)")
            }
            if ($parameter.ValueFromPipeline) {
                $attributes.Add('ValueFromPipeline')
            }
            if ($parameter.ValueFromPipelineByPropertyName) {
                $attributes.Add('ValueFromPipelineByPropertyName')
            }

            $suffix = if ($attributes.Count -gt 0) { " [$($attributes -join ', ')]" } else { '' }
            $lines.Add("  -$($parameter.Name) <$($parameter.ParameterType.FullName)>$suffix")
        }
        $lines.Add('')
    }
}

$generated = (($lines -join "`n").TrimEnd() + "`n")
if ($Update) {
    $directory = Split-Path $baselinePath -Parent
    [IO.Directory]::CreateDirectory($directory) | Out-Null
    [IO.File]::WriteAllText($baselinePath, $generated, [Text.UTF8Encoding]::new($false))
    Write-Host "Updated api/powershell/AnthonyCMartin.BicepTesting.txt"
    return
}

if (-not (Test-Path -LiteralPath $baselinePath)) {
    throw 'PowerShell public API baseline is missing. Run scripts/public-api.ps1 -Update.'
}
$baseline = ([IO.File]::ReadAllText($baselinePath) -replace "`r`n", "`n").TrimEnd() + "`n"
if ($generated -cne $baseline) {
    throw 'PowerShell public API has changed. Review it and run scripts/public-api.ps1 -Update.'
}
Write-Host 'PowerShell public API is up to date.'