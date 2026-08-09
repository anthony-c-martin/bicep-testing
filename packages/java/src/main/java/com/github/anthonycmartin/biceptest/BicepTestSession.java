package com.github.anthonycmartin.biceptest;

import com.azure.core.credential.TokenCredential;
import com.azure.resourcemanager.resources.deploymentstacks.models.DeploymentParameter;
import com.azure.resourcemanager.resources.deploymentstacks.models.DeploymentStack;
import com.azure.resourcemanager.resources.deploymentstacks.models.KeyVaultParameterReference;
import com.azure.resourcemanager.resources.deploymentstacks.models.KeyVaultReference;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.io.IOException;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.function.BiFunction;

/** Installs and invokes a pinned Bicep CLI for infrastructure tests. */
public final class BicepTestSession implements AutoCloseable {
    private static final ObjectMapper MAPPER = new ObjectMapper();
    static BiFunction<TokenCredential, String, DeploymentStackService> deploymentStackServiceFactory =
            DeploymentStackService::create;
    private final RpcCaller client;

    BicepTestSession(RpcCaller client) {
        this.client = client;
    }

    /** Installs a Bicep CLI version if needed and starts its RPC client. */
    public static BicepTestSession create(String bicepVersion) throws IOException {
        if (bicepVersion == null || bicepVersion.isBlank()) {
            throw new IllegalArgumentException("bicepVersion must not be empty");
        }
        RpcClient client = new RpcClient(BicepInstaller.install(bicepVersion));
        String version = client.call("bicep/version", MAPPER.createObjectNode()).get("version").asText();
        if (compareVersions(version, "0.36.1") < 0) {
            client.close();
            throw new IOException("Bicep CLI 0.36.1 or later is required; detected " + version);
        }
        return new BicepTestSession(client);
    }

    /** Evaluates a Bicep parameters file without deploying it. */
    public SnapshotResult snapshot(Path filePath, SnapshotMetadata metadata) throws IOException {
        Objects.requireNonNull(filePath, "filePath");
        ObjectNode request = MAPPER.createObjectNode();
        request.put("path", filePath.toAbsolutePath().normalize().toString());
        request.set("metadata", MAPPER.valueToTree(metadata == null ? SnapshotMetadata.builder().build().toRpcMap() : metadata.toRpcMap()));
        JsonNode response = client.call("bicep/getSnapshot", request);
        return MAPPER.readValue(response.get("snapshot").asText(), SnapshotResult.class);
    }

    /** Compiles and deploys a Bicep parameters file as a resource-group Deployment Stack. */
    public DeployResult deploy(TokenCredential credential, DeployOptions options) throws IOException {
        Objects.requireNonNull(credential, "credential");
        Objects.requireNonNull(options, "options");
        ObjectNode request = MAPPER.createObjectNode();
        request.put("path", options.filePath().toAbsolutePath().normalize().toString());
        request.set("parameterOverrides", MAPPER.valueToTree(options.parameterOverrides()));
        JsonNode compilation = client.call("bicep/compileParams", request);
        if (!compilation.path("success").asBoolean()
            || !compilation.hasNonNull("template")
            || !compilation.hasNonNull("parameters")) {
            throw new IOException("Bicep parameter compilation failed: " + compilation.path("diagnostics"));
        }

        Object template = MAPPER.convertValue(
            MAPPER.readTree(compilation.get("template").asText()), Object.class);
        JsonNode parameterFile = MAPPER.readTree(compilation.get("parameters").asText());
        Map<String, DeploymentParameter> parameters = new LinkedHashMap<>();
        parameterFile.path("parameters").fields().forEachRemaining(entry -> {
            JsonNode parameter = entry.getValue();
            DeploymentParameter deploymentParameter = new DeploymentParameter();
            if (parameter.has("value")) {
                deploymentParameter.withValue(MAPPER.convertValue(parameter.get("value"), Object.class));
            } else if (parameter.has("reference")) {
                JsonNode reference = parameter.get("reference");
                KeyVaultParameterReference keyVaultReference = new KeyVaultParameterReference()
                    .withKeyVault(new KeyVaultReference().withId(reference.path("keyVault").path("id").asText()))
                    .withSecretName(reference.path("secretName").asText());
                if (reference.has("secretVersion")) {
                    keyVaultReference.withSecretVersion(reference.get("secretVersion").asText());
                }
                deploymentParameter.withReference(keyVaultReference);
            }
            parameters.put(entry.getKey(), deploymentParameter);
        });

        DeploymentStackService service = deploymentStackServiceFactory.apply(credential, options.subscriptionId());
        DeploymentStack stack = service.deploy(options.resourceGroup(), options.stackName(), template, parameters);
        Map<String, JsonNode> outputs = new LinkedHashMap<>();
        JsonNode outputNode = MAPPER.valueToTree(stack.properties().outputs());
        outputNode.fields().forEachRemaining(entry -> outputs.put(
            entry.getKey(), entry.getValue().has("value") ? entry.getValue().get("value") : entry.getValue()));
        List<DeploymentResource> resources = new ArrayList<>();
        if (stack.properties().resources() != null) {
            stack.properties().resources().stream()
                .filter(resource -> resource.id() != null)
                .map(resource -> new DeploymentResource(resource.id(), resourceType(resource.id())))
                .forEach(resources::add);
    }
        return new DeployResult(
            outputs,
            resources,
            () -> service.delete(options.resourceGroup(), options.stackName()));
        }

    @Override
    public void close() {
        client.close();
    }

    private static int compareVersions(String first, String second) {
        String[] firstParts = first.split("\\.");
        String[] secondParts = second.split("\\.");
        for (int index = 0; index < Math.max(firstParts.length, secondParts.length); index++) {
            int firstPart = index < firstParts.length ? numericPrefix(firstParts[index]) : 0;
            int secondPart = index < secondParts.length ? numericPrefix(secondParts[index]) : 0;
            if (firstPart != secondPart) {
                return Integer.compare(firstPart, secondPart);
            }
        }
        return 0;
    }

    private static int numericPrefix(String value) {
        int end = 0;
        while (end < value.length() && Character.isDigit(value.charAt(end))) {
            end++;
        }
        return end == 0 ? 0 : Integer.parseInt(value.substring(0, end));
    }

    private static String resourceType(String resourceId) {
        String[] parts = resourceId.replaceAll("^/+|/+$", "").split("/");
        for (int index = 0; index < parts.length; index++) {
            if (parts[index].equalsIgnoreCase("providers") && index + 2 < parts.length) {
                List<String> typeParts = new ArrayList<>();
                typeParts.add(parts[index + 1]);
                for (int resourceIndex = index + 2; resourceIndex < parts.length; resourceIndex += 2) {
                    typeParts.add(parts[resourceIndex]);
                }
                return String.join("/", typeParts);
            }
        }
        return null;
    }
}