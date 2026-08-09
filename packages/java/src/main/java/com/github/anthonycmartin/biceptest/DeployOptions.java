package com.github.anthonycmartin.biceptest;

import com.fasterxml.jackson.databind.JsonNode;
import java.nio.file.Path;
import java.util.Map;
import java.util.Objects;

/** Inputs for a resource-group Deployment Stack deployment. */
public record DeployOptions(
        Path filePath,
        String subscriptionId,
        String resourceGroup,
        String stackName,
        Map<String, JsonNode> parameterOverrides) {
    public DeployOptions {
        Objects.requireNonNull(filePath, "filePath");
        if (subscriptionId == null || subscriptionId.isBlank()
                || resourceGroup == null || resourceGroup.isBlank()
                || stackName == null || stackName.isBlank()) {
            throw new IllegalArgumentException("subscriptionId, resourceGroup, and stackName must not be empty");
        }
        parameterOverrides = parameterOverrides == null ? Map.of() : Map.copyOf(parameterOverrides);
    }

    public DeployOptions(Path filePath, String subscriptionId, String resourceGroup, String stackName) {
        this(filePath, subscriptionId, resourceGroup, stackName, Map.of());
    }
}