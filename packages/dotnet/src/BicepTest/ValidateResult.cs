using Azure.ResourceManager.Resources.DeploymentStacks.Models;

namespace AnthonyCMartin.BicepTesting;

public sealed class ValidateResult
{
    private ValidateResult(
        IReadOnlyList<DeploymentResource> resources,
        string? correlationId,
        OperationError? error)
    {
        Resources = resources;
        CorrelationId = correlationId;
        Error = error;
    }

    public IReadOnlyList<DeploymentResource> Resources { get; }

    public string? CorrelationId { get; }

    public bool IsValid => Error is null;

    public OperationError? Error { get; }

    public string? ErrorCode => Error?.Code;

    public string? ErrorMessage => Error?.Message;

    internal static ValidateResult FromValidation(DeploymentStackValidateResult validation)
    {
        var resources = validation.Properties.ValidatedResources
            .Where(resource => resource.Id is not null)
            .Select(resource => new DeploymentResource(
                resource.Id!.ToString(),
                resource.Type?.ToString()))
            .ToArray();
        return new ValidateResult(
            resources,
            validation.Properties.CorrelationId,
            OperationError.FromResponseError(validation.Error));
    }
}