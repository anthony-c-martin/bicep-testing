#!/usr/bin/env pwsh

$ErrorActionPreference = 'Stop'
$repositoryRoot = Split-Path $PSScriptRoot -Parent
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("bicep-testing-samples-" + [Guid]::NewGuid().ToString('N'))

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

function New-Directory {
    param(
        [Parameter(Mandatory)]
        [string] $Path
    )

    [void](New-Item -ItemType Directory -Path $Path -Force)
    return $Path
}

function Get-PythonExecutable {
    param(
        [Parameter(Mandatory)]
        [string] $VenvPath
    )

    $scriptsPath = Join-Path $VenvPath 'Scripts/python.exe'
    if (Test-Path -LiteralPath $scriptsPath) {
        return $scriptsPath
    }

    return (Join-Path $VenvPath 'bin/python')
}

Push-Location $repositoryRoot
try {
    $nodeDist = New-Directory (Join-Path $tempRoot 'node-dist')
    $nugetFeed = New-Directory (Join-Path $tempRoot 'nuget-feed')
    $dotnetPackages = New-Directory (Join-Path $tempRoot 'nuget-packages')
    $goWork = New-Directory (Join-Path $tempRoot 'go')
    $goModCache = New-Directory (Join-Path $tempRoot 'go-mod-cache')
    $psModules = New-Directory (Join-Path $tempRoot 'ps-modules')
    $pythonDist = New-Directory (Join-Path $tempRoot 'python-dist')
    $pythonVenv = Join-Path $tempRoot 'python-venv'

    Write-Host 'Packing local Node package from this checkout...'
    Invoke-NativeCommand {
        npm pack --pack-destination "$nodeDist" ./packages/node
    } 'Node package pack'
    $nodePackageTarball = Get-ChildItem -Path $nodeDist -Filter 'anthony-c-martin-bicep-testing-*.tgz' |
        Select-Object -First 1
    if (-not $nodePackageTarball) {
        throw 'Node package tarball was not produced.'
    }

    Write-Host 'Packing local .NET package from this checkout...'
    Invoke-NativeCommand {
        dotnet pack ./packages/dotnet/src/BicepTest/BicepTest.csproj --configuration Release --output "$nugetFeed" --nologo
    } '.NET package pack'

    Write-Host 'Building local PowerShell module assets from this checkout...'
    Invoke-NativeCommand {
        pwsh -NoProfile -File ./packages/powershell/build.ps1
    } 'PowerShell module build'

    Write-Host 'Building local Python wheels from this checkout...'
    Invoke-NativeCommand {
        python -m pip wheel --no-deps --wheel-dir "$pythonDist" ./packages/python/bicep_rpc_client ./packages/python/bicep_testing
    } 'Python package wheel build'

    Write-Host 'Restoring and compiling the Node sample against the local package...'
    Invoke-NativeCommand { npm install --prefix samples/node --ignore-scripts --no-package-lock } 'Node sample dependency restore'
    Invoke-NativeCommand {
        npm install --prefix samples/node --no-package-lock --no-save "$($nodePackageTarball.FullName)"
    } 'Node sample local package install'
    try {
        $env:BICEP_TEST_VALIDATE_ONLY = '1'
        Invoke-NativeCommand { npm test --prefix samples/node } 'Node sample compilation'
    }
    finally {
        Remove-Item Env:BICEP_TEST_VALIDATE_ONLY -ErrorAction SilentlyContinue
    }

    Write-Host 'Building the C# sample tests against the local NuGet package...'
    Invoke-NativeCommand {
        dotnet build ./samples/dotnet/BicepTest.Sample.csproj --configuration Release --source "$nugetFeed" --source 'https://api.nuget.org/v3/index.json' -p:RestorePackagesPath="$dotnetPackages" --nologo
    } 'C# sample build'

    Write-Host 'Building the Go sample tests against local modules with a temporary modfile...'
    $goModFile = Join-Path $goWork 'go.local.mod'
    Copy-Item -LiteralPath (Join-Path $repositoryRoot 'samples/go/go.mod') -Destination $goModFile -Force
    Add-Content -LiteralPath $goModFile -Value ""
    Add-Content -LiteralPath $goModFile -Value "replace github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing => $repositoryRoot/packages/go/bicep-testing"
    Add-Content -LiteralPath $goModFile -Value "replace github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client => $repositoryRoot/packages/go/bicep-rpc-client"

    Push-Location samples/go
    try {
        Invoke-NativeCommand {
            $env:GOMODCACHE = "$goModCache"
            $env:GOSUMDB = 'off'
            go test -run '^$' -mod=mod -modfile "$goModFile" .
        } 'Go sample build'
    }
    finally {
        Remove-Item Env:GOMODCACHE -ErrorAction SilentlyContinue
        Remove-Item Env:GOSUMDB -ErrorAction SilentlyContinue
        Pop-Location
    }

    Write-Host 'Importing local PowerShell module and parsing sample tests...'
    $psModuleVersionPath = New-Directory (Join-Path $psModules 'AnthonyCMartin.BicepTesting/0.1.6')
    Copy-Item -Path (Join-Path $repositoryRoot 'packages/powershell/AnthonyCMartin.BicepTesting/*') -Destination $psModuleVersionPath -Recurse -Force
    $originalPSModulePath = $env:PSModulePath
    try {
        $env:PSModulePath = "$psModules$([IO.Path]::PathSeparator)$originalPSModulePath"
        Import-Module AnthonyCMartin.BicepTesting -RequiredVersion 0.1.6 -Force
        foreach ($command in @(
            'Get-BicepSnapshot',
            'New-BicepTestSession',
            'Start-BicepTestDeployment',
            'Test-BicepTestDeployment',
            'Remove-BicepTestDeployment',
            'Remove-BicepTestSession')) {
            if (-not (Get-Command $command -ErrorAction SilentlyContinue)) {
                throw "Expected command '$command' was not exported by the local module."
            }
        }

        $parseErrors = @()
        Get-ChildItem ./samples/powershell -Filter *.ps1 | ForEach-Object {
            [void][Management.Automation.Language.Parser]::ParseFile($_.FullName, [ref]$null, [ref]$parseErrors)
        }
        if ($parseErrors) {
            throw "PowerShell sample parsing failed:`n$($parseErrors | Out-String)"
        }
    }
    finally {
        Remove-Module AnthonyCMartin.BicepTesting -ErrorAction SilentlyContinue
        $env:PSModulePath = $originalPSModulePath
    }

    Write-Host 'Installing and collecting the Python sample tests in an isolated virtual environment...'
    Invoke-NativeCommand { python -m venv "$pythonVenv" } 'Python virtual environment creation'
    $python = Get-PythonExecutable -VenvPath $pythonVenv
    $pythonRpcWheel = (Get-ChildItem -Path $pythonDist -Filter 'anthonycmartin_bicep_rpc_client-*.whl' | Select-Object -First 1).FullName
    $pythonTestingWheel = (Get-ChildItem -Path $pythonDist -Filter 'anthonycmartin_bicep_testing-*.whl' | Select-Object -First 1).FullName
    if (-not $pythonRpcWheel -or -not $pythonTestingWheel) {
        throw 'Expected local Python wheel files were not produced.'
    }
    $pythonRequirements = Join-Path $tempRoot 'python-sample-requirements.txt'
    Get-Content ./samples/python/requirements.txt |
        Where-Object {
            $_ -notmatch '^anthonycmartin-bicep-rpc-client' -and $_ -notmatch '^anthonycmartin-bicep-testing'
        } |
        Set-Content -LiteralPath $pythonRequirements

    Invoke-NativeCommand { & $python -m pip install -r "$pythonRequirements" } 'Python sample dependency restore'
    Invoke-NativeCommand {
        & $python -m pip install "$pythonRpcWheel" "$pythonTestingWheel"
    } 'Python sample local package install'

    Push-Location samples/python
    try {
        Invoke-NativeCommand { & $python -m pytest --collect-only -q } 'Python sample collection'
    }
    finally {
        Pop-Location
    }
}
finally {
    Pop-Location
    Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host 'All language samples compiled or collected successfully.'