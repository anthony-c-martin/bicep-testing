package com.github.anthonycmartin.biceptesting;

import com.fasterxml.jackson.databind.JsonNode;
import java.util.List;
import java.util.Map;

/** Outputs and managed resources from a deployment, with deterministic cleanup. */
public final class DeployResult implements AutoCloseable {
    private final Map<String, JsonNode> outputs;
    private final List<DeploymentResource> resources;
    private final Runnable teardown;
    private boolean closed;
    private RuntimeException teardownError;

    DeployResult(Map<String, JsonNode> outputs, List<DeploymentResource> resources, Runnable teardown) {
        this.outputs = Map.copyOf(outputs);
        this.resources = List.copyOf(resources);
        this.teardown = teardown;
    }

    public Map<String, JsonNode> outputs() {
        return outputs;
    }

    public List<DeploymentResource> resources() {
        return resources;
    }

    @Override
    public synchronized void close() {
        if (closed) {
            if (teardownError != null) {
                throw teardownError;
            }
            return;
        }
        closed = true;
        try {
            teardown.run();
        } catch (RuntimeException exception) {
            teardownError = exception;
            throw exception;
        }
    }
}