using Azure.ResourceManager.Resources.DeploymentStacks.Models;
using Azure;
using Azure.Core;
using Microsoft.VisualStudio.TestTools.UnitTesting;
using System.ClientModel.Primitives;
using System.Text.Json;

namespace AnthonyCMartin.BicepTesting.Tests;

[TestClass]
public sealed class LiveTests
{
  [TestMethod]
  public void DeployOptions_generates_a_unique_stack_name_by_default()
  {
    var first = CreateDeployOptions();
    var second = CreateDeployOptions();

    StringAssert.StartsWith(first.StackName, "bicep-test-");
    Assert.AreNotEqual(first.StackName, second.StackName);
  }

  [TestMethod]
  [DataRow(null, "28c9069e-23e8-47d2-b640-00d2e0f09616", "test-rg", null,
    "/subscriptions/28c9069e-23e8-47d2-b640-00d2e0f09616/resourceGroups/test-rg")]
  [DataRow(null, "28c9069e-23e8-47d2-b640-00d2e0f09616", null, "eastus",
    "/subscriptions/28c9069e-23e8-47d2-b640-00d2e0f09616")]
  [DataRow("test-mg", null, null, "eastus",
    "/providers/Microsoft.Management/managementGroups/test-mg")]
  public void DeployOptions_accepts_supported_scopes(
    string? managementGroupId,
    string? subscriptionId,
    string? resourceGroup,
    string? location,
    string expectedScope)
  {
    var options = new DeployOptions
    {
      FilePath = "main.bicepparam",
      ManagementGroupId = managementGroupId,
      SubscriptionId = subscriptionId,
      ResourceGroup = resourceGroup,
      Location = location,
    };

    BicepTestSession.ValidateDeployOptions(options);
    Assert.AreEqual(expectedScope, BicepTestSession.BuildDeploymentScope(options).ToString());
  }

  [TestMethod]
  public void DeployOptions_requires_location_outside_resource_group_scope()
  {
    var options = new DeployOptions
    {
      FilePath = "main.bicepparam",
      SubscriptionId = "28c9069e-23e8-47d2-b640-00d2e0f09616",
    };

    Assert.ThrowsExactly<ArgumentNullException>(() => BicepTestSession.ValidateDeployOptions(options));
  }

  [TestMethod]
  public void DeployOptions_rejects_management_group_with_subscription_scope()
  {
    var options = new DeployOptions
    {
      FilePath = "main.bicepparam",
      ManagementGroupId = "test-mg",
      SubscriptionId = "28c9069e-23e8-47d2-b640-00d2e0f09616",
      Location = "eastus",
    };

    Assert.ThrowsExactly<ArgumentException>(() => BicepTestSession.ValidateDeployOptions(options));
  }

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

        [TestMethod]
        public void ValidateResult_preserves_correlation_and_validated_resources()
        {
          var resource = ArmResourcesDeploymentStacksModelFactory.DeploymentStackResourceReference(
            new ResourceIdentifier("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Storage/storageAccounts/test"),
            type: new ResourceType("Microsoft.Storage/storageAccounts"));
          var properties = ArmResourcesDeploymentStacksModelFactory.DeploymentStackValidateProperties(
            correlationId: "00000000-0000-0000-0000-000000000001",
            validatedResources: [resource]);
          var validation = ArmResourcesDeploymentStacksModelFactory.DeploymentStackValidateResult(
            new ResourceIdentifier("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Resources/deploymentStacks/test"),
            "test",
            new ResourceType("Microsoft.Resources/deploymentStacks"),
            systemData: null,
            error: null,
            properties);

          var result = ValidateResult.FromValidation(validation);

          Assert.IsTrue(result.IsValid);
          Assert.IsNull(result.ErrorCode);
          Assert.IsNull(result.ErrorMessage);
          Assert.AreEqual("00000000-0000-0000-0000-000000000001", result.CorrelationId);
          Assert.HasCount(1, result.Resources);
          Assert.AreEqual(resource.Id.ToString(), result.Resources[0].Id);
          Assert.AreEqual("Microsoft.Storage/storageAccounts", result.Resources[0].Type);
        }

        [TestMethod]
        public void ValidateResult_exposes_validation_errors()
        {
          var responseError = ModelReaderWriter.Read<ResponseError>(BinaryData.FromString("""
            {
              "code": "InvalidTemplate",
              "message": "The template is invalid.",
              "target": "resources[0]",
              "details": [
                {
                  "code": "InvalidResource",
                  "message": "The resource is invalid."
                }
              ]
            }
            """));
          var validation = ArmResourcesDeploymentStacksModelFactory.DeploymentStackValidateResult(
            new ResourceIdentifier("/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Resources/deploymentStacks/test"),
            "test",
            new ResourceType("Microsoft.Resources/deploymentStacks"),
            systemData: null,
            error: responseError,
            ArmResourcesDeploymentStacksModelFactory.DeploymentStackValidateProperties());

          var result = ValidateResult.FromValidation(validation);

          Assert.IsFalse(result.IsValid);
          Assert.AreEqual("InvalidTemplate", result.ErrorCode);
          Assert.AreEqual("The template is invalid.", result.ErrorMessage);
          Assert.IsNotNull(result.Error);
          using var errorDocument = JsonDocument.Parse(result.Error.RawData);
          var error = errorDocument.RootElement;
          Assert.AreEqual("resources[0]", error.GetProperty("target").GetString());
          var details = error.GetProperty("details");
          Assert.AreEqual("InvalidResource", details[0].GetProperty("code").GetString());
          Assert.AreEqual("The resource is invalid.", details[0].GetProperty("message").GetString());
        }

        private static DeployOptions CreateDeployOptions() => new()
        {
          FilePath = "main.bicepparam",
          SubscriptionId = "28c9069e-23e8-47d2-b640-00d2e0f09616",
          ResourceGroup = "test-rg",
        };

}