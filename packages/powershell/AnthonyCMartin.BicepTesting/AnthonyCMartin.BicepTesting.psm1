Set-StrictMode -Version Latest
$script:BicepTestSessionType = $null
$script:LiveBicepTestSessionType = $null

function Import-BicepTestAssembly {
    if ($script:BicepTestSessionType -and $script:LiveBicepTestSessionType) {
        return
    }

    $loadedAssembly = [AppDomain]::CurrentDomain.GetAssemblies() |
        Where-Object { $_.GetName().Name -eq 'AnthonyCMartin.BicepTesting' } |
        Select-Object -First 1
    if ($loadedAssembly) {
        $script:BicepTestSessionType = $loadedAssembly.GetType('AnthonyCMartin.BicepTesting.BicepTestSession', $true)
        $script:LiveBicepTestSessionType = $loadedAssembly.GetType('AnthonyCMartin.BicepTesting.LiveBicepTestSession', $true)
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
        $script:LiveBicepTestSessionType = $assembly.GetType('AnthonyCMartin.BicepTesting.LiveBicepTestSession', $true)
}

function New-BicepTestSession {
    [CmdletBinding()]
    param(
        [Parameter(Mandatory, Position = 0)]
        [ValidateNotNullOrEmpty()]
        [string] $BicepVersion
    )

    Import-BicepTestAssembly
    $credentialType = [AppDomain]::CurrentDomain.GetAssemblies() |
        Where-Object { $_.GetName().Name -eq 'Azure.Identity' } |
        Select-Object -First 1 |
        ForEach-Object { $_.GetType('Azure.Identity.AzurePowerShellCredential', $true) }
    $credential = [Activator]::CreateInstance($credentialType)
    $method = $script:LiveBicepTestSessionType.GetMethod('CreateAsync')
    $task = $method.Invoke($null, @($BicepVersion, $credential, [Threading.CancellationToken]::None))
    $task.GetAwaiter().GetResult()
}

function ConvertTo-BicepJsonElementDictionary {
    param(
        [AllowNull()]
        [hashtable] $ParameterOverrides
    )

    $overrides = [Collections.Generic.Dictionary[string, Text.Json.JsonElement]]::new()
    if ($null -eq $ParameterOverrides) {
        return $overrides
    }

    foreach ($item in $ParameterOverrides.GetEnumerator()) {
        $valueType = if ($null -eq $item.Value) { [object] } else { $item.Value.GetType() }
        $overrides.Add(
            [string] $item.Key,
            [Text.Json.JsonSerializer]::SerializeToElement(
                [object] $item.Value,
                $valueType,
                [Text.Json.JsonSerializerOptions] $null))
    }

    return $overrides
}

function New-BicepDeployOptions {
    param(
        [Parameter(Mandatory)]
        [string] $Path,

        [Parameter(Mandatory)]
        [string] $ParameterSetName,

        [string] $SubscriptionId,

        [string] $ResourceGroup,

        [string] $ManagementGroupId,

        [string] $Location,

        [AllowNull()]
        [string] $StackName,

        [AllowNull()]
        [hashtable] $ParameterOverrides
    )

    Import-BicepTestAssembly
    $properties = [ordered] @{
        FilePath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
        ParameterOverrides = ConvertTo-BicepJsonElementDictionary -ParameterOverrides $ParameterOverrides
    }

    switch ($ParameterSetName) {
        'ResourceGroup' {
            $properties.SubscriptionId = $SubscriptionId
            $properties.ResourceGroup = $ResourceGroup
            if (-not [string]::IsNullOrWhiteSpace($Location)) {
                $properties.Location = $Location
            }
        }
        'Subscription' {
            $properties.SubscriptionId = $SubscriptionId
            $properties.Location = $Location
        }
        'ManagementGroup' {
            $properties.ManagementGroupId = $ManagementGroupId
            $properties.Location = $Location
        }
        default {
            throw "Unsupported parameter set '$ParameterSetName'."
        }
    }

    if (-not [string]::IsNullOrWhiteSpace($StackName)) {
        $properties.StackName = $StackName
    }

    return [AnthonyCMartin.BicepTesting.DeployOptions] $properties
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
        Import-BicepTestAssembly
        $properties = [ordered] @{
            FilePath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
        }

        if ($PSBoundParameters.ContainsKey('TenantId')) {
            $properties.TenantId = $TenantId
        }
        if ($PSBoundParameters.ContainsKey('SubscriptionId')) {
            $properties.SubscriptionId = $SubscriptionId
        }
        if ($PSBoundParameters.ContainsKey('ResourceGroup')) {
            $properties.ResourceGroup = $ResourceGroup
        }
        if ($PSBoundParameters.ContainsKey('Location')) {
            $properties.Location = $Location
        }
        if ($PSBoundParameters.ContainsKey('DeploymentName')) {
            $properties.DeploymentName = $DeploymentName
        }
        $options = [AnthonyCMartin.BicepTesting.SnapshotOptions] $properties

        $task = $Session.SnapshotAsync(
            $options,
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
    [CmdletBinding(DefaultParameterSetName = 'ResourceGroup', SupportsShouldProcess, ConfirmImpact = 'Medium')]
    param(
        [Parameter(Mandatory, Position = 0, ValueFromPipeline, ParameterSetName = 'ResourceGroup')]
        [Parameter(Mandatory, Position = 0, ValueFromPipeline, ParameterSetName = 'Subscription')]
        [Parameter(Mandatory, Position = 0, ValueFromPipeline, ParameterSetName = 'ManagementGroup')]
        [ValidateNotNull()]
        [object] $Session,

        [Parameter(Mandatory, ParameterSetName = 'ResourceGroup')]
        [Parameter(Mandatory, ParameterSetName = 'Subscription')]
        [Parameter(Mandatory, ParameterSetName = 'ManagementGroup')]
        [ValidateNotNullOrEmpty()]
        [string] $Path,

        [Parameter(Mandatory, ParameterSetName = 'ResourceGroup')]
        [Parameter(Mandatory, ParameterSetName = 'Subscription')]
        [ValidateNotNullOrEmpty()]
        [string] $SubscriptionId,

        [Parameter(Mandatory, ParameterSetName = 'ResourceGroup')]
        [ValidateNotNullOrEmpty()]
        [string] $ResourceGroup,

        [Parameter(Mandatory, ParameterSetName = 'ManagementGroup')]
        [ValidateNotNullOrEmpty()]
        [string] $ManagementGroupId,

        [Parameter(ParameterSetName = 'ResourceGroup')]
        [Parameter(Mandatory, ParameterSetName = 'Subscription')]
        [Parameter(Mandatory, ParameterSetName = 'ManagementGroup')]
        [ValidateNotNullOrEmpty()]
        [string] $Location,

        [ValidateNotNullOrEmpty()]
        [string] $StackName,

        [hashtable] $ParameterOverrides
    )

    process {
        $options = New-BicepDeployOptions `
            -Path $Path `
            -ParameterSetName $PSCmdlet.ParameterSetName `
            -SubscriptionId $SubscriptionId `
            -ResourceGroup $ResourceGroup `
            -ManagementGroupId $ManagementGroupId `
            -Location $Location `
            -StackName $StackName `
            -ParameterOverrides $ParameterOverrides

        $scope = if ($PSCmdlet.ParameterSetName -eq 'ManagementGroup') {
            "managementGroups/$($options.ManagementGroupId)"
        }
        elseif ($PSCmdlet.ParameterSetName -eq 'Subscription') {
            "subscriptions/$($options.SubscriptionId)"
        }
        else {
            "subscriptions/$($options.SubscriptionId)/resourceGroups/$($options.ResourceGroup)"
        }
        $target = "$scope/stacks/$($options.StackName)"
        if (-not $PSCmdlet.ShouldProcess($target, 'Create or update Bicep test deployment')) {
            return
        }

        $task = $Session.DeployAsync(
            $options,
            [Threading.CancellationToken]::None)
        $task.GetAwaiter().GetResult()
    }
}

function Test-BicepTestDeployment {
    [CmdletBinding(DefaultParameterSetName = 'ResourceGroup')]
    param(
        [Parameter(Mandatory, Position = 0, ValueFromPipeline, ParameterSetName = 'ResourceGroup')]
        [Parameter(Mandatory, Position = 0, ValueFromPipeline, ParameterSetName = 'Subscription')]
        [Parameter(Mandatory, Position = 0, ValueFromPipeline, ParameterSetName = 'ManagementGroup')]
        [ValidateNotNull()]
        [object] $Session,

        [Parameter(Mandatory, ParameterSetName = 'ResourceGroup')]
        [Parameter(Mandatory, ParameterSetName = 'Subscription')]
        [Parameter(Mandatory, ParameterSetName = 'ManagementGroup')]
        [ValidateNotNullOrEmpty()]
        [string] $Path,

        [Parameter(Mandatory, ParameterSetName = 'ResourceGroup')]
        [Parameter(Mandatory, ParameterSetName = 'Subscription')]
        [ValidateNotNullOrEmpty()]
        [string] $SubscriptionId,

        [Parameter(Mandatory, ParameterSetName = 'ResourceGroup')]
        [ValidateNotNullOrEmpty()]
        [string] $ResourceGroup,

        [Parameter(Mandatory, ParameterSetName = 'ManagementGroup')]
        [ValidateNotNullOrEmpty()]
        [string] $ManagementGroupId,

        [Parameter(ParameterSetName = 'ResourceGroup')]
        [Parameter(Mandatory, ParameterSetName = 'Subscription')]
        [Parameter(Mandatory, ParameterSetName = 'ManagementGroup')]
        [ValidateNotNullOrEmpty()]
        [string] $Location,

        [ValidateNotNullOrEmpty()]
        [string] $StackName,

        [hashtable] $ParameterOverrides
    )

    process {
        $options = New-BicepDeployOptions `
            -Path $Path `
            -ParameterSetName $PSCmdlet.ParameterSetName `
            -SubscriptionId $SubscriptionId `
            -ResourceGroup $ResourceGroup `
            -ManagementGroupId $ManagementGroupId `
            -Location $Location `
            -StackName $StackName `
            -ParameterOverrides $ParameterOverrides

        $task = $Session.ValidateAsync(
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

Export-ModuleMember -Function Get-BicepSnapshot, New-BicepTestSession, Remove-BicepTestDeployment, Remove-BicepTestSession, Start-BicepTestDeployment, Test-BicepTestDeployment