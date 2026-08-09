# bicep-testing Repository Instructions

## Architecture

- This is a polyglot monorepo with first-class libraries under `packages/node`, `packages/dotnet`, `packages/go`, `packages/powershell`, and `packages/python`.
- Treat Node as the semantic source of truth, but expose equivalent behavior through each ecosystem's idioms rather than copying syntax or naming mechanically.
- For cross-language feature work, follow `.github/skills/add-test-framework-capability/SKILL.md`.
- Keep the public test-framework APIs thin. Bicep installation, process management, and JSON-RPC details belong behind those APIs.

## Bicep Integration

- Snapshot tests run locally and must not require Azure credentials, an Azure subscription, or a deployment.
- Node uses `@azure/bicep-rpc-client`; C# uses `Azure.Bicep.RpcClient`; PowerShell remains a thin wrapper over C#.
- Go owns its RPC transport in the standalone `packages/go/bicep-rpc-client` module; preserve Windows named-pipe and Unix-domain-socket behavior there.
- Python uses `bicep jsonrpc --stdio` with `Content-Length` framing. Keep that transport in the standalone `packages/python/bicep_rpc_client` distribution under `anthonycmartin.bicep_rpc_client`; do not introduce platform-specific pipe dependencies.
- APIs that own a Bicep process must provide and test deterministic cleanup using the language's native lifecycle pattern.

## Tests And Samples

- Reuse deterministic Bicep fixtures under `samples/infra`; do not duplicate equivalent fixtures per language.
- Use the native test framework for each ecosystem: Jest, MSTest, Go `testing`, Pester, and pytest.
- Consumer samples under `samples/<language>` are packaging tests as well as usage examples. Keep `scripts/ValidateSamples.ps1` able to compile and run all five.
- Keep equivalent behavioral assertions aligned across languages while expressing them idiomatically.

## Public API Governance

- Reviewable API artifacts live centrally under `api/<language>` and are required CI inputs.
- Regenerate API artifacts with the owning language's tooling, review the diff, and run its check mode. Do not bypass an API check or hand-edit a baseline to conceal unintended exposure.
- Avoid exposing transport clients, mutable implementation details, or factory-only constructors through public API artifacts.

## Repository Maintenance

- A new public capability is incomplete until implementation, native tests, consumer samples, API baselines, language docs, README behavior, CI, and the capability skill agree.
- When adding a package ecosystem or manifest location, update `.github/dependabot.yml` and `.gitignore` as needed.
- Keep generated outputs such as Python caches and metadata, .NET `bin`/`obj`, Node dependencies/build output, and PowerShell runtime payloads out of Git.
- Prefer the checked-in Dev Container for a consistent five-language toolchain. Keep dependency restore explicit so container creation does not depend on every package registry being available.