using Azure;
using Azure.Core;
using Azure.ResourceManager;
using Azure.ResourceManager.Resources.DeploymentStacks;

namespace AnthonyCMartin.BicepTesting;

public sealed class LiveBicepTestSession : IDisposable, IAsyncDisposable
{
    private readonly BicepTestSession session;
    private readonly TokenCredential credential;

    private LiveBicepTestSession(BicepTestSession session, TokenCredential credential)
    {
        this.session = session;
        this.credential = credential;
    }

    public static async Task<LiveBicepTestSession> CreateAsync(
        string bicepVersion,
        TokenCredential credential,
        CancellationToken cancellationToken = default)
    {
        ArgumentNullException.ThrowIfNull(credential);
        var session = await BicepTestSession.CreateAsync(bicepVersion, cancellationToken);
        return new LiveBicepTestSession(session, credential);
    }

    public Task<SnapshotResult> SnapshotAsync(
        SnapshotOptions options,
        CancellationToken cancellationToken = default) => session.SnapshotAsync(options, cancellationToken);

    public async Task<DeployResult> DeployAsync(
        DeployOptions options,
        CancellationToken cancellationToken = default)
    {
        BicepTestSession.ValidateDeployOptions(options);
        var compilation = await session.CompileDeploymentAsync(options, cancellationToken);
        var stackData = BicepTestSession.BuildDeploymentStackData(
            compilation.Template,
            compilation.Parameters,
            options.Location);
        var scope = BicepTestSession.BuildDeploymentScope(options);
        var armClient = new ArmClient(credential, options.SubscriptionId);
        var stackId = DeploymentStackResource.CreateResourceIdentifier(
            scope.ToString(),
            options.StackName);
        var stack = armClient.GetDeploymentStackResource(stackId);
        var failedDeployment = DeployResult.FromStackReference(stack);
        return await CompleteDeploymentAsync(
            failedDeployment,
            async () =>
            {
                var operation = await armClient.GetDeploymentStacks(scope).CreateOrUpdateAsync(
                    WaitUntil.Completed,
                    options.StackName,
                    stackData,
                    cancellationToken);
                return operation.Value;
            });
    }

    public async Task<ValidateResult> ValidateAsync(
        DeployOptions options,
        CancellationToken cancellationToken = default)
    {
        BicepTestSession.ValidateDeployOptions(options);
        var compilation = await session.CompileDeploymentAsync(options, cancellationToken);
        var stackData = BicepTestSession.BuildDeploymentStackData(
            compilation.Template,
            compilation.Parameters,
            options.Location);
        var scope = BicepTestSession.BuildDeploymentScope(options);
        var armClient = new ArmClient(credential, options.SubscriptionId);
        var stackId = DeploymentStackResource.CreateResourceIdentifier(
            scope.ToString(),
            options.StackName);
        var stack = armClient.GetDeploymentStackResource(stackId);
        var operation = await stack.ValidateStackAsync(
            WaitUntil.Completed,
            stackData,
            cancellationToken);
        return ValidateResult.FromValidation(operation.Value);
    }

    internal static async Task<DeployResult> CompleteDeploymentAsync(
        DeployResult failedDeployment,
        Func<Task<DeploymentStackResource>> deployAsync)
    {
        try
        {
            return DeployResult.FromStack(await deployAsync());
        }
        catch (Exception exception)
        {
            return failedDeployment.WithError(exception);
        }
    }

    public void Dispose() => session.Dispose();

    public ValueTask DisposeAsync() => session.DisposeAsync();
}