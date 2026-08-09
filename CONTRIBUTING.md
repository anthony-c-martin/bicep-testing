# Contributing to bicep-test

## Development container

With Docker and the VS Code Dev Containers extension installed, run **Dev Containers: Reopen in Container** from the command palette. The configuration under `.devcontainer/` installs Node 24, .NET 10, Go 1.24, PowerShell 7.6 with Pester, Python 3.12, and Java 17 with Maven. Package-manager caches persist in named Docker volumes, while dependency restore remains explicit for each language.

## Repository layout

Each language implementation owns its build, dependencies, tests, and packaging under `packages/`:

```text
packages/
├── node/
│   ├── src/
│   ├── test/
│   ├── package.json
│   ├── jest.config.ts
│   └── tsconfig.json
├── dotnet/
│   ├── BicepTest.slnx
│   ├── src/BicepTest/
│   └── test/BicepTest.Tests/
├── go/
    ├── rpcclient/
    ├── biceptest.go
    ├── snapshot.go
    └── go.mod
├── powershell/
    ├── BicepTest/
    ├── scripts/
    └── test/
├── python/
│   ├── src/bicep_test/
│   ├── scripts/
│   └── tests/
└── java/
    ├── src/main/java/
    ├── src/test/java/
    └── pom.xml
```

The Node package defines the reference snapshot behavior. The C#, Go, PowerShell, Python, and Java conformance tests exercise the same Bicep fixture and assertions.

Runnable consumer tests are under `samples/`. Run all six language samples with:

```powershell
./scripts/ValidateSamples.ps1
```

## Node

Prerequisites:

- Node.js 24
- npm

Build and test from the repository root:

```sh
cd packages/node
npm ci --legacy-peer-deps
npm run build
npm test
npm run api:check
```

The legacy peer resolver is currently required because TypeScript 7 is outside the version range declared by the installed `@typescript-eslint` packages.

Review the Node public API in `api/node/bicep-test.d.ts`. After an intentional API change, run `npm run api:update` from `packages/node` and include the baseline change in the pull request.

## C#

Prerequisite: .NET 10 SDK or later.

Build and verify packaging from the repository root:

```sh
dotnet test packages/dotnet/BicepTest.slnx
dotnet pack packages/dotnet/src/BicepTest/BicepTest.csproj --configuration Release
```

Review the C# public API in `api/dotnet/PublicAPI.Unshipped.txt`. The Public API analyzer fails the build when a declaration is added or removed without updating this file. Apply the RS0016 or RS0017 code fix after reviewing an intentional API change.

Project conventions:

- Target framework: `net10.0`
- Root namespace and package ID: `BicepTest`
- Nullable reference types and implicit global usings are enabled.
- Add library code under `packages/dotnet/src/BicepTest`.
- Add tests under `packages/dotnet/test/BicepTest.Tests` and include the project in `BicepTest.slnx`.

## Go

Prerequisite: Go 1.24 or later.

Test from the repository root:

```sh
cd packages/go
go test ./...
go run ./internal/apidoc --check
```

Format and analyze Go changes before submitting them:

```sh
cd packages/go
gofmt -w .
go vet ./...
```

Review the Go public API in `api/go`. After an intentional API change, run `go generate ./...` from `packages/go` and include both affected baselines in the pull request.

Project conventions:

- Module path: `github.com/anthony-c-martin/bicep-test/packages/go`
- Package name: `biceptest`
- Keep exported names idiomatic to Go rather than reproducing the Node API naming exactly.
- Keep the public snapshot API in the module root.
- Keep Bicep installation, process, pipe, and JSON-RPC behavior in the separate `rpcclient` package.
- Preserve both Windows named-pipe and Unix-domain-socket support when changing the transport.

## PowerShell

Prerequisites:

- PowerShell 7.6 or later
- .NET 10 SDK or later
- Pester 5.7 or later

Build and test from the repository root:

```powershell
./packages/powershell/build.ps1
Invoke-Pester ./packages/powershell/test -CI
./packages/powershell/scripts/public-api.ps1 -Check
```

Review the PowerShell public API in `api/powershell/BicepTest.txt`. After an intentional API change, run `./packages/powershell/scripts/public-api.ps1 -Update` and include the baseline change in the pull request.

Project conventions:

- Keep the module manifest export lists explicit; do not use wildcard exports.
- Add public commands to `packages/powershell/BicepTest/BicepTest.psm1` and the manifest.
- Keep the PowerShell commands as a thin, idiomatic wrapper over the C# implementation.
- Preserve compatibility with PowerShell Core on Windows and Linux.

## Python

Prerequisite: Python 3.11 or later.

Install, test, and check the public API from the repository root:

```sh
python -m pip install -e "./packages/python[test]"
python -m pytest packages/python/tests
python packages/python/scripts/public_api.py --check
```

Review `api/python/bicep-test.txt`. After an intentional API change, run the API script with `--update` and include the baseline change in the pull request.

Project conventions:

- Keep runtime dependencies in the Python standard library where practical.
- Use type annotations, immutable dataclasses for result data, and context managers for owned processes.
- Add package code under `packages/python/src/bicep_test` and pytest tests under `packages/python/tests`.

## Java

Prerequisites: JDK 17 or later and Maven 3.9 or later.

Test and check the public API from `packages/java`:

```sh
mvn --batch-mode --no-transfer-progress test
mvn --batch-mode --no-transfer-progress --quiet -DskipTests test-compile exec:java -Dexec.classpathScope=test -Dexec.mainClass=com.github.anthonycmartin.biceptest.ApiSurface -Dexec.args=--check
```

Review `api/java/bicep-test.txt`. After an intentional API change, replace `--check` with `--update` in the API command and include the baseline change in the pull request.

Project conventions:

- Target Java 17 and use Maven for builds and dependency management.
- Use `AutoCloseable` for process ownership, builders for optional request metadata, and immutable result containers.
- Add library code under `packages/java/src/main/java` and JUnit 5 tests under `packages/java/src/test/java`.

## Pull requests

- Keep changes scoped to one behavior or package where practical.
- Add or update tests for behavior changes.
- Run the relevant language build and test commands before opening a pull request.
- Review and update the checked-in public API baseline for intentional API changes.
- Update user documentation when public APIs or supported behavior change.
- Add or update the native test-framework sample when user-facing workflows change.