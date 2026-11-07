using AnthonyCMartin.BicepTesting;
using Azure.Core;
using Azure.Identity;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using System.Net;
using System.Net.Http.Headers;
using System.Text.Json;

namespace Samples;

[TestClass]
public sealed class LiveTests
{
    private static readonly HttpClient HttpClient = new();
    private static readonly TimeSpan CleanupTimeout = TimeSpan.FromMinutes(5);
    private static DefaultAzureCredential credential = null!;
    private static LiveBicepTestSession session = null!;

    public TestContext TestContext { get; set; } = null!;

    [ClassInitialize]
    public static async Task ClassInitialize(TestContext testContext)
    {
        credential = new DefaultAzureCredential();
        session = await LiveBicepTestSession.CreateAsync(
            "0.46.1",
            credential,
            testContext.CancellationToken);
    }

    [ClassCleanup]
    public static async Task ClassCleanup()
    {
        await session.DisposeAsync();
    }

    [TestMethod]
    [Timeout(5 * 60_000)]
    public async Task Secure_storage_template_is_valid_in_Azure()
    {
        var settings = LiveSettings.Load();
        var validation = await session.ValidateAsync(
            CreateDeployOptions(settings, CreateParameterOverrides(settings, false)),
            TestContext.CancellationToken);

        Assert.IsTrue(validation.IsValid, validation.ErrorMessage);
        Assert.IsTrue(validation.Resources.Any(
            resource => resource.Type == "Microsoft.Storage/storageAccounts"));
    }

    [TestMethod]
    [Timeout(15 * 60_000)]
    public async Task Deployed_storage_meets_the_security_baseline_in_Azure()
    {
        var settings = LiveSettings.Load();
        DeployResult? deployment = null;
        try
        {
            deployment = await session.DeployAsync(
                CreateDeployOptions(settings, CreateParameterOverrides(settings, false)),
                TestContext.CancellationToken);
            Assert.IsTrue(deployment.Succeeded, deployment.ErrorMessage);
            var primaryStorageId = deployment.Outputs["primaryStorageId"].GetString()!;
            using var response = await GetAzureResourceAsync(credential, primaryStorageId);
            Assert.AreEqual(HttpStatusCode.OK, response.StatusCode);
            using var storage = JsonDocument.Parse(await response.Content.ReadAsStringAsync(TestContext.CancellationToken));
            var properties = storage.RootElement.GetProperty("properties");
            Assert.IsFalse(properties.GetProperty("allowBlobPublicAccess").GetBoolean());
            Assert.IsFalse(properties.GetProperty("allowSharedKeyAccess").GetBoolean());
            Assert.AreEqual("TLS1_2", properties.GetProperty("minimumTlsVersion").GetString());
            Assert.AreEqual("Disabled", properties.GetProperty("publicNetworkAccess").GetString());
            Assert.IsTrue(properties.GetProperty("supportsHttpsTrafficOnly").GetBoolean());
        }
        finally
        {
            if (deployment is not null)
            {
                using var cleanup = new CancellationTokenSource(CleanupTimeout);
                await deployment.TeardownAsync(cleanup.Token);
            }
        }
    }

    [TestMethod]
    [Timeout(20 * 60_000)]
    public async Task Disabling_audit_storage_removes_the_account_from_Azure()
    {
        var settings = LiveSettings.Load();
        DeployResult? deployment = null;
        try
        {
            var parameterOverrides = CreateParameterOverrides(settings, true);
            var options = CreateDeployOptions(settings, parameterOverrides);
            deployment = await session.DeployAsync(options, TestContext.CancellationToken);
            Assert.IsTrue(deployment.Succeeded, deployment.ErrorMessage);
            var primaryStorageId = deployment.Outputs["primaryStorageId"].GetString()!;
            var auditStorageId = deployment.Outputs["auditStorageId"].GetString()!;

            parameterOverrides["includeAuditStorage"] = JsonSerializer.SerializeToElement(false);
            deployment = await session.DeployAsync(options, TestContext.CancellationToken);
            Assert.IsTrue(deployment.Succeeded, deployment.ErrorMessage);
            Assert.AreEqual(primaryStorageId, deployment.Outputs["primaryStorageId"].GetString());
            using var primary = await GetAzureResourceAsync(credential, primaryStorageId);
            Assert.AreEqual(HttpStatusCode.OK, primary.StatusCode);
            using var removedAudit = await GetAzureResourceAsync(credential, auditStorageId);
            Assert.AreEqual(HttpStatusCode.NotFound, removedAudit.StatusCode);
        }
        finally
        {
            if (deployment is not null)
            {
                using var cleanup = new CancellationTokenSource(CleanupTimeout);
                await deployment.TeardownAsync(cleanup.Token);
            }
        }
    }

    [TestMethod]
    [Timeout(20 * 60_000)]
    public async Task Identical_redeployment_preserves_resource_identity_and_security()
    {
        var settings = LiveSettings.Load();
        var options = CreateDeployOptions(settings, CreateParameterOverrides(settings, false));
        DeployResult? deployment = null;
        try
        {
            deployment = await session.DeployAsync(options, TestContext.CancellationToken);
            Assert.IsTrue(deployment.Succeeded, deployment.ErrorMessage);
            var primaryStorageId = deployment.Outputs["primaryStorageId"].GetString()!;

            deployment = await session.DeployAsync(options, TestContext.CancellationToken);
            Assert.IsTrue(deployment.Succeeded, deployment.ErrorMessage);
            Assert.AreEqual(primaryStorageId, deployment.Outputs["primaryStorageId"].GetString());
            using var response = await GetAzureResourceAsync(credential, primaryStorageId);
            Assert.AreEqual(HttpStatusCode.OK, response.StatusCode);
            using var storage = JsonDocument.Parse(await response.Content.ReadAsStringAsync(TestContext.CancellationToken));
            var properties = storage.RootElement.GetProperty("properties");
            Assert.IsFalse(properties.GetProperty("allowBlobPublicAccess").GetBoolean());
            Assert.AreEqual("TLS1_2", properties.GetProperty("minimumTlsVersion").GetString());
            Assert.AreEqual("Disabled", properties.GetProperty("publicNetworkAccess").GetString());
        }
        finally
        {
            if (deployment is not null)
            {
                using var cleanup = new CancellationTokenSource(CleanupTimeout);
                await deployment.TeardownAsync(cleanup.Token);
            }
        }
    }

    private static DeployOptions CreateDeployOptions(
        LiveSettings settings,
        IReadOnlyDictionary<string, JsonElement> parameterOverrides) => new()
        {
            FilePath = InfraPath("live-storage/main.bicepparam"),
            SubscriptionId = settings.SubscriptionId,
            ResourceGroup = settings.ResourceGroup,
            ParameterOverrides = parameterOverrides,
        };

    private static Dictionary<string, JsonElement> CreateParameterOverrides(
        LiveSettings settings,
        bool includeAuditStorage) => new()
        {
            ["resourcePrefix"] = JsonSerializer.SerializeToElement(settings.ResourcePrefix),
            ["includeAuditStorage"] = JsonSerializer.SerializeToElement(includeAuditStorage),
        };

    private async Task<HttpResponseMessage> GetAzureResourceAsync(TokenCredential credential, string resourceId)
    {
        var token = await credential.GetTokenAsync(
            new TokenRequestContext(["https://management.azure.com/.default"]),
            TestContext.CancellationToken);
        using var request = new HttpRequestMessage(
            HttpMethod.Get,
            $"https://management.azure.com{resourceId}?api-version=2023-05-01");
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token.Token);
        return await HttpClient.SendAsync(request, TestContext.CancellationToken);
    }

    private static string InfraPath(string relativePath) => Path.GetFullPath(
        Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "infra", relativePath));

    private sealed record LiveSettings(string SubscriptionId, string ResourceGroup, string ResourcePrefix)
    {
        public static LiveSettings Load() => new(
            Require("AZURE_SUBSCRIPTION_ID"),
            Require("AZURE_RESOURCE_GROUP"),
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
