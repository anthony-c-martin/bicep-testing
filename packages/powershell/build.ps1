[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$projectPath = Join-Path $PSScriptRoot '../dotnet/src/BicepTest/BicepTest.csproj'
$outputPath = Join-Path $PSScriptRoot 'BicepTest/lib/net10.0'

dotnet publish $projectPath --configuration Release --framework net10.0 --output $outputPath --nologo
if ($LASTEXITCODE -ne 0) {
    throw "dotnet publish failed with exit code $LASTEXITCODE."
}