# Java

The Java library provides typed helpers for testing the predicted resources, outputs, and diagnostics of a Bicep deployment without deploying to Azure.

## Requirements

- JDK 17 or later
- Maven 3.9 or later when building from source
- A `.bicepparam` entry point for the Bicep deployment under test

## Installation

Until `com.github.anthonycmartin:bicep-testing` is published to Maven Central, install it from a local checkout:

```sh
mvn --file packages/java/pom.xml install
```

## Usage

Use try-with-resources so the Bicep JSON-RPC process is always closed:

```java
import com.github.anthonycmartin.biceptesting.BicepTestSession;
import com.github.anthonycmartin.biceptesting.SnapshotMetadata;
import com.github.anthonycmartin.biceptesting.SnapshotResult;

SnapshotMetadata metadata = SnapshotMetadata.builder()
        .subscriptionId("00000000-0000-0000-0000-000000000000")
        .resourceGroup("my-resource-group")
        .location("eastus")
        .deploymentName("my-deployment")
        .build();

try (BicepTestSession session = BicepTestSession.create("0.43.1")) {
        SnapshotResult snapshot = session.snapshot(Path.of("infra/main.bicepparam"), metadata);
    boolean allPrivate = snapshot.predictedResources().stream()
            .filter(resource -> resource.getType().equals("Microsoft.Storage/storageAccounts"))
            .allMatch(resource -> !resource.getProperties().get("allowBlobPublicAccess").asBoolean());
    assertTrue(allPrivate);
}
```

`BicepTestSession.create` downloads the requested Bicep CLI version into `~/.bicep/bin` and reuses it on later runs. Snapshot tests do not require Azure credentials or an Azure subscription.

## Snapshot result

`SnapshotResult` contains immutable lists of predicted resources and diagnostics plus resolved outputs. `SnapshotResource` exposes resource identity, type, API version, location, properties, and additional fields returned by Bicep.

## Live deployment tests

Use `deploy` with an Azure `TokenCredential` when a test needs real resources or service behavior. Try-with-resources guarantees stack cleanup:

```java
TokenCredential credential = new DefaultAzureCredentialBuilder().build();
DeployOptions options = new DeployOptions(
        Path.of("infra/main.bicepparam"),
        subscriptionId,
        resourceGroup,
        "storage-test-" + UUID.randomUUID());

try (DeployResult deployment = session.deploy(credential, options)) {
    assertTrue(deployment.resources().stream()
            .anyMatch(resource -> resource.type().equals("Microsoft.Storage/storageAccounts")));
    URI endpoint = URI.create(deployment.outputs().get("endpoint").asText());
    assertEquals(200, httpClient.send(request(endpoint), BodyHandlers.discarding()).statusCode());
}
```

The result exposes normalized outputs and immutable managed-resource data. `close()` is idempotent and deletes the Deployment Stack and its managed resources. Live tests require an existing resource group, Azure credentials, and deployment/deletion permissions.

## Sample

See the runnable [JUnit sample](../samples/java/src/test/java/com/github/anthonycmartin/samples/SnapshotTest.java) for a complete consumer test using the shared example infrastructure.

## Public API

The complete exported Java API is available in [`api/java/bicep-testing.txt`](../api/java/bicep-testing.txt).