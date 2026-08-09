using System.Text.Json;

namespace AnthonyCMartin.BicepTesting;

public sealed class DeployOptions
{
    public required string FilePath { get; init; }

    public required string SubscriptionId { get; init; }

    public required string ResourceGroup { get; init; }

    public required string StackName { get; init; }

    public IReadOnlyDictionary<string, JsonElement> ParameterOverrides { get; init; }
        = new Dictionary<string, JsonElement>();
}