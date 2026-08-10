import { fakeSubscriptionId, fakeTenantId, repositoryUrl } from "./constants";
import type { LanguageSample } from "./types";

export const csharp: LanguageSample = {
  id: "csharp",
  label: "C#",
  highlightLanguage: "csharp",
  packageManager: "NuGet",
  runtime: ".NET 10+",
  install: "dotnet add package AnthonyCMartin.BicepTesting",
  registry: "NuGet",
  packageUrl: "https://www.nuget.org/packages/AnthonyCMartin.BicepTesting",
  guideUrl: `${repositoryUrl}/blob/main/packages/dotnet/README.md`,
  testCommand: "dotnet test",
  offlineStarter: `using AnthonyCMartin.BicepTesting;

[TestClass]
public sealed class OfflineTests
{
  private static BicepTestSession session = null!;

  [ClassInitialize]
  public static async Task Initialize(TestContext context)
  {
    session = await BicepTestSession.CreateAsync("0.46.1");
  }

  [ClassCleanup]
  public static async Task Cleanup() => await session.DisposeAsync();

  [TestMethod]
  public async Task All_storage_accounts_disable_anonymous_access()
  {
    var snapshot = await session.SnapshotAsync(new SnapshotOptions
    {
      FilePath = "infra/main.bicepparam",
      TenantId = "${fakeTenantId}",
      SubscriptionId = "${fakeSubscriptionId}",
      ResourceGroup = "example-rg",
      Location = "eastus",
    });

    Assert.IsTrue(snapshot.PredictedResources
      .Where(resource => resource.Type.Equals(
        "Microsoft.Storage/storageAccounts",
        StringComparison.OrdinalIgnoreCase))
      .All(resource =>
        !resource.Properties.GetProperty("allowBlobPublicAccess").GetBoolean()));
  }
}`,
  liveValidateStarter: `using AnthonyCMartin.BicepTesting;
using Azure.Identity;

[TestClass]
public sealed class LiveTests
{
  private static LiveBicepTestSession session = null!;

  [ClassInitialize]
  public static async Task Initialize(TestContext context)
  {
    session = await LiveBicepTestSession.CreateAsync(
      "0.46.1", new DefaultAzureCredential());
  }

  [ClassCleanup]
  public static async Task Cleanup() => await session.DisposeAsync();

  [TestMethod]
  public async Task Template_passes_Azure_validation()
  {
    var validation = await session.ValidateAsync(new DeployOptions
    {
      FilePath = "infra/main.bicepparam",
      SubscriptionId = subscriptionId,
      ResourceGroup = resourceGroup,
    });

    Assert.IsTrue(validation.IsValid, validation.ErrorMessage);
  }
}`,
  liveDeployStarter: `using AnthonyCMartin.BicepTesting;
using Azure.Identity;

[TestClass]
public sealed class LiveTests
{
  private static LiveBicepTestSession session = null!;

  [ClassInitialize]
  public static async Task Initialize(TestContext context)
  {
    session = await LiveBicepTestSession.CreateAsync(
      "0.46.1", new DefaultAzureCredential());
  }

  [ClassCleanup]
  public static async Task Cleanup() => await session.DisposeAsync();

  [TestMethod]
  public async Task Template_deploys_successfully()
  {
    var deployment = await session.DeployAsync(new DeployOptions
    {
      FilePath = "infra/main.bicepparam",
      SubscriptionId = subscriptionId,
      ResourceGroup = resourceGroup,
    });

    try
    {
      Assert.IsTrue(deployment.Succeeded, deployment.ErrorMessage);
    }
    finally
    {
      await deployment.TeardownAsync();
    }
  }
}`,
  offlineSampleUrl: `${repositoryUrl}/blob/main/samples/dotnet/OfflineTests.cs`,
  liveSampleUrl: `${repositoryUrl}/blob/main/samples/dotnet/LiveTests.cs`,
};
