package com.github.anthonycmartin.samples;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.github.anthonycmartin.biceptesting.BicepTestSession;
import com.github.anthonycmartin.biceptesting.SnapshotMetadata;
import com.github.anthonycmartin.biceptesting.SnapshotResource;
import com.github.anthonycmartin.biceptesting.SnapshotResult;
import java.nio.file.Path;
import java.util.Set;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

class SnapshotTest {
    @Test
    void evaluatesInfrastructureWithoutDeploying() throws Exception {
        Path parameters = Path.of("..", "infra", "main.bicepparam").toAbsolutePath();
        SnapshotMetadata metadata = SnapshotMetadata.builder()
                .tenantId("00000000-0000-0000-0000-000000000000")
                .subscriptionId("00000000-0000-0000-0000-000000000000")
                .resourceGroup("sample-rg")
                .location("eastus")
                .deploymentName("sample-deployment")
                .build();

        try (BicepTestSession session = BicepTestSession.create("0.43.1")) {
            SnapshotResult snapshot = session.snapshot(parameters, metadata);
            assertTrue(snapshot.diagnostics().isEmpty());
            assertEquals(
                    Set.of("sampleprimary", "samplebackup", "samplekv"),
                    snapshot.predictedResources().stream().map(SnapshotResource::getName).collect(Collectors.toSet()));
        }
    }
}