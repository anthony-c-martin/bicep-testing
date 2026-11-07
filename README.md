# Bicep Testing Framework

A set of libraries for writing tests against [Bicep](https://github.com/Azure/bicep) files.

## Overview

BicepTesting is an independent, non-official set of language-native testing libraries for Bicep infrastructure code. Each library supports credential-free **offline** snapshot evaluation and opt-in **live** Azure validation or deployment tests. Creating an offline session downloads the requested Bicep CLI version when it is not already cached, so first use may require network access; snapshot evaluation itself requires no Azure credentials or subscription.

Live sessions own Azure credentials and support three Deployment Stack scopes (resource group, subscription, and management group). Live tests compile a `.bicepparam` file, run Azure validation when requested, and deploy when needed so tests can assert outputs and managed resources. Validation returns scope-aware resources, correlation IDs, and structured errors. Deployment returns pre-submission failures as operation errors and post-submission Azure failures as failed results that still own cleanup. Cleanup deletes the stack and managed resources, is idempotent after successful deletion, and should run on both success and failure paths. Standard repository tests remain credential-free.

## Goals
* Create a very thin unopinionated library that can easily be supported in multiple languages.
* Use Node as the semantic reference implementation while exposing equivalent behavior through each ecosystem's idioms.
* Allow simple assertions about predicted goal state (e.g. "all storage accounts must be zone-redundant").
* Support end-to-end assertions against real Azure resources with deterministic cleanup.

## Language support

- [Node](packages/node/README.md) 22 or later: `@anthony-c-martin/bicep-testing` 0.1.6 on npm
- [C#](packages/dotnet/README.md) on .NET 10 or later: `AnthonyCMartin.BicepTesting` 0.1.6 on NuGet
- [Go](packages/go/README.md) 1.25 or later: `github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing` v0.1.6
- [PowerShell](packages/powershell/README.md) 7.6 or later: `AnthonyCMartin.BicepTesting` 0.1.6 on the PowerShell Gallery
- [Python](packages/python/README.md) 3.11 or later: `anthonycmartin-bicep-testing` 0.1.6 on PyPI

Lower-level Bicep CLI integrations are published as the Go `bicep-rpc-client` module at v0.1.6 and the Python `anthonycmartin-bicep-rpc-client` distribution at 0.1.6, imported as `anthonycmartin.bicep_rpc_client`.

## Samples

Runnable test suites under [`samples/`](samples/) demonstrate four credential-free snapshot scenarios and four opt-in live scenarios (validation, deployment, reconciliation, and identical redeployment) with Jest, MSTest, Go's `testing` package, Pester, and pytest. Every language uses the same shared Bicep fixtures and equivalent assertions. Standard CI compiles or collects the samples without requiring Azure credentials or creating resources. See the [sample instructions](samples/README.md) for the scenario catalog and the environment variables required to run the live tests.

## Snapshot usability (unreleased)

The development checkout supports optional evaluation context at resource-group,
subscription, management-group, and tenant scopes. Context values do not have to
exist in Azure. Supply them when assertions depend on them; deployment names can
also affect offline results. Management-group context requires Bicep CLI 0.47.16+
and fails explicitly on older versions.

Generic helpers select resources by type, require a single resource by ID or
type/name, extract resource scope, and traverse JSON with JSON Pointers. Required
selection and missing paths fail explicitly rather than letting assertions pass
on absent data. Raw result data remains available, with idiomatic access in each
language.

The opt-in identical-redeployment sample tests the same inputs and resource
identities twice before cleanup, separately from changed-input reconciliation.
It can catch provider idempotency failures but cannot establish that every Azure
resource or SKU is safe to redeploy.

For consistent AI-authored consumer tests, copy the
[`author-bicep-tests` skill](.github/skills/author-bicep-tests/) into your test
repository. It guides naming, literal parameter inputs, scenario matrices,
meaningful assertions, cleanup, and iteration. Automated scenario/source coverage
reporting is not yet implemented; the skill does not promise exhaustive coverage.

See [CONTRIBUTING.md](CONTRIBUTING.md) for repository setup, build commands, tests, and project conventions.
