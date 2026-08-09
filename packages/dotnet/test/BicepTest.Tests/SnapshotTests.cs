using Microsoft.VisualStudio.TestTools.UnitTesting;

namespace BicepTest.Tests;

[TestClass]
public sealed class SnapshotTests
{
    private const string TenantId = "00000000-0000-0000-0000-000000000000";
    private const string SubscriptionId = "00000000-0000-0000-0000-000000000000";
    private const string ResourceGroup = "test-rg";
    private const string Location = "eastus";
    private const string DeploymentName = "test-deployment";

    public TestContext TestContext { get; set; } = null!;

    [TestMethod]
    [Timeout(60_000)]
    public async Task Snapshot_matches_reference_behavior()
    {
        await using var session = await BicepTestSession.CreateAsync(
            "0.43.1",
            TestContext.CancellationToken);
        var snapshot = await session.SnapshotAsync(
            GetFixturePath(),
            TenantId,
            SubscriptionId,
            ResourceGroup,
            Location,
            DeploymentName,
            TestContext.CancellationToken);

        Assert.HasCount(0, snapshot.Diagnostics);

        var storageAccounts = snapshot.PredictedResources
            .Where(resource => resource.Type == "Microsoft.Storage/storageAccounts")
            .ToArray();
        var keyVaults = snapshot.PredictedResources
            .Where(resource => resource.Type == "Microsoft.KeyVault/vaults")
            .ToArray();

        Assert.HasCount(2, storageAccounts);
        Assert.HasCount(1, keyVaults);
        Assert.IsFalse(snapshot.PredictedResources.Any(
            resource => resource.Type == "Microsoft.Network/virtualNetworks"));
        foreach (var resource in storageAccounts)
        {
            Assert.IsFalse(resource.Properties.GetProperty("allowBlobPublicAccess").GetBoolean());
            Assert.AreEqual("TLS1_2", resource.Properties.GetProperty("minimumTlsVersion").GetString());
            Assert.AreEqual(Location, resource.Location, ignoreCase: true);
        }

        foreach (var resource in keyVaults)
        {
            Assert.IsTrue(resource.Properties.GetProperty("enableSoftDelete").GetBoolean());
            Assert.AreEqual(90, resource.Properties.GetProperty("softDeleteRetentionInDays").GetInt32());
        }

        foreach (var resource in snapshot.PredictedResources)
        {
            Assert.AreEqual(Location, resource.Location, ignoreCase: true);
        }

        Assert.IsTrue(storageAccounts.Any(resource => resource.Name == "testprimary"));
        Assert.IsTrue(storageAccounts.Any(resource => resource.Name == "testbackup"));

        var primaryStorageId = snapshot.Outputs["primaryStorageId"].GetString();
        Assert.AreEqual(
            $"/subscriptions/{SubscriptionId}/resourceGroups/{ResourceGroup}/providers/Microsoft.Storage/storageAccounts/testprimary",
            primaryStorageId);
    }

    private static string GetFixturePath()
    {
        return Path.GetFullPath(
            Path.Combine(
                AppContext.BaseDirectory,
                "..",
                "..",
                "..",
                "..",
                "..",
                "..",
                "node",
                "test",
                "samples",
                "snapshot",
                "main.bicepparam"));
    }
}