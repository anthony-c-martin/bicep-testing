using System.Text.Json;

namespace AnthonyCMartin.BicepTesting;

public sealed class DeployOptions
{
    public required string FilePath { get; init; }

    public string? ManagementGroupId { get; init; }

    public string? SubscriptionId { get; init; }

    public string? ResourceGroup { get; init; }

    public string? Location { get; init; }

    public string StackName { get; init; } = $"bicep-test-{Guid.NewGuid():N}";

    public IReadOnlyDictionary<string, JsonElement> ParameterOverrides { get; init; }
        = new Dictionary<string, JsonElement>();
}