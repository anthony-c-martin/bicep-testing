using AnthonyCMartin.BicepTesting;
using Azure.Identity;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using System.Text.Json;

namespace Samples;

[TestClass]
public sealed class DeploymentTests
{
    public TestContext TestContext { get; set; } = null!;

    [TestMethod]
    [Timeout(15 * 60_000)]
    public async Task Infrastructure_deploys_and_is_removed_afterward()
    {
        var subscriptionId = RequireEnvironmentVariable("AZURE_SUBSCRIPTION_ID");
        var resourceGroup = RequireEnvironmentVariable("AZURE_RESOURCE_GROUP");
        var stackName = RequireEnvironmentVariable("BICEP_TEST_STACK_NAME");
        var resourcePrefix = RequireEnvironmentVariable("BICEP_TEST_RESOURCE_PREFIX");
        var parametersPath = Path.GetFullPath(
            Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "infra", "main.bicepparam"));

        await using var session = await AnthonyCMartin.BicepTesting.BicepTestSession.CreateAsync("0.43.1", TestContext.CancellationToken);
        await using var deployment = await session.DeployAsync(
            new DefaultAzureCredential(),
            new DeployOptions
            {
                FilePath = parametersPath,
                SubscriptionId = subscriptionId,
                ResourceGroup = resourceGroup,
                StackName = stackName,
                ParameterOverrides = new Dictionary<string, JsonElement>
                {
                    ["env"] = JsonSerializer.SerializeToElement(resourcePrefix),
                },
            },
            TestContext.CancellationToken);

        Assert.IsTrue(deployment.Resources.Any(resource =>
            resource.Type == "Microsoft.Storage/storageAccounts"));
        StringAssert.Contains(
            deployment.Outputs["primaryStorageId"].GetString(),
            "/providers/Microsoft.Storage/storageAccounts/");
    }

    private static string RequireEnvironmentVariable(string name)
    {
        var value = Environment.GetEnvironmentVariable(name);
        if (string.IsNullOrWhiteSpace(value))
        {
            Assert.Inconclusive($"Set {name} to run the live deployment sample.");
        }
        return value!;
    }
}