using System.Text.Json;
using System.Text.Json.Serialization;

namespace AnthonyCMartin.BicepTesting;

public sealed class SnapshotResult
{
    [JsonPropertyName("predictedResources")]
    public SnapshotResource[] PredictedResources { get; init; } = [];

    [JsonPropertyName("diagnostics")]
    public string[] Diagnostics { get; init; } = [];

    [JsonPropertyName("outputs")]
    public Dictionary<string, JsonElement> Outputs { get; init; } = [];
}

public sealed class SnapshotResource
{
    [JsonPropertyName("id")]
    public string Id { get; init; } = string.Empty;

    [JsonPropertyName("type")]
    public string Type { get; init; } = string.Empty;

    [JsonPropertyName("name")]
    public string Name { get; init; } = string.Empty;

    [JsonPropertyName("apiVersion")]
    public string ApiVersion { get; init; } = string.Empty;

    [JsonPropertyName("location")]
    public string? Location { get; init; }

    [JsonPropertyName("properties")]
    public JsonElement Properties { get; init; }

    [JsonExtensionData]
    public Dictionary<string, JsonElement>? AdditionalProperties { get; init; }
}