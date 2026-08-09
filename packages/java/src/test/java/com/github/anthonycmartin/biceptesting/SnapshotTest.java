package com.github.anthonycmartin.biceptesting;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.nio.file.Path;
import java.util.Set;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

class SnapshotTest {
    @Test
    void snapshotMatchesReferenceBehavior() throws Exception {
        Path fixture = Path.of("..", "..", "samples", "infra", "main.bicepparam").toAbsolutePath();
        SnapshotMetadata metadata = SnapshotMetadata.builder()
                .tenantId("00000000-0000-0000-0000-000000000000")
                .subscriptionId("00000000-0000-0000-0000-000000000000")
                .resourceGroup("sample-rg")
                .location("eastus")
                .deploymentName("sample-deployment")
                .build();

        try (BicepTestSession session = BicepTestSession.create("0.43.1")) {
            SnapshotResult snapshot = session.snapshot(fixture, metadata);
            assertTrue(snapshot.diagnostics().isEmpty());
            assertEquals(3, snapshot.predictedResources().size());
            assertEquals(
                    Set.of("sampleprimary", "samplebackup", "samplekv"),
                    snapshot.predictedResources().stream().map(SnapshotResource::getName).collect(Collectors.toSet()));
        }
    }
}