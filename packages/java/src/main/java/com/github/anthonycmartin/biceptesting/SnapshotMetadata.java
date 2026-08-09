package com.github.anthonycmartin.biceptesting;

import java.util.LinkedHashMap;
import java.util.Map;

/** Azure deployment context used to evaluate a snapshot. */
public final class SnapshotMetadata {
    private final String tenantId;
    private final String subscriptionId;
    private final String resourceGroup;
    private final String location;
    private final String deploymentName;

    private SnapshotMetadata(Builder builder) {
        tenantId = builder.tenantId;
        subscriptionId = builder.subscriptionId;
        resourceGroup = builder.resourceGroup;
        location = builder.location;
        deploymentName = builder.deploymentName;
    }

    public static Builder builder() {
        return new Builder();
    }

    Map<String, String> toRpcMap() {
        Map<String, String> result = new LinkedHashMap<>();
        putIfNotNull(result, "tenantId", tenantId);
        putIfNotNull(result, "subscriptionId", subscriptionId);
        putIfNotNull(result, "resourceGroup", resourceGroup);
        putIfNotNull(result, "location", location);
        putIfNotNull(result, "deploymentName", deploymentName);
        return result;
    }

    private static void putIfNotNull(Map<String, String> target, String name, String value) {
        if (value != null) {
            target.put(name, value);
        }
    }

    /** Builds snapshot deployment metadata. */
    public static final class Builder {
        private String tenantId;
        private String subscriptionId;
        private String resourceGroup;
        private String location;
        private String deploymentName;

        private Builder() {}

        public Builder tenantId(String value) {
            tenantId = value;
            return this;
        }

        public Builder subscriptionId(String value) {
            subscriptionId = value;
            return this;
        }

        public Builder resourceGroup(String value) {
            resourceGroup = value;
            return this;
        }

        public Builder location(String value) {
            location = value;
            return this;
        }

        public Builder deploymentName(String value) {
            deploymentName = value;
            return this;
        }

        public SnapshotMetadata build() {
            return new SnapshotMetadata(this);
        }
    }
}