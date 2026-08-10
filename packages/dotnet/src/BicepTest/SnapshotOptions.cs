namespace AnthonyCMartin.BicepTesting;

public sealed class SnapshotOptions
{
    public required string FilePath { get; init; }

    public string? TenantId { get; init; }

    public string? SubscriptionId { get; init; }

    public string? ResourceGroup { get; init; }

    public string? Location { get; init; }

    public string DeploymentName { get; init; } = $"bicep-test-{Guid.NewGuid():N}";
}