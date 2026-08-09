# Samples

Each language demonstrates both local snapshot assertions and an opt-in live Azure deployment through its standard testing workflow:

- [Node](node/) uses Jest.
- [C#](dotnet/) uses MSTest.
- [Go](go/) uses the standard `testing` package.
- [PowerShell](powershell/) uses Pester.
- [Python](python/) uses pytest.

All samples share [`infra/main.bicepparam`](infra/main.bicepparam). Snapshot tests predict its resources without Azure credentials. Deployment tests compile it, create an Azure Deployment Stack, assert against the resulting resources and outputs, and delete the stack and its managed resources during cleanup.

Live deployment tests are skipped unless all of these environment variables are set:

- `AZURE_SUBSCRIPTION_ID`: subscription containing the target resource group.
- `AZURE_RESOURCE_GROUP`: existing resource group used by the Deployment Stack.
- `BICEP_TEST_STACK_NAME`: unique stack name for this test run.
- `BICEP_TEST_RESOURCE_PREFIX`: unique lowercase alphanumeric prefix for globally named resources.

The default Azure credential chain for the language must also be able to deploy and delete resources in the target resource group. Use disposable test resources and never reuse a stack that manages non-test infrastructure.

Validate every sample from the repository root:

```powershell
./scripts/ValidateSamples.ps1
```

The validator restores the local libraries and dependencies, then compiles, parses, or collects every sample test. It does not execute snapshot or live deployment tests, so standard CI remains credential-free and cannot create Azure resources.

To run a language's tests, use its native test command after setting the live deployment environment variables. Without those variables, only the credential-free snapshot tests run and the deployment test is skipped.