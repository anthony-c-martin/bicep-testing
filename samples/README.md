# Learn to test your Bicep

These tests are examples to copy and adapt to your own infrastructure. They
test requirements of the accompanying Bicep templates, not the correctness of
the testing library. Library conformance and edge-case tests live under
[`packages/`](../packages/), with shared test-only data in
[`packages/testdata/`](../packages/testdata/).

Choose the test framework your project already uses:

| Language | Offline examples | Opt-in Azure examples |
| --- | --- | --- |
| Node / Jest | [snapshot.test.js](node/snapshot.test.js) | [deployment.test.js](node/deployment.test.js) |
| C# / MSTest | [OfflineTests.cs](dotnet/OfflineTests.cs) | [LiveTests.cs](dotnet/LiveTests.cs) |
| Go / `testing` | [snapshot_test.go](go/snapshot_test.go) | [deployment_test.go](go/deployment_test.go) |
| PowerShell / Pester | [BicepTest.Sample.Tests.ps1](powershell/BicepTest.Sample.Tests.ps1) | [BicepTest.Deployment.Sample.Tests.ps1](powershell/BicepTest.Deployment.Sample.Tests.ps1) |
| Python / pytest | [test_snapshot.py](python/test_snapshot.py) | [test_deployment.py](python/test_deployment.py) |

The languages share Bicep templates and demonstrate comparable requirements,
but use their own native assertion and cleanup idioms.

## Start with an offline security test

Read the security-baseline test in your language alongside
[`secure.bicepparam`](infra/security-baseline/secure.bicepparam) and
[`main.bicep`](infra/security-baseline/main.bicep). It shows the essential workflow:

1. Create a session with a pinned Bicep CLI version and arrange its cleanup.
2. Evaluate a parameter file locally. No Azure credentials or context are needed
   for these assertions because the relevant inputs are explicit.
3. Check diagnostics, then select the expected storage account and Key Vault.
4. Assert the security requirements: HTTPS/TLS 1.2, disabled public/shared-key
   access, RBAC, purge protection, and 90-day secret retention.

To see why the assertions matter, temporarily point **this test** at
[`insecure.bicepparam`](infra/security-baseline/insecure.bicepparam) and rerun it.
The security assertions should fail. Restore the secure parameter file afterward.
Do not turn that regression into a passing test by asserting the insecure values
are correct.

Snapshots predict a template's goal state; they do not prove that Azure will
accept it or that a deployed service behaves correctly.

## Add scenarios as requirements grow

| Scenario | Parameter file | Requirement demonstrated |
| --- | --- | --- |
| Development | [dev.bicepparam](infra/environment-topology/dev.bicepparam) | One inexpensive LRS account, internal-data tags, no audit account. |
| Production | [prod.bicepparam](infra/environment-topology/prod.bicepparam) | Zone-redundant primary storage, geo-redundant audit storage, confidential-data tags. |
| Private storage | [main.bicepparam](infra/private-network/main.bicepparam) | Planned address ranges and NSGs, a blob endpoint on the data subnet, DNS linked to the correct VNet. |
| Platform policy | [management-group.bicepparam](infra/snapshot-scopes/management-group.bicepparam) | A policy at the intended management group that audits resources missing a cost-center tag. |

Development and production are separate tests so failures identify the affected
scenario. Each parameter file keeps its inputs literal and easy to review.

Resource-selection helpers make assertions independent of prediction order.
The networking example follows output IDs to the expected resources and compares
actual references, rather than only checking that a reference is nonempty.
Use direct JSON traversal where natural; C# and Go can use JSON Pointer helpers
to reduce nested traversal boilerplate. In PowerShell, pipe a selected resource
through `Get-BicepSnapshotValue -Pointer ''` to project the whole resource into
native values, so assertions do not need chains of .NET JSON accessors.

Supply evaluation context only where the Bicep expressions need it. The
topology/network tests share fake resource-group context, while the policy test
supplies a management-group ID. These values do not need to exist in Azure.

## Run and adapt the examples

The new resource helpers and management-group snapshot context are **unreleased**.
For this checkout, use the local-package workflow from the repository root:

```powershell
./scripts/ValidateSamples.ps1 -RunSnapshots
```

It packs the local libraries, restores sample dependencies, and runs the five
offline tests in each language with the native runners. It selects only snapshot
tests, even if live Azure environment variables are set. It cannot run the live
deployment samples. CLI installation and dependency/module restoration may still
need network access.

The checked-in Dev Container provides the five-language toolchain. The Node
sample requires Node 24.9+ and enables Jest VM modules for ESM Azure dependencies.
Snapshots use Bicep CLI 0.47.16.

Without `-RunSnapshots`, the script only compiles, parses, or collects the
examples; this is the standard credential-free CI packaging check, not evidence
that the infrastructure assertions passed. NuGet restore respects configured
feeds and adds the local package feed.

In your own project, install the published library version that provides the
APIs you use, copy the relevant template/parameter/test files, and run them with
your normal Jest, MSTest, Go, Pester, or pytest command. Keep the native lifecycle
hooks and replace the example's names, inputs, and expectations with your
requirements. Add separate scenarios when a parameter changes the intended
behavior; do not copy every example merely to exercise each library method.

## Opt-in tests against Azure

Only use these with disposable resources and explicit authorization:

- **Validation** asks Azure to validate the storage template before deployment.
  Some languages demonstrate this as a separate test, others before deploying.
- **Secure storage** deploys a storage account and reads its actual Azure
  security settings, rather than relying on snapshot predictions.
- **Reconciliation** enables audit storage, then disables it in the same stack.
  The primary account must remain available and the audit account must disappear.
- **Identical redeployment** reuses the same stack and inputs for two deployments.
  It checks that the primary account ID and actual security settings remain
  correct after the second deployment.

These use [`live-storage/main.bicepparam`](infra/live-storage/main.bicepparam)
and `Standard_LRS` storage accounts. Cleanup is in `finally` or native cleanup
hooks, including failed deployment results. Preserve that pattern when copying
the tests; cleanup is safety plumbing, not an assertion about your template.
Neither reconciliation nor identical redeployment proves every provider/SKU is
idempotent.

Live examples are skipped unless the required settings are present:

- `AZURE_SUBSCRIPTION_ID`: subscription containing the existing target resource group.
- `AZURE_RESOURCE_GROUP`: resource group used by the Deployment Stack.
- `BICEP_TEST_RESOURCE_PREFIX`: unique lowercase alphanumeric resource-name prefix.
- `BICEP_TEST_STACK_NAME`: disposable base stack name for Node, Go, PowerShell,
  and Python. C# demonstrates the library's generated stack-name default.

The language's default Azure credential chain must have deploy/delete
permissions; PowerShell uses the current Az login. After enabling these settings,
run the live test file with your native runner, not `-RunSnapshots`. These tests
create resources and can incur costs. Never reuse a stack that manages non-test
infrastructure.

## AI-authored consumer tests

Copy [the consumer authoring skill](../.github/skills/author-bicep-tests/) into a
consumer repository's `.github/skills/` directory. It guides requirement-based
scenarios, test and parameter-file naming, literal inputs, meaningful assertions,
and iterative validation. It does not measure coverage or promise all possible
inputs are bug-free.
