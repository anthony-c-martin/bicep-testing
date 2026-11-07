---
name: author-bicep-tests
description: 'Author or improve consumer Bicep tests using bicep-testing with Jest, MSTest, Go testing, Pester, or pytest. Use when generating tests or bicepparam scenarios, reviewing scenario gaps, grouping and naming infrastructure tests, or improving AI-authored snapshot assertions.'
argument-hint: 'Describe the Bicep module, test language, and behaviors to validate'
user-invocable: true
disable-model-invocation: false
---

# Author Bicep Tests

Write readable, credential-free tests that validate an infrastructure contract,
not merely reproduce its current implementation. This skill is for consumers of
the framework, not contributors adding framework capabilities.

## Establish The Contract

1. Read the module, referenced modules, existing parameter files, and tests.
2. Identify the intended behaviors from requirements and documentation. Do not
   infer that the current output is correct merely because the module emits it.
3. Use the consumer's existing native runner: Jest, MSTest, Go `testing`, Pester,
   or pytest. Inspect the installed framework version and its public API before
   suggesting helpers; do not assume unreleased APIs exist.
4. Ask for clarification when intended behavior or required scenarios are
   ambiguous. Do not change infrastructure behavior just to simplify testing.

## Build A Scenario Matrix Before Writing Tests

Describe the matrix in the conversation or an existing test document; do not
create tracking files unless requested. Include scenario, parameter values,
expected resources/outputs/diagnostics, and verification status.

- Every finite allowed value or union member that changes behavior.
- Defaults, explicitly supplied defaults, and relevant omitted optional values.
- Both outcomes of resource/module conditions and important conditional values.
- Empty, single, and multiple loop inputs; relevant numeric/string boundaries.
- Feature interactions, such as environment plus networking mode plus SKU.
- Invalid combinations, with the expected compilation/evaluation or live error.
- Explicit exclusions and why they are not applicable.

Enumerate combinations when the domain is small. Otherwise select representative
boundaries and pairwise combinations, then add known higher-order interactions.
Never call pairwise coverage exhaustive or promise that all possible inputs are
bug-free. The framework does not yet measure scenario or source coverage:
report manually identified gaps and verified cases, not an invented percentage.
Generating a parameter file or evaluating a snapshot is not itself a passing test.

## Keep Test Inputs And Names Simple

- Group tests by observable behavior, not one assertion per source line.
- Name tests with scenario and expectation, for example
  `production_with_audit_storage_uses_zone_redundant_primary_storage`.
- Use descriptive parameter filenames, for example `prod-private.bicepparam`,
  `dev-public.bicepparam`, and `invalid-premium-with-public-access.bicepparam`.
- Put each scenario's actual values directly on parameters/properties in test
  `.bicepparam` files. Avoid variables or layered indirection unless they
  materially improve readability or express a necessary relationship.
- Reuse existing fixtures; keep expected values independent of calculations in
  the implementation under test. Avoid opaque generated inputs.
- Pin a Bicep CLI version. Management-group snapshot context requires 0.47.16+.

## Write Meaningful Assertions

- Check diagnostics before asserting successful snapshot content. For negative
  cases, assert the expected diagnostic or exception, not just any failure.
- Assert presence/count before checking selected resources. A loop over an empty
  selection can otherwise pass without validating anything.
- Prefer generic resource selection and required-single-resource helpers when
  available. Assert absence explicitly for resources disabled by a condition.
- Check relevant properties, IDs/scopes, references, and output contracts.
  Keep direct JSON access where natural; do not build resource-specific wrapper
  classes for a few assertions.
- JSON Pointer helpers preserve explicit null and fail for missing paths. Do not
  treat a missing security property as a valid null or safe default.
- Make failures explain the scenario and expected behavior.
- Create one session per suite where natural and always close it with the
  language's lifecycle pattern, including failure paths.

## Keep Live Tests Separate And Opt-In

Offline snapshots need no Azure account, subscription, or deployment. CLI
installation or remote module restoration may still require network access.
Supply evaluation context only when the tested expressions need it; these
values do not have to exist in Azure. A deployment name can affect offline
`deployment().name` evaluation.

Never deploy, authenticate, or incur Azure costs without explicit authorization.
Keep live tests skipped unless the consumer intentionally enables them. Use
disposable resources, unique names, least-privilege credentials, and cleanup in
`finally`/native cleanup hooks even when Azure returns a failed deployment result.

Idempotency tests deploy twice with identical parameters, stack name, and resource
identities, without teardown between deployments. Assert success and relevant
state after the second deployment, then clean up. A changed-input reconciliation
test is a different scenario. Neither proves every Azure SKU is idempotent.

## Validate And Iterate

1. Run the targeted native tests with live execution disabled.
2. Distinguish infrastructure defects from incorrect test expectations using the
   agreed contract. Never weaken assertions or update expected output merely to
   turn a failing test green.
3. Revisit the matrix after changes; add missing cases and rerun affected tests.
4. Report commands/outcomes, verified scenarios, remaining gaps, and skipped live
   checks. Clearly distinguish execution, successful assertions, and exclusions.

## Adoption

Copy this directory into a consumer repository's `.github/skills/` directory
(or another skill location supported by its agent). Use `author-bicep-tests`
when asking an agent to author infrastructure tests. The workflow deliberately
does not depend on a particular model or on automatic coverage instrumentation.

## Version-Specific Bicep Reference

When parameter constraints or conditional semantics are unclear, verify them
against official documentation instead of inventing syntax:

- [Parameter files](https://learn.microsoft.com/azure/azure-resource-manager/bicep/parameter-files)
- [Parameter constraints](https://learn.microsoft.com/azure/azure-resource-manager/bicep/parameters)
- [Conditional resources](https://learn.microsoft.com/azure/azure-resource-manager/bicep/conditional-resource-deployment)

Useful Learn MCP queries include `Bicep allowed values union parameter types`,
`Bicep conditional deployment nested resource if`, and `Bicep parameter files
variables using`. These docs describe Bicep, not this independent framework:
verify framework entry points from the consumer's installed package.

If Learn MCP is unavailable, use its CLI equivalent:

| MCP tool | CLI command |
| --- | --- |
| `microsoft_docs_search` | `npx @microsoft/learn-cli search "query"` |
| `microsoft_docs_fetch` | `npx @microsoft/learn-cli fetch "url"` |
| `microsoft_code_sample_search` | `npx @microsoft/learn-cli code-search "query" --language powershell` |
