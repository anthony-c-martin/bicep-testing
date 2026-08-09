#!/usr/bin/env pwsh

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path $PSScriptRoot -Parent

function Invoke-NativeCommand {
    param(
        [Parameter(Mandatory)]
        [scriptblock] $Command,

        [Parameter(Mandatory)]
        [string] $Description
    )

    & $Command
    if ($LASTEXITCODE -ne 0) {
        throw "$Description failed with exit code $LASTEXITCODE."
    }
}

Push-Location $repositoryRoot
try {
    Write-Host 'Restoring and compiling the Node sample...'
    Invoke-NativeCommand { npm install --prefix samples/node --ignore-scripts --no-package-lock } 'Node sample dependency restore'
    Invoke-NativeCommand { $env:BICEP_TEST_VALIDATE_ONLY = '1'; npm test --prefix samples/node } 'Node sample compilation'
    Remove-Item Env:BICEP_TEST_VALIDATE_ONLY -ErrorAction SilentlyContinue

    Write-Host 'Building the C# sample tests...'
    Invoke-NativeCommand { dotnet build samples/dotnet/BicepTest.Sample.csproj --configuration Release } 'C# sample build'

    Write-Host 'Building the Go sample tests...'
    Push-Location samples/go
    try {
        Invoke-NativeCommand { go test -run '^$' . } 'Go sample build'
    }
    finally {
        Pop-Location
    }

    Write-Host 'Installing and parsing the PowerShell sample tests...'
    Install-PSResource AnthonyCMartin.BicepTesting -Version 0.1.1 -Repository PSGallery -Scope CurrentUser -TrustRepository -Reinstall -Quiet
    Invoke-NativeCommand {
        pwsh -NoProfile -Command '$errors = @(); Get-ChildItem ./samples/powershell -Filter *.ps1 | ForEach-Object { [void][Management.Automation.Language.Parser]::ParseFile($_.FullName, [ref]$null, [ref]$errors) }; if ($errors) { $errors | Out-String | Write-Error; exit 1 }'
    } 'PowerShell sample parsing'

    Write-Host 'Installing and collecting the Python sample tests...'
    Push-Location samples/python
    try {
        Invoke-NativeCommand { python -m pip install -r requirements.txt } 'Python sample dependency restore'
        Invoke-NativeCommand { python -m pytest --collect-only -q } 'Python sample collection'
    }
    finally {
        Pop-Location
    }

}
finally {
    Pop-Location
}

Write-Host 'All language samples compiled or collected successfully.'