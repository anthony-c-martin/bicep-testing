using Microsoft.VisualStudio.TestTools.UnitTesting;
using System.Text.Json;

namespace AnthonyCMartin.BicepTesting.Tests;

[TestClass]
public sealed class SnapshotHelperTests
{
    private static string Infra(string relative) => Path.GetFullPath(Path.Combine(
        AppContext.BaseDirectory, "../../../../../../../samples/infra", relative));

    [TestMethod]
    public void Helpers_match_shared_edge_cases()
    {
        using var cases = JsonDocument.Parse(File.ReadAllText(Infra("snapshot-scopes/helper-cases.json")));
        foreach (var item in cases.RootElement.GetProperty("scopes").EnumerateArray())
        {
            var scope = SnapshotHelpers.GetResourceScope(item.GetProperty("id").GetString()!);
            Assert.AreEqual(Optional(item, "subscriptionId"), scope.SubscriptionId);
            Assert.AreEqual(Optional(item, "resourceGroup"), scope.ResourceGroup);
            Assert.AreEqual(Optional(item, "managementGroupId"), scope.ManagementGroupId);
        }
        foreach (var id in cases.RootElement.GetProperty("invalidIds").EnumerateArray())
        {
            Assert.ThrowsExactly<ArgumentException>(() => SnapshotHelpers.GetResourceScope(id.GetString()!));
        }
        var document = cases.RootElement.GetProperty("document");
        foreach (var item in cases.RootElement.GetProperty("pointers").EnumerateArray())
        {
            var actual = SnapshotHelpers.GetSnapshotValue(document, item.GetProperty("pointer").GetString()!);
            Assert.IsTrue(JsonElement.DeepEquals(item.GetProperty("value"), actual));
        }
        foreach (var pointer in cases.RootElement.GetProperty("invalidPointers").EnumerateArray())
        {
            Assert.Throws<Exception>(() => SnapshotHelpers.GetSnapshotValue(document, pointer.GetString()!));
        }
        Assert.IsTrue(JsonElement.DeepEquals(document, SnapshotHelpers.GetSnapshotValue(document, "")));
        var empty = new SnapshotResult();
        Assert.HasCount(0, empty.ResourcesOfType("Missing/type"));
        Assert.ThrowsExactly<ArgumentException>(() => empty.ResourcesOfType(""));
        Assert.ThrowsExactly<InvalidOperationException>(() => empty.RequireResource("/missing"));
        var resource = new SnapshotResource { Id = "/id", Type = "Test/type", Name = "test" };
        var duplicate = new SnapshotResult { PredictedResources = [resource, resource] };
        Assert.ThrowsExactly<InvalidOperationException>(() => duplicate.RequireResource("/ID"));
        Assert.ThrowsExactly<InvalidOperationException>(() => duplicate.RequireResource("test/TYPE", "test"));
        Assert.ThrowsExactly<InvalidOperationException>(() => duplicate.RequireResource("Test/type", "TEST"));
    }

    [TestMethod]
    [Timeout(120_000)]
    public async Task Snapshots_support_every_scope_and_optional_context()
    {
        await using var session = await BicepTestSession.CreateAsync("0.47.16");
        var snapshot = await session.SnapshotAsync(new SnapshotOptions
        {
            FilePath = Infra("snapshot-scopes/management-group.bicepparam"),
            ManagementGroupId = "platform", Location = "eastus", DeploymentName = "offline-test",
        });
        Assert.HasCount(0, snapshot.Diagnostics);
        Assert.AreEqual("/providers/Microsoft.Management/managementGroups/platform", snapshot.Outputs["scopeId"].GetString());
        Assert.AreEqual("offline-test", snapshot.Outputs["deploymentName"].GetString());
        Assert.HasCount(1, snapshot.ResourcesOfType("microsoft.authorization/POLICYDEFINITIONS"));
        var policy = snapshot.RequireResource("Microsoft.Authorization/policyDefinitions", "require-cost-center");
        Assert.AreSame(policy, snapshot.RequireResource(policy.Id.ToUpperInvariant()));
        Assert.AreEqual("platform", SnapshotHelpers.GetResourceScope(policy.Id).ManagementGroupId);
        Assert.AreEqual("audit", SnapshotHelpers.GetSnapshotValue(policy.Properties, "/policyRule/then/effect").GetString());
        var subscription = await session.SnapshotAsync(new SnapshotOptions
        {
            FilePath = Infra("snapshot-scopes/subscription.bicepparam"),
            SubscriptionId = "28c9069e-23e8-47d2-b640-00d2e0f09616",
        });
        Assert.HasCount(0, subscription.Diagnostics);
        Assert.AreEqual("/subscriptions/28c9069e-23e8-47d2-b640-00d2e0f09616", subscription.Outputs["scopeId"].GetString());
        var tenant = await session.SnapshotAsync(new SnapshotOptions
        {
            FilePath = Infra("snapshot-scopes/tenant.bicepparam"),
            TenantId = "ddbe463a-0554-485d-b589-0b17d60cd38b",
        });
        Assert.HasCount(0, tenant.Diagnostics);
        Assert.AreEqual("ddbe463a-0554-485d-b589-0b17d60cd38b", tenant.Outputs["tenantId"].GetString());
        var minimal = await session.SnapshotAsync(new SnapshotOptions { FilePath = Infra("security-baseline/secure.bicepparam") });
        Assert.HasCount(0, minimal.Diagnostics);
    }

    [TestMethod]
    public async Task Older_CLIs_and_empty_management_group_context_fail_explicitly()
    {
        await using var session = await BicepTestSession.CreateAsync("0.46.1");
        await Assert.ThrowsExactlyAsync<NotSupportedException>(() => session.SnapshotAsync(new SnapshotOptions
        {
            FilePath = Infra("snapshot-scopes/management-group.bicepparam"), ManagementGroupId = "platform",
        }));
        await Assert.ThrowsExactlyAsync<ArgumentException>(() => session.SnapshotAsync(new SnapshotOptions
        {
            FilePath = "unused", ManagementGroupId = " ",
        }));
    }

    private static string? Optional(JsonElement value, string name) =>
        value.TryGetProperty(name, out var property) ? property.GetString() : null;
}
