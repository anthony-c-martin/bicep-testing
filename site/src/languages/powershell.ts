import { fakeSubscriptionId, fakeTenantId, repositoryUrl } from "./constants";
import type { LanguageSample } from "./types";

const powershellContinuation = "`";

export const powershell: LanguageSample = {
  id: "powershell",
  label: "PowerShell",
  highlightLanguage: "powershell",
  packageManager: "PSGallery",
  runtime: "PowerShell 7.6+",
  install: "Install-PSResource AnthonyCMartin.BicepTesting",
  registry: "PowerShell Gallery",
  packageUrl:
    "https://www.powershellgallery.com/packages/AnthonyCMartin.BicepTesting",
  guideUrl: `${repositoryUrl}/blob/main/packages/powershell/README.md`,
  testCommand: "Invoke-Pester",
  offlineStarter: `Describe 'Bicep template' {
    BeforeAll {
        $session = New-BicepTestSession -BicepVersion '0.46.1'
    }

    AfterAll { $session | Remove-BicepTestSession }

    It 'all storage accounts have anonymous access disabled' {
        $snapshot = $session | Get-BicepSnapshot ${powershellContinuation}
            -Path 'infra/main.bicepparam' ${powershellContinuation}
            -TenantId '${fakeTenantId}' ${powershellContinuation}
            -SubscriptionId '${fakeSubscriptionId}' ${powershellContinuation}
            -ResourceGroup 'example-rg' ${powershellContinuation}
            -Location 'eastus'
        $snapshot.PredictedResources |
            Where-Object Type -ieq 'Microsoft.Storage/storageAccounts' |
            ForEach-Object {
            $_.Properties.GetProperty('allowBlobPublicAccess').GetBoolean() |
                Should -BeFalse
        }
    }
}`,
  liveValidateStarter: `$session = New-BicepLiveTestSession ${powershellContinuation}
    -BicepVersion '0.46.1' ${powershellContinuation}
    -Credential ([Azure.Identity.DefaultAzureCredential]::new())

try {
    $validation = $session | Test-BicepTestDeployment ${powershellContinuation}
        -Path 'infra/main.bicepparam' ${powershellContinuation}
        -SubscriptionId $env:AZURE_SUBSCRIPTION_ID ${powershellContinuation}
        -ResourceGroup $env:AZURE_RESOURCE_GROUP

    $validation.IsValid | Should -BeTrue
}
finally {
    $session | Remove-BicepTestSession
}`,
  liveDeployStarter: `$session = New-BicepLiveTestSession ${powershellContinuation}
    -BicepVersion '0.46.1' ${powershellContinuation}
    -Credential ([Azure.Identity.DefaultAzureCredential]::new())

$deployment = $null
try {
    $deployment = $session | Start-BicepTestDeployment ${powershellContinuation}
        -Path 'infra/main.bicepparam' ${powershellContinuation}
        -SubscriptionId $env:AZURE_SUBSCRIPTION_ID ${powershellContinuation}
        -ResourceGroup $env:AZURE_RESOURCE_GROUP

    $deployment.Succeeded | Should -BeTrue
}
finally {
    if ($deployment) { $deployment | Remove-BicepTestDeployment }
    $session | Remove-BicepTestSession
}`,
  offlineSampleUrl: `${repositoryUrl}/blob/main/samples/powershell/BicepTest.Sample.Tests.ps1`,
  liveSampleUrl: `${repositoryUrl}/blob/main/samples/powershell/BicepTest.Deployment.Sample.Tests.ps1`,
};
