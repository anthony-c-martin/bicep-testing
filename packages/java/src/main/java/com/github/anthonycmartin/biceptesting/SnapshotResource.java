package com.github.anthonycmartin.biceptesting;

import com.fasterxml.jackson.annotation.JsonAnySetter;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.databind.JsonNode;
import java.util.LinkedHashMap;
import java.util.Map;

/** A resource predicted by a Bicep snapshot. */
public final class SnapshotResource {
    private String id;
    private String type;
    private String name;
    private String apiVersion;
    private String location;
    private JsonNode properties;
    private final Map<String, JsonNode> additionalProperties = new LinkedHashMap<>();

    public String getId() {
        return id;
    }

    public String getType() {
        return type;
    }

    public String getName() {
        return name;
    }

    @JsonProperty("apiVersion")
    public String getApiVersion() {
        return apiVersion;
    }

    public String getLocation() {
        return location;
    }

    public JsonNode getProperties() {
        return properties;
    }

    public Map<String, JsonNode> getAdditionalProperties() {
        return Map.copyOf(additionalProperties);
    }

    @JsonAnySetter
    void addAdditionalProperty(String key, JsonNode value) {
        additionalProperties.put(key, value);
    }
}