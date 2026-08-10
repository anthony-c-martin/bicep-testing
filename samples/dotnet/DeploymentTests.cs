using AnthonyCMartin.BicepTesting;
using Azure.Core;
using Azure.Identity;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using System.Net;
using System.Net.Http.Headers;
using System.Text.Json;

namespace Samples;

[TestClass]
public sealed class DeploymentTests
{
    private static readonly HttpClient HttpClient = new();
    private static BicepTestSession session = null!;

    public TestContext TestContext { get; set; } = null!;

    [ClassInitialize]
    public static async Task ClassInitialize(TestContext testContext)
    {
        session = await BicepTestSession.CreateAsync("0.43.1", testContext.CancellationToken);
    }

    [ClassCleanup]
    public static async Task ClassCleanup()
    {
        await session.DisposeAsync();
    }

    [TestMethod]
    [Timeout(15 * 60_000)]
    public async Task Secure_storage_is_verified_in_Azure_and_removed()
    {
        var settings = LiveSettings.Load();
        var credential = new DefaultAzureCredential();
        DeployResult? deployment = null;
        string? primaryStorageId = null;
        try
        {
            deployment = await DeployAsync(session, credential, settings, $"{settings.StackName}-secure", false);
            primaryStorageId = deployment.Outputs["primaryStorageId"].GetString()!;
            using var response = await GetAzureResourceAsync(credential, primaryStorageId);
            Assert.AreEqual(HttpStatusCode.OK, response.StatusCode);
            var storage = JsonDocument.Parse(await response.Content.ReadAsStringAsync(TestContext.CancellationToken));
            var properties = storage.RootElement.GetProperty("properties");
            Assert.IsFalse(properties.GetProperty("allowBlobPublicAccess").GetBoolean());
            Assert.IsFalse(properties.GetProperty("allowSharedKeyAccess").GetBoolean());
            Assert.AreEqual("TLS1_2", properties.GetProperty("minimumTlsVersion").GetString());
            Assert.AreEqual("Disabled", properties.GetProperty("publicNetworkAccess").GetString());
            Assert.IsTrue(properties.GetProperty("supportsHttpsTrafficOnly").GetBoolean());
            Assert.IsTrue(deployment.Resources.Any(resource => resource.Id == primaryStorageId));
        }
        finally
        {
            if (deployment is not null)
            {
                await deployment.TeardownAsync(TestContext.CancellationToken);
            }
        }

        using var deleted = await GetAzureResourceAsync(credential, primaryStorageId!);
        Assert.AreEqual(HttpStatusCode.NotFound, deleted.StatusCode);
    }

    [TestMethod]
    [Timeout(20 * 60_000)]
    public async Task Deployment_reconciles_removed_audit_storage_and_cleans_up()
    {
        var settings = LiveSettings.Load();
        var credential = new DefaultAzureCredential();
        DeployResult? reconciled = null;
        string? primaryStorageId = null;
        string? auditStorageId = null;
        try
        {
            var initial = await DeployAsync(session, credential, settings, settings.StackName, true);
            primaryStorageId = initial.Outputs["primaryStorageId"].GetString()!;
            auditStorageId = initial.Outputs["auditStorageId"].GetString()!;
            Assert.HasCount(2, initial.Resources);

            reconciled = await DeployAsync(session, credential, settings, settings.StackName, false);
            Assert.HasCount(1, reconciled.Resources);
            Assert.AreEqual(primaryStorageId, reconciled.Resources[0].Id);
            using var removedAudit = await GetAzureResourceAsync(credential, auditStorageId);
            Assert.AreEqual(HttpStatusCode.NotFound, removedAudit.StatusCode);
        }
        finally
        {
            if (reconciled is not null)
            {
                await reconciled.TeardownAsync(TestContext.CancellationToken);
            }
        }

        using var removedPrimary = await GetAzureResourceAsync(credential, primaryStorageId!);
        Assert.AreEqual(HttpStatusCode.NotFound, removedPrimary.StatusCode);
    }

    private async Task<DeployResult> DeployAsync(
        BicepTestSession session,
        TokenCredential credential,
        LiveSettings settings,
        string stackName,
        bool includeAuditStorage)
    {
        return await session.DeployAsync(
            credential,
            new DeployOptions
            {
                FilePath = InfraPath("live-storage/main.bicepparam"),
                SubscriptionId = settings.SubscriptionId,
                ResourceGroup = settings.ResourceGroup,
                StackName = stackName,
                ParameterOverrides = new Dictionary<string, JsonElement>
                {
                    ["resourcePrefix"] = JsonSerializer.SerializeToElement(settings.ResourcePrefix),
                    ["includeAuditStorage"] = JsonSerializer.SerializeToElement(includeAuditStorage),
                },
            },
            TestContext.CancellationToken);
    }

    private async Task<HttpResponseMessage> GetAzureResourceAsync(TokenCredential credential, string resourceId)
    {
        var token = await credential.GetTokenAsync(
            new TokenRequestContext(["https://management.azure.com/.default"]),
            TestContext.CancellationToken);
        var request = new HttpRequestMessage(
            HttpMethod.Get,
            $"https://management.azure.com{resourceId}?api-version=2023-05-01");
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token.Token);
        return await HttpClient.SendAsync(request, TestContext.CancellationToken);
    }

    private static string InfraPath(string relativePath) => Path.GetFullPath(
        Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "infra", relativePath));

    private sealed record LiveSettings(string SubscriptionId, string ResourceGroup, string StackName, string ResourcePrefix)
    {
        public static LiveSettings Load() => new(
            Require("AZURE_SUBSCRIPTION_ID"),
            Require("AZURE_RESOURCE_GROUP"),
            Require("BICEP_TEST_STACK_NAME"),
            Require("BICEP_TEST_RESOURCE_PREFIX"));

        private static string Require(string name)
        {
            var value = Environment.GetEnvironmentVariable(name);
            if (string.IsNullOrWhiteSpace(value))
            {
                Assert.Inconclusive($"Set {name} to run the live deployment samples.");
            }
            return value!;
        }
    }
}
