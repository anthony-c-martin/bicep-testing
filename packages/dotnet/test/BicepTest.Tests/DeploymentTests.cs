using Azure.ResourceManager.Resources.DeploymentStacks.Models;
using Microsoft.VisualStudio.TestTools.UnitTesting;

namespace AnthonyCMartin.BicepTesting.Tests;

[TestClass]
public sealed class DeploymentTests
{
    [TestMethod]
    public void BuildDeploymentStackData_preserves_parameters_and_cleanup_policy()
    {
        const string parameters = """
            {
              "parameters": {
                "environment": { "value": "test" },
                "optionalValue": { "value": null }
              }
            }
            """;

        var stackData = BicepTestSession.BuildDeploymentStackData("{ \"resources\": [] }", parameters);

        Assert.AreEqual(UnmanageActionResourceMode.Delete, stackData.ActionOnUnmanage.Resources);
        Assert.AreEqual(UnmanageActionResourceGroupMode.Delete, stackData.ActionOnUnmanage.ResourceGroups);
        Assert.AreEqual(UnmanageActionManagementGroupMode.Delete, stackData.ActionOnUnmanage.ManagementGroups);
        Assert.AreEqual(
            ResourcesWithoutDeleteSupportAction.Fail,
            stackData.ActionOnUnmanage.ResourcesWithoutDeleteSupport);
        Assert.AreEqual("\"test\"", stackData.Parameters["environment"].Value?.ToString());
        Assert.AreEqual("null", stackData.Parameters["optionalValue"].Value?.ToString());
    }

    [TestMethod]
    public void BuildDeploymentStackData_rejects_incomplete_key_vault_references()
    {
        const string parameters = """
            {
              "parameters": {
                "secret": {
                  "reference": {
                    "keyVault": { "id": "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/test" }
                  }
                }
              }
            }
            """;

        Assert.ThrowsExactly<InvalidDataException>(
            () => BicepTestSession.BuildDeploymentStackData("{}", parameters));
    }
}