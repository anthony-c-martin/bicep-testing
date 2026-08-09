# Bicep Test Framework

A set of libraries for writing tests against [Bicep](https://github.com/Azure/bicep) files.

## Overview

`bicep-test` provides language-native testing workflows for Bicep infrastructure code. Each library can capture a fast, local **snapshot** of what a deployment would produce without deploying to Azure or run an opt-in **live deployment test** against Azure. Creating a `BicepTestSession` downloads the requested Bicep CLI version when it is not already cached, so first use may require network access; snapshot evaluation itself requires no Azure credentials or subscription.

Live tests compile a `.bicepparam` file, deploy it as an Azure Deployment Stack, and return deployment outputs and managed resource IDs for infrastructure and post-deployment behavior checks. `BicepTestSession` owns the Bicep CLI process; the returned `DeployResult` owns Azure cleanup. Disposing or tearing down the deployment result deletes the stack and its managed resources, and repeated cleanup returns the first cleanup outcome. Live tests require an Azure credential, an existing resource group, and appropriate deployment and deletion permissions. Use a unique stack name for each test run, and never reuse a stack that manages non-test resources. Standard repository tests remain credential-free.

## Goals
* Create a very thin unopinionated library that can easily be supported in multiple languages.
* Use Node as the semantic reference implementation while exposing equivalent behavior through each ecosystem's idioms.
* Allow simple assertions about predicted goal state (e.g. "all storage accounts must be zone-redundant").
* Support end-to-end assertions against real Azure resources with deterministic cleanup.

## Language support

- [Node](docs/node.md) 22 or later: implemented, not yet available through npm
- [C#](docs/csharp.md) on .NET 10 or later: implemented, not yet available through NuGet
- [Go](docs/go.md) 1.24 or later: implemented, not yet released as a versioned Go module
- [PowerShell](docs/powershell.md) 7.6 or later: implemented, not yet available through the PowerShell Gallery
- [Python](docs/python.md) 3.11 or later: implemented, not yet available through PyPI
- [Java](docs/java.md) 17 or later: implemented, not yet available through Maven Central

## Samples

Runnable test suites under [`samples/`](samples/) demonstrate the same credential-free snapshot assertions with Jest, MSTest, Go's `testing` package, Pester, pytest, and JUnit. They share one Bicep fixture and are compiled and executed in CI. Live deployment examples are documented in each language guide but remain opt-in and are not run by standard CI.

See [CONTRIBUTING.md](CONTRIBUTING.md) for repository setup, build commands, tests, and project conventions.
