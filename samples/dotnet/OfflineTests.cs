using AnthonyCMartin.BicepTesting;
using Microsoft.VisualStudio.TestTools.UnitTesting;

namespace Samples;

[TestClass]
public sealed class OfflineTests
{
    private static BicepTestSession session = null!;

    public TestContext TestContext { get; set; } = null!;

    [ClassInitialize]
    public static async Task ClassInitialize(TestContext testContext)
    {
        session = await BicepTestSession.CreateAsync("0.47.16", testContext.CancellationToken);
    }

    [ClassCleanup]
    public static async Task ClassCleanup()
    {
        await session.DisposeAsync();
    }

    [TestMethod]
    [Timeout(60_000)]
    public async Task Storage_and_Key_Vault_meet_the_security_baseline()
    {
        // Context is unnecessary when the assertions only depend on explicit parameter values.
        var snapshot = await session.SnapshotAsync(new SnapshotOptions
        {
            FilePath = InfraPath("security-baseline/secure.bicepparam"),
        }, TestContext.CancellationToken);
        Assert.IsEmpty(snapshot.Diagnostics);
        var storage = snapshot.RequireResource("Microsoft.Storage/storageAccounts", "orderssecurestore");
        var vault = snapshot.RequireResource("Microsoft.KeyVault/vaults", "orders-secure-kv");

        Assert.IsFalse(storage.Properties.GetProperty("allowBlobPublicAccess").GetBoolean());
        Assert.IsFalse(storage.Properties.GetProperty("allowSharedKeyAccess").GetBoolean());
        Assert.AreEqual("TLS1_2", storage.Properties.GetProperty("minimumTlsVersion").GetString());
        Assert.AreEqual("Disabled", storage.Properties.GetProperty("publicNetworkAccess").GetString());
        Assert.IsTrue(storage.Properties.GetProperty("supportsHttpsTrafficOnly").GetBoolean());
        Assert.IsTrue(vault.Properties.GetProperty("enablePurgeProtection").GetBoolean());
        Assert.IsTrue(vault.Properties.GetProperty("enableRbacAuthorization").GetBoolean());
        Assert.AreEqual("Disabled", vault.Properties.GetProperty("publicNetworkAccess").GetString());
        Assert.AreEqual(90, vault.Properties.GetProperty("softDeleteRetentionInDays").GetInt32());
    }

    [TestMethod]
    [Timeout(60_000)]
    public async Task Development_uses_inexpensive_storage_without_an_audit_account()
    {
        var snapshot = await SnapshotInResourceGroupAsync("environment-topology/dev.bicepparam");
        Assert.IsEmpty(snapshot.Diagnostics);
        Assert.HasCount(1, snapshot.ResourcesOfType("Microsoft.Storage/storageAccounts"));
        var primary = snapshot.RequireResource("Microsoft.Storage/storageAccounts", "ordersdevprimary");

        Assert.AreEqual("Standard_LRS", primary.AdditionalProperties!["sku"].GetProperty("name").GetString());
        var tags = primary.AdditionalProperties["tags"];
        Assert.AreEqual("dev", tags.GetProperty("environment").GetString());
        Assert.AreEqual("internal", tags.GetProperty("dataClassification").GetString());
        Assert.AreEqual(primary.Id, snapshot.Outputs["primaryStorageId"].GetString());
        Assert.AreEqual(System.Text.Json.JsonValueKind.Null, snapshot.Outputs["auditStorageId"].ValueKind);
    }

    [TestMethod]
    [Timeout(60_000)]
    public async Task Production_uses_resilient_primary_and_audit_storage()
    {
        var snapshot = await SnapshotInResourceGroupAsync("environment-topology/prod.bicepparam");
        Assert.IsEmpty(snapshot.Diagnostics);
        Assert.HasCount(2, snapshot.ResourcesOfType("Microsoft.Storage/storageAccounts"));
        var primary = snapshot.RequireResource("Microsoft.Storage/storageAccounts", "ordersprodprimary");
        var audit = snapshot.RequireResource("Microsoft.Storage/storageAccounts", "ordersprodaudit");

        Assert.AreEqual("Standard_ZRS", primary.AdditionalProperties!["sku"].GetProperty("name").GetString());
        Assert.AreEqual("Standard_GRS", audit.AdditionalProperties!["sku"].GetProperty("name").GetString());
        var tags = primary.AdditionalProperties["tags"];
        Assert.AreEqual("prod", tags.GetProperty("environment").GetString());
        Assert.AreEqual("confidential", tags.GetProperty("dataClassification").GetString());
        Assert.AreEqual(primary.Id, snapshot.Outputs["primaryStorageId"].GetString());
        Assert.AreEqual(audit.Id, snapshot.Outputs["auditStorageId"].GetString());
    }

    [TestMethod]
    [Timeout(60_000)]
    public async Task Private_storage_connects_to_the_data_subnet_and_private_DNS()
    {
        var snapshot = await SnapshotInResourceGroupAsync("private-network/main.bicepparam");
        Assert.IsEmpty(snapshot.Diagnostics);
        var networkIds = snapshot.Outputs["networkIds"];
        var network = snapshot.RequireResource(networkIds.GetProperty("virtualNetworkId").GetString()!);
        var appSubnet = snapshot.RequireResource(networkIds.GetProperty("appSubnetId").GetString()!);
        var dataSubnet = snapshot.RequireResource(networkIds.GetProperty("dataSubnetId").GetString()!);
        var endpoint = snapshot.RequireResource(networkIds.GetProperty("privateEndpointId").GetString()!);
        var storage = snapshot.RequireResource("Microsoft.Storage/storageAccounts", "ordersprivatestore");
        var appNsg = snapshot.RequireResource("Microsoft.Network/networkSecurityGroups", "orders-app-nsg");
        var dataNsg = snapshot.RequireResource("Microsoft.Network/networkSecurityGroups", "orders-data-nsg");
        var dnsLink = snapshot.RequireResource(
            "Microsoft.Network/privateDnsZones/virtualNetworkLinks",
            "privatelink.blob.core.windows.net/orders-vnet-link");

        Assert.AreEqual("10.42.0.0/16", SnapshotHelpers.GetSnapshotValue(network.Properties, "/addressSpace/addressPrefixes/0").GetString());
        Assert.AreEqual("10.42.1.0/24", appSubnet.Properties.GetProperty("addressPrefix").GetString());
        Assert.AreEqual(appNsg.Id, SnapshotHelpers.GetSnapshotValue(appSubnet.Properties, "/networkSecurityGroup/id").GetString());
        Assert.AreEqual("10.42.2.0/24", dataSubnet.Properties.GetProperty("addressPrefix").GetString());
        Assert.AreEqual(dataNsg.Id, SnapshotHelpers.GetSnapshotValue(dataSubnet.Properties, "/networkSecurityGroup/id").GetString());
        Assert.AreEqual("Disabled", dataSubnet.Properties.GetProperty("privateEndpointNetworkPolicies").GetString());
        Assert.AreEqual(dataSubnet.Id, SnapshotHelpers.GetSnapshotValue(endpoint.Properties, "/subnet/id").GetString());
        var connection = SnapshotHelpers.GetSnapshotValue(endpoint.Properties, "/privateLinkServiceConnections/0/properties");
        Assert.AreEqual("blob", connection.GetProperty("groupIds")[0].GetString());
        Assert.AreEqual(storage.Id, connection.GetProperty("privateLinkServiceId").GetString());
        Assert.AreEqual(network.Id, SnapshotHelpers.GetSnapshotValue(dnsLink.Properties, "/virtualNetwork/id").GetString());
    }

    [TestMethod]
    [Timeout(60_000)]
    public async Task Platform_policy_audits_resources_missing_a_cost_center_tag()
    {
        var snapshot = await session.SnapshotAsync(new SnapshotOptions
        {
            FilePath = InfraPath("snapshot-scopes/management-group.bicepparam"),
            ManagementGroupId = "sample-platform",
            Location = "eastus",
        }, TestContext.CancellationToken);
        Assert.IsEmpty(snapshot.Diagnostics);
        var policy = snapshot.RequireResource("Microsoft.Authorization/policyDefinitions", "require-cost-center");

        Assert.AreEqual("sample-platform", SnapshotHelpers.GetResourceScope(policy.Id).ManagementGroupId);
        var condition = SnapshotHelpers.GetSnapshotValue(policy.Properties, "/policyRule/if");
        Assert.AreEqual("tags[costCenter]", condition.GetProperty("field").GetString());
        Assert.AreEqual("false", condition.GetProperty("exists").GetString());
        Assert.AreEqual("audit", SnapshotHelpers.GetSnapshotValue(policy.Properties, "/policyRule/then/effect").GetString());
        Assert.AreEqual(policy.Id, snapshot.Outputs["policyId"].GetString());
    }

    private Task<SnapshotResult> SnapshotInResourceGroupAsync(string relativePath) =>
        session.SnapshotAsync(new SnapshotOptions
        {
            FilePath = InfraPath(relativePath),
            SubscriptionId = "28c9069e-23e8-47d2-b640-00d2e0f09616",
            ResourceGroup = "sample-rg",
            Location = "eastus",
        }, TestContext.CancellationToken);

    private static string InfraPath(string relativePath) => Path.GetFullPath(
        Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "infra", relativePath));
}
