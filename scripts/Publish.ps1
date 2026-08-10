#!/usr/bin/env pwsh

[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateSet('Validate', 'Node', 'DotNet', 'PowerShell', 'Python', 'Go')]
    [string] $Package,

    [Parameter(Mandatory)]
    [string] $Version,

    [switch] $SkipPublish,

    [string] $CommitSha
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 3.0

$repositoryRoot = Split-Path $PSScriptRoot -Parent
$releaseVersion = $Version -replace '^v', ''
if ($releaseVersion -notmatch '^\d+\.\d+\.\d+$') {
    throw "Version must have the form X.Y.Z or vX.Y.Z, got '$Version'."
}

function Invoke-NativeCommand {
    param(
        [Parameter(Mandatory)]
        [string] $FilePath,

        [Parameter()]
        [string[]] $ArgumentList = @()
    )

    & $FilePath @ArgumentList
    if ($LASTEXITCODE -ne 0) {
        throw "$FilePath failed with exit code $LASTEXITCODE."
    }
}

function Assert-Version {
    param(
        [Parameter(Mandatory)]
        [string] $Name,

        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string] $Actual
    )

    if ($Actual -ne $releaseVersion) {
        throw "$Name version '$Actual' does not match release version '$releaseVersion'."
    }
}

function Test-AllVersions {
    $node = Get-Content (Join-Path $repositoryRoot 'packages/node/package.json') -Raw | ConvertFrom-Json
    $nodeLock = Get-Content (Join-Path $repositoryRoot 'packages/node/package-lock.json') -Raw | ConvertFrom-Json -AsHashtable
    [xml] $dotnet = Get-Content (Join-Path $repositoryRoot 'packages/dotnet/src/BicepTest/BicepTest.csproj') -Raw
    $powerShellManifest = Import-PowerShellDataFile (Join-Path $repositoryRoot 'packages/powershell/AnthonyCMartin.BicepTesting/AnthonyCMartin.BicepTesting.psd1')
    $pythonContent = Get-Content (Join-Path $repositoryRoot 'packages/python/bicep_testing/pyproject.toml') -Raw
    $pythonRpcContent = Get-Content (Join-Path $repositoryRoot 'packages/python/bicep_rpc_client/pyproject.toml') -Raw
    $goMod = Get-Content (Join-Path $repositoryRoot 'packages/go/go.mod') -Raw

    Assert-Version 'Node' $node.version
    Assert-Version 'Node lockfile' $nodeLock.version
    Assert-Version 'Node lockfile root package' $nodeLock.packages[''].version
    Assert-Version '.NET' $dotnet.Project.PropertyGroup.Version
    Assert-Version 'PowerShell' $powerShellManifest.ModuleVersion
    Assert-Version 'Python' ([regex]::Match($pythonContent, '(?m)^version = "([^"]+)"$').Groups[1].Value)
    Assert-Version 'Python RPC client' ([regex]::Match($pythonRpcContent, '(?m)^version = "([^"]+)"$').Groups[1].Value)
    Assert-Version 'Go RPC dependency' ([regex]::Match($goMod, 'bicep-rpc-client\s+v([^\s]+)').Groups[1].Value)

    Write-Host "Every package is ready for v$releaseVersion."
}

function Publish-NodePackage {
    Push-Location (Join-Path $repositoryRoot 'packages/node')
    try {
        Invoke-NativeCommand npm @('ci', '--legacy-peer-deps')
        Invoke-NativeCommand npm @('test')
        Invoke-NativeCommand npm @('run', 'api:check')
        $actualVersion = & node -p "require('./package.json').version"
        if ($LASTEXITCODE -ne 0) { throw 'Unable to read the Node package version.' }
        Assert-Version 'Node' $actualVersion
        if (-not $SkipPublish) {
            Invoke-NativeCommand npm @('publish')
        }
    }
    finally {
        Pop-Location
    }
}

function Publish-DotNetPackage {
    $project = Join-Path $repositoryRoot 'packages/dotnet/src/BicepTest/BicepTest.csproj'
    $solution = Join-Path $repositoryRoot 'packages/dotnet/BicepTest.slnx'
    $output = Join-Path $repositoryRoot 'artifacts/dotnet'
    $actualVersion = & dotnet msbuild $project '-getProperty:Version' '-nologo'
    if ($LASTEXITCODE -ne 0) { throw 'Unable to read the .NET package version.' }
    Assert-Version '.NET' $actualVersion
    Invoke-NativeCommand dotnet @('test', $solution, '--configuration', 'Release')
    Invoke-NativeCommand dotnet @('pack', $project, '--configuration', 'Release', "-p:PackageOutputPath=$output", '-p:ContinuousIntegrationBuild=true')
}

function Publish-PowerShellPackage {
    $manifestPath = Join-Path $repositoryRoot 'packages/powershell/AnthonyCMartin.BicepTesting/AnthonyCMartin.BicepTesting.psd1'
    $modulePath = Split-Path $manifestPath -Parent
    Assert-Version 'PowerShell' (Import-PowerShellDataFile $manifestPath).ModuleVersion
    & (Join-Path $repositoryRoot 'packages/powershell/build.ps1')
    Invoke-Pester (Join-Path $repositoryRoot 'packages/powershell/test') -CI
    & (Join-Path $repositoryRoot 'packages/powershell/scripts/public-api.ps1') -Check
    if (-not $SkipPublish) {
        if ([string]::IsNullOrWhiteSpace($env:PSGALLERY_API_KEY)) {
            throw 'PSGALLERY_API_KEY is required to publish the PowerShell module.'
        }
        Publish-Module -Path $modulePath -NuGetApiKey $env:PSGALLERY_API_KEY -Repository PSGallery
    }
}

function Publish-PythonPackages {
    Invoke-NativeCommand python @('-m', 'pip', 'install', 'build', '-e', "$(Join-Path $repositoryRoot 'packages/python/bicep_rpc_client')[test]", '-e', "$(Join-Path $repositoryRoot 'packages/python/bicep_testing')[test]")
    Invoke-NativeCommand python @('-m', 'pytest', (Join-Path $repositoryRoot 'packages/python/bicep_testing/tests'), (Join-Path $repositoryRoot 'packages/python/bicep_rpc_client/tests'))
    Invoke-NativeCommand python @((Join-Path $repositoryRoot 'packages/python/scripts/public_api.py'), '--check')

    $projectDirectory = Join-Path $repositoryRoot 'packages/python/bicep_testing'
    $rpcProjectDirectory = Join-Path $repositoryRoot 'packages/python/bicep_rpc_client'
    $distributionDirectory = Join-Path $repositoryRoot 'artifacts/python'
    $pyproject = Get-Content (Join-Path $projectDirectory 'pyproject.toml') -Raw
    $rpcPyproject = Get-Content (Join-Path $rpcProjectDirectory 'pyproject.toml') -Raw
    Assert-Version 'Python' ([regex]::Match($pyproject, '(?m)^version = "([^"]+)"$').Groups[1].Value)
    Assert-Version 'Python RPC client' ([regex]::Match($rpcPyproject, '(?m)^version = "([^"]+)"$').Groups[1].Value)
    Remove-Item $distributionDirectory -Recurse -Force -ErrorAction SilentlyContinue
    Invoke-NativeCommand python @('-m', 'build', $rpcProjectDirectory, '--outdir', $distributionDirectory)
    Invoke-NativeCommand python @('-m', 'build', $projectDirectory, '--outdir', $distributionDirectory)
}

function Publish-GoPackages {
    $rootModuleDirectory = Join-Path $repositoryRoot 'packages/go'
    $rpcModuleDirectory = Join-Path $rootModuleDirectory 'bicep-rpc-client'
    Push-Location $rootModuleDirectory
    try {
        Invoke-NativeCommand go @('test', './...')
        Invoke-NativeCommand go @('run', './internal/apidoc', '--check')
        $rootModule = & go list -m
        if ($LASTEXITCODE -ne 0) { throw 'Unable to read the root Go module path.' }
    }
    finally {
        Pop-Location
    }
    Push-Location $rpcModuleDirectory
    try {
        Invoke-NativeCommand go @('test', './...')
        $rpcModule = & go list -m
        if ($LASTEXITCODE -ne 0) { throw 'Unable to read the RPC Go module path.' }
    }
    finally {
        Pop-Location
    }

    if ($rootModule -ne 'github.com/anthony-c-martin/bicep-testing/packages/go') {
        throw "Unexpected root Go module path: $rootModule"
    }
    if ($rpcModule -ne 'github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client') {
        throw "Unexpected RPC Go module path: $rpcModule"
    }
    if ($SkipPublish) { return }

    if ([string]::IsNullOrWhiteSpace($CommitSha)) {
        $CommitSha = & git -C $repositoryRoot rev-parse HEAD
        if ($LASTEXITCODE -ne 0) { throw 'Unable to determine the release commit.' }
    }
    $tags = @(
        @{ Name = "packages/go/bicep-rpc-client/v$releaseVersion"; Message = "Release Go RPC client v$releaseVersion" }
        @{ Name = "packages/go/v$releaseVersion"; Message = "Release Go test library v$releaseVersion" }
    )
    foreach ($tag in $tags) {
        & git -C $repositoryRoot rev-parse --quiet --verify "refs/tags/$($tag.Name)" *> $null
        if ($LASTEXITCODE -eq 0) {
            throw "Tag $($tag.Name) already exists."
        }
    }
    Invoke-NativeCommand git @('-C', $repositoryRoot, 'config', 'user.name', 'github-actions[bot]')
    Invoke-NativeCommand git @('-C', $repositoryRoot, 'config', 'user.email', '41898282+github-actions[bot]@users.noreply.github.com')
    foreach ($tag in $tags) {
        Invoke-NativeCommand git @('-C', $repositoryRoot, 'tag', '-a', $tag.Name, $CommitSha, '-m', $tag.Message)
    }
    Invoke-NativeCommand git @('-C', $repositoryRoot, 'push', 'origin', "refs/tags/$($tags[0].Name)", "refs/tags/$($tags[1].Name)")
}

Set-Location $repositoryRoot
switch ($Package) {
    'Validate' { Test-AllVersions }
    'Node' { Publish-NodePackage }
    'DotNet' { Publish-DotNetPackage }
    'PowerShell' { Publish-PowerShellPackage }
    'Python' { Publish-PythonPackages }
    'Go' { Publish-GoPackages }
}