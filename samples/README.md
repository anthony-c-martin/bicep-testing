# Samples

Each language demonstrates both local snapshot assertions and an opt-in live Azure deployment through its standard testing workflow:

- [Node](node/) uses Jest.
- [C#](dotnet/) uses MSTest.
- [Go](go/) uses the standard `testing` package.
- [PowerShell](powershell/) uses Pester.
- [Python](python/) uses pytest.

All languages exercise the same shared Bicep fixtures and equivalent assertions.

## Offline snapshot scenarios

- **Environment topology** evaluates separate development and production parameter files, checking conditional resources, storage SKUs, names, tags, and outputs.
- **Security baseline** verifies hardened Storage and Key Vault properties and demonstrates how a deliberately weakened parameter set exposes a regression.
- **Private network wiring** checks VNet and subnet address plans, NSG associations, private endpoint connections, private DNS links, and output contracts.

Snapshot tests evaluate these scenarios locally without Azure credentials or a subscription.

## Live deployment scenarios

- **Secure storage** deploys an ephemeral storage account, verifies its security properties through an authenticated Azure Resource Manager request, and confirms teardown removed it.
- **Deployment reconciliation** deploys primary and audit storage accounts, updates the same Deployment Stack to remove the audit account, verifies Azure reconciled the change, and confirms final teardown removed the remaining account.

Both scenarios use [`infra/live-storage/main.bicepparam`](infra/live-storage/main.bicepparam) and inexpensive `Standard_LRS` storage accounts. Deployment tests always delete the stack and its managed resources during cleanup.

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

The validator restores the published version 0.1.3 libraries and dependencies, then compiles, parses, or collects every sample test. It does not execute snapshot or live deployment tests, so standard CI remains credential-free and cannot create Azure resources.

To run a language's tests, use its native test command after setting the live deployment environment variables. Without those variables, the three credential-free snapshot tests run and the two deployment tests are skipped.