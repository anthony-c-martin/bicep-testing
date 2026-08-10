using Microsoft.VisualStudio.TestTools.UnitTesting;

namespace AnthonyCMartin.BicepTesting.Tests;

[TestClass]
public sealed class DeployResultTests
{
    [TestMethod]
    public async Task TeardownAsync_shares_cleanup_and_only_cancels_the_callers_wait()
    {
        var deleteStarted = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var allowDelete = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var deleteCalls = 0;
        var result = new DeployResult(async () =>
        {
            Interlocked.Increment(ref deleteCalls);
            deleteStarted.SetResult();
            await allowDelete.Task;
        });
        using var cancellation = new CancellationTokenSource();

        var canceledWait = result.TeardownAsync(cancellation.Token);
        await deleteStarted.Task;
        cancellation.Cancel();

        await Assert.ThrowsAsync<OperationCanceledException>(async () => await canceledWait);
        var successfulWait = result.TeardownAsync();
        Assert.AreEqual(1, deleteCalls);
        Assert.IsFalse(successfulWait.IsCompleted);

        allowDelete.SetResult();
        await successfulWait;
        await result.TeardownAsync();
        Assert.AreEqual(1, deleteCalls);
    }
}