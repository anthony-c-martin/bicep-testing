using AnthonyCMartin.BicepTesting;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using System.Text.Json;

namespace Samples;

[TestClass]
public sealed class SnapshotTests
{
    private const string TenantId = "ddbe463a-0554-485d-b589-0b17d60cd38b";
    private const string SubscriptionId = "28c9069e-23e8-47d2-b640-00d2e0f09616";
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
    [Timeout(60_000)]
    public async Task Environment_parameters_select_topology_skus_and_tags()
    {
        var development = await SnapshotAsync(session, "environment-topology/dev.bicepparam");
        var production = await SnapshotAsync(session, "environment-topology/prod.bicepparam");

        Assert.IsEmpty(development.Diagnostics);
        Assert.HasCount(1, development.PredictedResources);
        var developmentStorage = development.PredictedResources[0];
        Assert.AreEqual("ordersdevprimary", developmentStorage.Name);
        Assert.AreEqual("Standard_LRS", Extra(developmentStorage, "sku").GetProperty("name").GetString());
        Assert.AreEqual("dev", Extra(developmentStorage, "tags").GetProperty("environment").GetString());
        Assert.AreEqual(JsonValueKind.Null, development.Outputs["auditStorageId"].ValueKind);

        CollectionAssert.AreEqual(
            new[] { "ordersprodprimary", "ordersprodaudit" },
            production.PredictedResources.Select(resource => resource.Name).ToArray());
        Assert.AreEqual("Standard_ZRS", Extra(production.PredictedResources[0], "sku").GetProperty("name").GetString());
        Assert.AreEqual("confidential", Extra(production.PredictedResources[0], "tags").GetProperty("dataClassification").GetString());
        Assert.AreEqual("Standard_GRS", Extra(production.PredictedResources[1], "sku").GetProperty("name").GetString());
        StringAssert.Contains(production.Outputs["auditStorageId"].GetString(), "/storageAccounts/ordersprodaudit");
    }

    [TestMethod]
    [Timeout(60_000)]
    public async Task Security_baseline_catches_weakened_parameters()
    {
        var secure = await SnapshotAsync(session, "security-baseline/secure.bicepparam");
        var insecure = await SnapshotAsync(session, "security-baseline/insecure.bicepparam");
        var secureStorage = Resource(secure, "Microsoft.Storage/storageAccounts");
        var secureVault = Resource(secure, "Microsoft.KeyVault/vaults");
        var insecureStorage = Resource(insecure, "Microsoft.Storage/storageAccounts");

        Assert.IsEmpty(secure.Diagnostics);
        Assert.IsFalse(secureStorage.Properties.GetProperty("allowBlobPublicAccess").GetBoolean());
        Assert.IsFalse(secureStorage.Properties.GetProperty("allowSharedKeyAccess").GetBoolean());
        Assert.AreEqual("TLS1_2", secureStorage.Properties.GetProperty("minimumTlsVersion").GetString());
        Assert.AreEqual("Disabled", secureStorage.Properties.GetProperty("publicNetworkAccess").GetString());
        Assert.IsTrue(secureVault.Properties.GetProperty("enablePurgeProtection").GetBoolean());
        Assert.IsTrue(secureVault.Properties.GetProperty("enableRbacAuthorization").GetBoolean());
        Assert.AreEqual(90, secureVault.Properties.GetProperty("softDeleteRetentionInDays").GetInt32());
        Assert.AreEqual("TLS1_0", insecureStorage.Properties.GetProperty("minimumTlsVersion").GetString());
        Assert.IsTrue(insecureStorage.Properties.GetProperty("allowBlobPublicAccess").GetBoolean());
    }

    [TestMethod]
    [Timeout(60_000)]
    public async Task Private_network_references_are_wired_together()
    {
        var snapshot = await SnapshotAsync(session, "private-network/main.bicepparam");
        var resources = snapshot.PredictedResources.ToDictionary(resource => resource.Name);
        var networkIds = snapshot.Outputs["networkIds"];

        Assert.IsEmpty(snapshot.Diagnostics);
        Assert.AreEqual("10.42.0.0/16", resources["orders-vnet"].Properties.GetProperty("addressSpace").GetProperty("addressPrefixes")[0].GetString());
        Assert.AreEqual("10.42.1.0/24", resources["orders-vnet/app"].Properties.GetProperty("addressPrefix").GetString());
        StringAssert.Contains(resources["orders-vnet/app"].Properties.GetProperty("networkSecurityGroup").GetProperty("id").GetString(), "/orders-app-nsg");
        Assert.AreEqual("Disabled", resources["orders-vnet/data"].Properties.GetProperty("privateEndpointNetworkPolicies").GetString());
        Assert.AreEqual(
            networkIds.GetProperty("dataSubnetId").GetString(),
            resources["orders-storage-pe"].Properties.GetProperty("subnet").GetProperty("id").GetString());
        var connection = resources["orders-storage-pe"].Properties.GetProperty("privateLinkServiceConnections")[0].GetProperty("properties");
        Assert.AreEqual("blob", connection.GetProperty("groupIds")[0].GetString());
        StringAssert.Contains(connection.GetProperty("privateLinkServiceId").GetString(), "/storageAccounts/ordersprivatestore");
        Assert.AreEqual(
            networkIds.GetProperty("virtualNetworkId").GetString(),
            resources["privatelink.blob.core.windows.net/orders-vnet-link"].Properties.GetProperty("virtualNetwork").GetProperty("id").GetString());
    }

    private async Task<SnapshotResult> SnapshotAsync(BicepTestSession session, string relativePath)
    {
        return await session.SnapshotAsync(
            InfraPath(relativePath),
            TenantId,
            SubscriptionId,
            "sample-rg",
            "eastus",
            "sample-deployment",
            TestContext.CancellationToken);
    }

    private static SnapshotResource Resource(SnapshotResult snapshot, string type) =>
        snapshot.PredictedResources.Single(resource => resource.Type == type);

    private static JsonElement Extra(SnapshotResource resource, string name) =>
        resource.AdditionalProperties![name];

    private static string InfraPath(string relativePath) => Path.GetFullPath(
        Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "infra", relativePath));
}