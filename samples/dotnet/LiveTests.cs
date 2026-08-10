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
    public async Task Secure_storage_is_verified_in_Azure_and_removed()
    {
        var settings = LiveSettings.Load();
        DeployResult? deployment = null;
        string? primaryStorageId = null;
        try
        {
            deployment = await DeployAsync(
                session,
                CreateDeployOptions(settings, CreateParameterOverrides(settings, false)));
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
                using var cleanup = new CancellationTokenSource(CleanupTimeout);
                await deployment.TeardownAsync(cleanup.Token);
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
        DeployResult? deployment = null;
        string? primaryStorageId = null;
        string? auditStorageId = null;
        try
        {
            var parameterOverrides = CreateParameterOverrides(settings, true);
            var options = CreateDeployOptions(settings, parameterOverrides);
            deployment = await DeployAsync(session, options);
            primaryStorageId = deployment.Outputs["primaryStorageId"].GetString()!;
            auditStorageId = deployment.Outputs["auditStorageId"].GetString()!;
            Assert.HasCount(2, deployment.Resources);

            parameterOverrides["includeAuditStorage"] = JsonSerializer.SerializeToElement(false);
            deployment = await DeployAsync(session, options);
            Assert.HasCount(1, deployment.Resources);
            Assert.AreEqual(primaryStorageId, deployment.Resources[0].Id);
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

        using var removedPrimary = await GetAzureResourceAsync(credential, primaryStorageId!);
        Assert.AreEqual(HttpStatusCode.NotFound, removedPrimary.StatusCode);
    }

    private async Task<DeployResult> DeployAsync(
        LiveBicepTestSession session,
        DeployOptions options)
    {
        var deployment = await session.DeployAsync(options, TestContext.CancellationToken);
        Assert.IsTrue(deployment.Succeeded, deployment.ErrorMessage);
        return deployment;
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
        var request = new HttpRequestMessage(
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
