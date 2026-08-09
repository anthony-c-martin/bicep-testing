using Microsoft.VisualStudio.TestTools.UnitTesting;

namespace AnthonyCMartin.BicepTesting.Sample;

[TestClass]
public sealed class SnapshotTests
{
    public TestContext TestContext { get; set; } = null!;

    [TestMethod]
    [Timeout(60_000)]
    public async Task Infrastructure_has_expected_resources_and_no_diagnostics()
    {
        var parametersPath = Path.GetFullPath(
            Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "infra", "main.bicepparam"));

        await using var session = await AnthonyCMartin.BicepTesting.BicepTestSession.CreateAsync("0.43.1", TestContext.CancellationToken);
        var snapshot = await session.SnapshotAsync(
            parametersPath,
            "00000000-0000-0000-0000-000000000000",
            "00000000-0000-0000-0000-000000000000",
            "sample-rg",
            "eastus",
            "sample-deployment",
            TestContext.CancellationToken);

        Assert.IsEmpty(snapshot.Diagnostics);
        Assert.HasCount(3, snapshot.PredictedResources);
        Assert.IsTrue(snapshot.PredictedResources.Any(resource =>
            resource.Type == "Microsoft.Storage/storageAccounts" && resource.Name == "sampleprimary"));
        Assert.IsTrue(snapshot.PredictedResources.Any(resource =>
            resource.Type == "Microsoft.Storage/storageAccounts" && resource.Name == "samplebackup"));
        Assert.IsTrue(snapshot.PredictedResources.Any(resource =>
            resource.Type == "Microsoft.KeyVault/vaults" && resource.Name == "samplekv"));
    }
}