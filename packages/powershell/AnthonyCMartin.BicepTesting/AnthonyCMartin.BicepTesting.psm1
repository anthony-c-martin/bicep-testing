Set-StrictMode -Version Latest
$script:BicepTestSessionType = $null

function Import-BicepTestAssembly {
    if ($script:BicepTestSessionType) {
        return
    }

    $loadedAssembly = [AppDomain]::CurrentDomain.GetAssemblies() |
        Where-Object { $_.GetName().Name -eq 'AnthonyCMartin.BicepTesting' } |
        Select-Object -First 1
    if ($loadedAssembly) {
        $script:BicepTestSessionType = $loadedAssembly.GetType('AnthonyCMartin.BicepTesting.BicepTestSession', $true)
        return
    }

    $libraryPath = Join-Path $PSScriptRoot 'lib/net10.0'
    $assemblyPath = Join-Path $libraryPath 'AnthonyCMartin.BicepTesting.dll'
    if (-not (Test-Path -LiteralPath $assemblyPath)) {
        throw "The BicepTest runtime has not been built. Run packages/powershell/build.ps1."
    }

    Get-ChildItem -LiteralPath $libraryPath -Filter '*.dll' |
        Where-Object Name -ne 'AnthonyCMartin.BicepTesting.dll' |
        ForEach-Object {
            try {
                [System.Runtime.Loader.AssemblyLoadContext]::Default.LoadFromAssemblyPath($_.FullName) | Out-Null
            }
            catch [System.IO.FileLoadException] {
                # The PowerShell host may already provide the same framework assembly.
            }
        }
    $assembly = [System.Runtime.Loader.AssemblyLoadContext]::Default.LoadFromAssemblyPath($assemblyPath)
    $script:BicepTestSessionType = $assembly.GetType('AnthonyCMartin.BicepTesting.BicepTestSession', $true)
}

function New-BicepTestSession {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory, Position = 0)]
        [ValidateNotNullOrEmpty()]
        [string] $BicepVersion
    )

    Import-BicepTestAssembly
    $method = $script:BicepTestSessionType.GetMethod('CreateAsync')
    $task = $method.Invoke($null, @($BicepVersion, [Threading.CancellationToken]::None))
    $task.GetAwaiter().GetResult()
}

function Get-BicepSnapshot {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory, Position = 0, ValueFromPipeline)]
        [ValidateNotNull()]
        [object] $Session,

        [Parameter(Mandatory, Position = 1)]
        [ValidateNotNullOrEmpty()]
        [string] $Path,

        [string] $TenantId,

        [string] $SubscriptionId,

        [string] $ResourceGroup,

        [string] $Location,

        [string] $DeploymentName
    )

    process {
        $resolvedPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
        $task = $Session.SnapshotAsync(
            $resolvedPath,
            $TenantId,
            $SubscriptionId,
            $ResourceGroup,
            $Location,
            $DeploymentName,
            [Threading.CancellationToken]::None)
        $task.GetAwaiter().GetResult()
    }
}

function Remove-BicepTestSession {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory, Position = 0, ValueFromPipeline)]
        [ValidateNotNull()]
        [object] $Session
    )

    process {
        $Session.Dispose()
    }
}

function Start-BicepTestDeployment {
    [CmdletBinding(SupportsShouldProcess, ConfirmImpact = 'Medium')]
    param(
        [Parameter(Mandatory, Position = 0, ValueFromPipeline)]
        [ValidateNotNull()]
        [object] $Session,

        [Parameter(Mandatory)]
        [ValidateNotNull()]
        [object] $Credential,

        [Parameter(Mandatory)]
        [ValidateNotNullOrEmpty()]
        [string] $Path,

        [Parameter(Mandatory)]
        [ValidateNotNullOrEmpty()]
        [string] $SubscriptionId,

        [Parameter(Mandatory)]
        [ValidateNotNullOrEmpty()]
        [string] $ResourceGroup,

        [Parameter(Mandatory)]
        [ValidateNotNullOrEmpty()]
        [string] $StackName,

        [hashtable] $ParameterOverrides = @{}
    )

    process {
        $target = "$SubscriptionId/$ResourceGroup/$StackName"
        if (-not $PSCmdlet.ShouldProcess($target, 'Create or update Bicep test deployment')) {
            return
        }

        Import-BicepTestAssembly
        $options = [AnthonyCMartin.BicepTesting.DeployOptions]::new()
        $options.FilePath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
        $options.SubscriptionId = $SubscriptionId
        $options.ResourceGroup = $ResourceGroup
        $options.StackName = $StackName

        $overrides = [Collections.Generic.Dictionary[string, Text.Json.JsonElement]]::new()
        foreach ($item in $ParameterOverrides.GetEnumerator()) {
            $valueType = if ($null -eq $item.Value) { [object] } else { $item.Value.GetType() }
            $overrides.Add(
                [string] $item.Key,
                [Text.Json.JsonSerializer]::SerializeToElement(
                    [object] $item.Value,
                    $valueType,
                    [Text.Json.JsonSerializerOptions] $null))
        }
        $options.ParameterOverrides = $overrides

        $task = $Session.DeployAsync(
            $Credential,
            $options,
            [Threading.CancellationToken]::None)
        $task.GetAwaiter().GetResult()
    }
}

function Remove-BicepTestDeployment {
    [CmdletBinding(SupportsShouldProcess, ConfirmImpact = 'Medium')]
    param(
        [Parameter(Mandatory, Position = 0, ValueFromPipeline)]
        [ValidateNotNull()]
        [object] $Deployment
    )

    process {
        if (-not $PSCmdlet.ShouldProcess('Bicep test deployment', 'Remove deployment stack and managed resources')) {
            return
        }

        $task = $Deployment.TeardownAsync([Threading.CancellationToken]::None)
        $task.GetAwaiter().GetResult()
    }
}

Export-ModuleMember -Function Get-BicepSnapshot, New-BicepTestSession, Remove-BicepTestSession, Start-BicepTestDeployment, Remove-BicepTestDeployment