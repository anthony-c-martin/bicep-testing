package com.github.anthonycmartin.samples;

import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assumptions.assumeTrue;

import com.azure.identity.DefaultAzureCredentialBuilder;
import com.fasterxml.jackson.databind.node.TextNode;
import com.github.anthonycmartin.biceptest.BicepTestSession;
import com.github.anthonycmartin.biceptest.DeployOptions;
import com.github.anthonycmartin.biceptest.DeployResult;
import java.nio.file.Path;
import java.util.Map;
import org.junit.jupiter.api.Test;

class DeploymentTest {
    @Test
    void deploysInfrastructureAndRemovesItAfterward() throws Exception {
        String subscriptionId = requireEnvironmentVariable("AZURE_SUBSCRIPTION_ID");
        String resourceGroup = requireEnvironmentVariable("AZURE_RESOURCE_GROUP");
        String stackName = requireEnvironmentVariable("BICEP_TEST_STACK_NAME");
        String resourcePrefix = requireEnvironmentVariable("BICEP_TEST_RESOURCE_PREFIX");
        Path parameters = Path.of("..", "infra", "main.bicepparam").toAbsolutePath();
        DeployOptions options = new DeployOptions(
                parameters,
                subscriptionId,
                resourceGroup,
                stackName,
                Map.of("env", TextNode.valueOf(resourcePrefix)));

        try (BicepTestSession session = BicepTestSession.create("0.43.1");
                DeployResult deployment = session.deploy(new DefaultAzureCredentialBuilder().build(), options)) {
            assertTrue(deployment.resources().stream()
                    .anyMatch(resource -> "Microsoft.Storage/storageAccounts".equals(resource.type())));
            assertTrue(deployment.outputs().get("primaryStorageId").asText()
                    .contains("/providers/Microsoft.Storage/storageAccounts/"));
        }
    }

    private static String requireEnvironmentVariable(String name) {
        String value = System.getenv(name);
        assumeTrue(value != null && !value.isBlank(), "set " + name + " to run the live deployment sample");
        return value;
    }
}