---
name: add-test-framework-capability
description: 'Add or extend a bicep-testing framework capability across Node, C#, Go, PowerShell, and Python. Use when implementing new snapshot behavior, options, result data, lifecycle operations, or other public features that require idiomatic APIs, cross-language conformance tests, shared Bicep samples, regenerated public API baselines, language docs, and an abstract README explanation.'
argument-hint: 'Describe the capability and expected behavior'
user-invocable: true
disable-model-invocation: false
---

# Add Test Framework Capability

Implement one capability end to end across every supported library without forcing identical syntax between languages.

## Required Outcome

A capability is complete only when all of the following are true:

- Node, C#, Go, PowerShell, and Python expose equivalent behavior through idiomatic public APIs.
- Each language has focused tests for success, relevant edge cases, and errors introduced by the capability.
- A shared Bicep sample demonstrates the capability and is exercised by all applicable conformance tests.
- Public API artifacts under `api/` are regenerated and verified.
- Each language-specific document explains its API and includes a realistic usage example.
- `README.md` explains the capability in language-neutral terms.
- All affected language test and API checks pass.

Do not declare the work complete when one implementation is still a stub, tests only cover one language, or an API baseline was edited without its check passing.

## Source Of Truth

Use the Node implementation to determine semantic behavior, but not naming or syntax in other languages. Establish these language-neutral facts before editing:

1. What user problem does the capability solve?
2. What inputs does it accept, which are required, and what defaults apply?
3. What result or side effect does it produce?
4. What lifecycle, cancellation, cleanup, and error behavior does it require?
5. Which behavior can be demonstrated by a shared `.bicep` or `.bicepparam` fixture?

If the requested behavior is ambiguous, inspect the nearest existing implementation and tests. Ask the user only when a product-level choice remains unresolved.

## Workflow

### 1. Trace The Existing Behavior

- Start at `packages/node/src` and the nearest test in `packages/node/test`.
- Inspect the equivalent implementation and test surfaces in:
  - `packages/dotnet/src/BicepTest` and `packages/dotnet/test/BicepTest.Tests`
  - `packages/go` and, when transport behavior changes, `packages/go/rpcclient`
  - `packages/powershell/AnthonyCMartin.BicepTesting` and `packages/powershell/test`
  - `packages/python/src/anthonycmartin/bicep_testing` and, when transport behavior changes, `packages/python/src/anthonycmartin/bicep_testing/rpcclient`
- Identify the smallest shared behavior contract and the cheapest focused test that can disprove the proposed implementation.
- Check the current worktree before editing and preserve unrelated user changes.

### 2. Add Or Update The Shared Sample

- Keep reusable example Bicep fixtures under `samples/infra`; this is the canonical consumer-sample location.
- Prefer extending an existing fixture when it remains understandable. Create a focused fixture when extension would mix unrelated behavior.
- Make the sample deterministic, offline, and independent of Azure credentials or deployment.
- Include only the Bicep constructs needed to demonstrate the behavior clearly.
- Update the runnable Jest, MSTest, Go `testing`, Pester, and pytest examples under `samples/` to demonstrate the capability idiomatically.
- Point every applicable sample and conformance test at the same fixture. Do not duplicate equivalent fixtures per language.

### 3. Implement Idiomatically In Every Library

Preserve equivalent semantics while following each ecosystem's conventions.

#### Node

- Treat Node as the behavioral reference implementation.
- Use TypeScript types for public inputs and results.
- Follow the existing async lifecycle and naming conventions.
- Keep Azure/Bicep client details behind the public test-framework abstraction.
- Add tests with Jest under `packages/node/test`.

#### C#

- Use .NET naming, nullable annotations, `Task`/`ValueTask`, cancellation tokens, and disposal patterns as appropriate.
- Prefer typed request/result models over loosely structured dictionaries when the shape is stable.
- Keep the implementation in `packages/dotnet/src/BicepTest`.
- Add MSTest tests under `packages/dotnet/test/BicepTest.Tests`.
- Do not reproduce JavaScript naming or optional-value conventions when a clearer .NET API exists.

#### Go

- Use idiomatic exported names, explicit errors, `context.Context`, and `Close` where ownership requires cleanup.
- Keep the high-level snapshot API in the module root.
- Put Bicep installation, process, pipe/socket, and JSON-RPC behavior in `packages/go/rpcclient`.
- Preserve Windows named-pipe and Unix-domain-socket behavior when transport code changes.
- Add table-driven or focused Go tests next to the owning package.
- Do not copy class-oriented APIs into Go.

#### PowerShell

- Use approved `Verb-Noun` command names, `[CmdletBinding()]`, pipeline input where natural, and PowerShell parameter validation.
- Keep explicit exports synchronized between `AnthonyCMartin.BicepTesting.psm1` and `AnthonyCMartin.BicepTesting.psd1`; never use wildcard exports.
- Keep commands as a thin, idiomatic wrapper over the C# implementation when the capability exists there.
- Ensure errors are terminating when the command cannot produce a valid result.
- Add Pester tests under `packages/powershell/test`.
- Preserve PowerShell Core compatibility on Windows and Linux.

#### Python

- Use type annotations, standard naming conventions, immutable dataclasses for result data, and context managers for owned resources.
- Keep the implementation under `packages/python/src/anthonycmartin/bicep_testing` and pytest tests under `packages/python/tests`.
- Put Bicep process and JSON-RPC behavior in the separate public `rpcclient` package while keeping it behind the high-level `BicepTestSession` abstraction.

### 4. Add Cross-Language Tests

For each language, test the public surface rather than only private helpers. Cover:

- The primary successful workflow against the shared sample.
- The capability-specific result, side effect, or invariant.
- Optional/default behavior when relevant.
- Invalid input or client errors when the new code introduces a meaningful failure path.
- Resource cleanup when the capability owns a tester, process, stream, or disposable client.

Keep equivalent assertions aligned across languages while expressing them in each framework's native style. Avoid assertions that merely duplicate implementation details.

### 5. Regenerate And Review Public APIs

Run the update command for every language, even when no API change is expected. Review all resulting diffs under `api/` for accidental exposure.

#### Node

```powershell
Push-Location packages/node
npm run api:update
npm run api:check
Pop-Location
```

Review `api/node/bicep-testing.d.ts`.

#### C#

Build once to collect RS0016/RS0017 diagnostics, then apply the analyzer's public API updates using the IDE code fix or `dotnet format`:

```powershell
dotnet build packages/dotnet/src/BicepTest/BicepTest.csproj --configuration Release
dotnet format packages/dotnet/src/BicepTest/BicepTest.csproj analyzers --diagnostics RS0016 RS0017
dotnet build packages/dotnet/src/BicepTest/BicepTest.csproj --configuration Release
```

Review `api/dotnet/PublicAPI.Unshipped.txt` and `api/dotnet/PublicAPI.Shipped.txt`. Never suppress RS0016 or RS0017 to bypass an intentional API update.

#### Go

```powershell
Push-Location packages/go
go generate ./...
go run ./internal/apidoc --check
Pop-Location
```

Review `api/go/biceptesting.txt` and, when applicable, `api/go/rpcclient.txt`.

#### PowerShell

```powershell
./packages/powershell/scripts/public-api.ps1 -Update
./packages/powershell/scripts/public-api.ps1 -Check
```

Review `api/powershell/AnthonyCMartin.BicepTesting.txt`. Confirm the manifest exports exactly the intended commands.

#### Python

```powershell
python packages/python/scripts/public_api.py --update
python packages/python/scripts/public_api.py --check
```

Review `api/python/bicep-testing.txt` and `api/python/rpcclient.txt`.

### 6. Update Documentation

Update every language document, even when the capability has the same conceptual behavior:

- `docs/node.md`
- `docs/csharp.md`
- `docs/go.md`
- `docs/powershell.md`
- `docs/python.md`

Each document must explain:

- Why a user would use the capability.
- The language-specific public entry point and important inputs/defaults.
- A concise, runnable example in that language.
- The returned data, side effects, cleanup, and notable errors where relevant.

Update `README.md` separately. Describe the capability once in language-neutral, user-centered terms. Explain what it enables and its major behavioral constraints without listing language-specific method names or duplicating all five examples.

### 7. Validate End To End

Run focused tests during implementation, then run all affected package checks before finishing.

```powershell
# Node
Push-Location packages/node
npm test
npm run api:check
Pop-Location

# C#
dotnet test packages/dotnet/BicepTest.slnx --configuration Release

# Go
Push-Location packages/go
gofmt -w .
go vet ./...
go test ./...
go run ./internal/apidoc --check
Pop-Location

# PowerShell
./packages/powershell/build.ps1
Invoke-Pester ./packages/powershell/test -CI
./packages/powershell/scripts/public-api.ps1 -Check

# Python
python -m pytest packages/python/tests
python packages/python/scripts/public_api.py --check

# Runnable consumer samples
./scripts/ValidateSamples.ps1
```

Also check editor diagnostics for every changed implementation, test, manifest, script, and workflow file.

## Completion Report

Summarize the finished capability in this order:

1. Language-neutral behavior added.
2. Idiomatic API chosen for each language.
3. Shared sample and tests added or updated.
4. Public API artifacts regenerated.
5. Documentation updated.
6. Exact validation commands and outcomes.

Call out any unsupported platform, skipped check, or intentional semantic difference between languages. Do not hide incomplete work behind a general statement that tests passed.