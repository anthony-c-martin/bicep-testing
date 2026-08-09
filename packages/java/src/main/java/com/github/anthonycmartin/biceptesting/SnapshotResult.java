package com.github.anthonycmartin.biceptesting;

import com.fasterxml.jackson.databind.JsonNode;
import java.util.List;
import java.util.Map;

/** The predicted result of evaluating a Bicep parameters file. */
public record SnapshotResult(
        List<SnapshotResource> predictedResources,
        List<String> diagnostics,
                Map<String, JsonNode> outputs) {
        public SnapshotResult {
                predictedResources = List.copyOf(predictedResources);
                diagnostics = List.copyOf(diagnostics);
                outputs = Map.copyOf(outputs);
        }
}