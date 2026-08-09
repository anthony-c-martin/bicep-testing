package com.github.anthonycmartin.biceptesting;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.azure.core.credential.AccessToken;
import com.azure.core.credential.TokenCredential;
import com.azure.resourcemanager.resources.deploymentstacks.models.DeploymentStack;
import com.azure.resourcemanager.resources.deploymentstacks.models.DeploymentStackProperties;
import com.azure.resourcemanager.resources.deploymentstacks.models.DeploymentParameter;
import com.azure.resourcemanager.resources.deploymentstacks.models.ManagedResourceReference;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.lang.reflect.Field;
import java.lang.reflect.Proxy;
import java.nio.file.Path;
import java.time.OffsetDateTime;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;
import reactor.core.publisher.Mono;

class DeploymentTest {
    private static final ObjectMapper MAPPER = new ObjectMapper();

    @Test
    void deployCompilesDeploysAndTearsDownOnce() throws Exception {
        AtomicInteger deleteCalls = new AtomicInteger();
        DeploymentStack stack = stackResult();
        DeploymentStackService service = new DeploymentStackService() {
            @Override
            public DeploymentStack deploy(
                    String resourceGroup,
                    String stackName,
                    Object template,
                    Map<String, DeploymentParameter> parameters) {
                assertEquals("resource-group", resourceGroup);
                assertEquals("stack", stackName);
                assertEquals("hello", parameters.get("message").value());
                assertEquals("password", parameters.get("secret").reference().secretName());
                assertEquals("v1", parameters.get("secret").reference().secretVersion());
                return stack;
            }

            @Override
            public void delete(String resourceGroup, String stackName) {
                deleteCalls.incrementAndGet();
            }
        };
        var originalFactory = BicepTestSession.deploymentStackServiceFactory;
        BicepTestSession.deploymentStackServiceFactory = (credential, subscription) -> service;
        try {
            FakeRpcCaller rpc = new FakeRpcCaller();
            TokenCredential credential = request -> Mono.just(
                    new AccessToken("token", OffsetDateTime.now().plusHours(1)));
            try (BicepTestSession session = new BicepTestSession(rpc)) {
                DeployResult result = session.deploy(
                        credential,
                        new DeployOptions(
                                Path.of("main.bicepparam"),
                                "subscription",
                                "resource-group",
                                "stack",
                                Map.of("message", MAPPER.valueToTree("override"))));

                assertTrue(Path.of(rpc.params.get("path").asText()).isAbsolute());
                assertEquals("override", rpc.params.get("parameterOverrides").get("message").asText());
                assertEquals("https://example.test", result.outputs().get("endpoint").asText());
                assertEquals("Microsoft.Storage/storageAccounts", result.resources().get(0).type());
                result.close();
                result.close();
            }
        } finally {
            BicepTestSession.deploymentStackServiceFactory = originalFactory;
        }
        assertEquals(1, deleteCalls.get());
    }

    private static DeploymentStack stackResult() throws Exception {
        String resourceId = "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Storage/storageAccounts/example";
        ManagedResourceReference resource = new ManagedResourceReference();
        setField(resource, "id", resourceId);
        DeploymentStackProperties properties = new DeploymentStackProperties();
        setField(properties, "outputs", Map.of("endpoint", Map.of("value", "https://example.test")));
        setField(properties, "resources", List.of(resource));
        return (DeploymentStack) Proxy.newProxyInstance(
                DeploymentStack.class.getClassLoader(),
                new Class<?>[] {DeploymentStack.class},
                (proxy, method, args) -> method.getName().equals("properties") ? properties : null);
    }

    private static void setField(Object target, String name, Object value) throws Exception {
        Field field = target.getClass().getDeclaredField(name);
        field.setAccessible(true);
        field.set(target, value);
    }

    private static final class FakeRpcCaller implements RpcCaller {
        private JsonNode params;

        @Override
        public JsonNode call(String method, JsonNode params) throws java.io.IOException {
            assertEquals("bicep/compileParams", method);
            this.params = params;
            return MAPPER.readTree("""
                    {
                      "success": true,
                      "template": "{\\"resources\\":[]}",
                                            "parameters": "{\\"parameters\\":{\\"message\\":{\\"value\\":\\"hello\\"},\\"secret\\":{\\"reference\\":{\\"keyVault\\":{\\"id\\":\\"/subscriptions/sub/resourceGroups/rg/providers/Microsoft.KeyVault/vaults/test\\"},\\"secretName\\":\\"password\\",\\"secretVersion\\":\\"v1\\"}}}}"
                    }
                    """);
        }

        @Override
        public void close() {}
    }
}