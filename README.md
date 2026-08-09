# Bicep Test Framework

A set of libraries for writing tests against [Bicep](https://github.com/Azure/bicep) files.

## Overview

`bicep-test` provides language-native testing workflows for Bicep infrastructure code. Each library can capture a fast, offline **snapshot** of what a deployment would produce or run an opt-in **live deployment test** against Azure.

Live tests compile a `.bicepparam` file, deploy it as an Azure Deployment Stack, and return deployment outputs and managed resource IDs for infrastructure and post-deployment behavior checks. The result owns cleanup: disposing or tearing it down deletes the stack and its managed resources. Live tests require an Azure credential, an existing resource group, and appropriate deployment and deletion permissions; standard repository tests remain credential-free.

## Goals
* Create a very thin unopinionated library that can easily be supported in multiple languages.
* Use Node as an example language, to determine viability and community interest.
* Allow simple assertions about predicted goal state (e.g. "all storage accounts must be zone-redundant").
* Support end-to-end assertions against real Azure resources with deterministic cleanup.

## Language support

- [Node](docs/node.md): available through npm
- [C#](docs/csharp.md): implemented, not yet available through NuGet
- [Go](docs/go.md): implemented, not yet released as a versioned Go module
- [PowerShell](docs/powershell.md): implemented, not yet available through the PowerShell Gallery
- [Python](docs/python.md): implemented, not yet available through PyPI
- [Java](docs/java.md): implemented, not yet available through Maven Central

## Samples

Runnable test suites under [`samples/`](samples/) demonstrate the same infrastructure assertions with Jest, MSTest, Go's `testing` package, Pester, pytest, and JUnit. They share one Bicep fixture and are compiled and executed in CI.

See [CONTRIBUTING.md](CONTRIBUTING.md) for repository setup, build commands, tests, and project conventions.
