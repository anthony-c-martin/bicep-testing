# Bicep Testing Framework

A set of libraries for writing tests against [Bicep](https://github.com/Azure/bicep) files.

## Overview

BicepTesting is an independent, non-official set of language-native testing libraries for Bicep infrastructure code. Each library can capture a fast, local **snapshot** of what a deployment would produce without deploying to Azure or run an opt-in **live deployment test** against Azure. Creating a `BicepTestSession` downloads the requested Bicep CLI version when it is not already cached, so first use may require network access; snapshot evaluation itself requires no Azure credentials or subscription.

Live tests compile a `.bicepparam` file, deploy it as an Azure Deployment Stack, and return deployment outputs and managed resource IDs for infrastructure and post-deployment behavior checks. `BicepTestSession` owns the Bicep CLI process; the returned `DeployResult` owns Azure cleanup. Disposing or tearing down the deployment result deletes the stack and its managed resources, and repeated cleanup returns the first cleanup outcome. Live tests require an Azure credential, an existing resource group, and appropriate deployment and deletion permissions. Use a unique stack name for each test run, and never reuse a stack that manages non-test resources. Standard repository tests remain credential-free.

## Goals
* Create a very thin unopinionated library that can easily be supported in multiple languages.
* Use Node as the semantic reference implementation while exposing equivalent behavior through each ecosystem's idioms.
* Allow simple assertions about predicted goal state (e.g. "all storage accounts must be zone-redundant").
* Support end-to-end assertions against real Azure resources with deterministic cleanup.

## Language support

- [Node](docs/node.md) 22 or later: `@anthony-c-martin/bicep-testing` 0.1.2 on npm
- [C#](docs/csharp.md) on .NET 10 or later: `AnthonyCMartin.BicepTesting` 0.1.2 on NuGet
- [Go](docs/go.md) 1.24 or later: `github.com/anthony-c-martin/bicep-testing/packages/go` v0.1.2
- [PowerShell](docs/powershell.md) 7.6 or later: `AnthonyCMartin.BicepTesting` 0.1.2 on the PowerShell Gallery
- [Python](docs/python.md) 3.11 or later: `anthonycmartin-bicep-testing` 0.1.2 on PyPI

Lower-level Bicep CLI integrations are published as the Go `bicep-rpc-client` module at v0.1.2 and the Python `anthonycmartin-bicep-rpc-client` distribution at 0.1.2, imported as `anthonycmartin.bicep_rpc_client`.

## Samples

Runnable test suites under [`samples/`](samples/) demonstrate three credential-free snapshot scenarios and two opt-in live deployment scenarios with Jest, MSTest, Go's `testing` package, Pester, and pytest. Every language uses the same shared Bicep fixtures and equivalent assertions. Standard CI compiles or collects the samples without requiring Azure credentials or creating resources. See the [sample instructions](samples/README.md) for the scenario catalog and the environment variables required to run the live tests.

See [CONTRIBUTING.md](CONTRIBUTING.md) for repository setup, build commands, tests, and project conventions.
