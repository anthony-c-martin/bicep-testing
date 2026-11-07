# Bicep Testing Framework for Python

Test the resources, outputs, and diagnostics produced by a Bicep deployment without deploying to Azure. The package can also run opt-in integration tests against a real Azure Deployment Stack when an assertion requires live resources.

This is an independent, non-official project.

## Requirements

- Python 3.11 or later
- A `.bicepparam` entry point for the deployment under test

## Installation

```sh
python -m pip install anthonycmartin-bicep-testing==0.1.6
```

## Snapshot testing

Use `BicepTestSession` as a context manager so its Bicep process is always closed:

```python
from anthonycmartin.bicep_testing import BicepTestSession

with BicepTestSession.create("0.46.1") as session:
	snapshot = session.snapshot("infra/main.bicepparam")

storage_accounts = [
	resource
	for resource in snapshot.predicted_resources
	if resource.type == "Microsoft.Storage/storageAccounts"
]
assert snapshot.diagnostics == ()
assert storage_accounts
assert all(
	resource.properties["allowBlobPublicAccess"] is False
	for resource in storage_accounts
)
```

`BicepTestSession.create()` downloads the requested Bicep CLI version into `~/.bicep/bin` and reuses it on later runs. The session owns a Bicep process, so use it as a context manager or call `close()` after the tests finish.

Snapshot tests run locally. The subscription, tenant, resource group, location, and deployment values provide evaluation context only; they do not need to exist, and no Azure credentials are required.

Metadata is optional. Supply it when assertions depend on those values.
`deployment_name` also affects offline `deployment().name` evaluation.

### Management-group context and helpers (unreleased)

The development checkout adds `SnapshotMetadata.management_group_id`, requiring
Bicep CLI 0.47.16+. Older CLIs raise `RuntimeError`.

```python
from anthonycmartin.bicep_testing import (
    BicepTestSession, SnapshotMetadata, resources_of_type, require_resource,
    get_resource_scope, get_snapshot_value,
)

with BicepTestSession.create("0.47.16") as session:
    snapshot = session.snapshot("infra/management-group.bicepparam", SnapshotMetadata(
        management_group_id="example-platform", location="eastus", deployment_name="offline-test",
    ))
assert snapshot.diagnostics == ()
assert len(resources_of_type(snapshot, "Microsoft.Authorization/policyDefinitions")) == 1
policy = require_resource(snapshot, type="Microsoft.Authorization/policyDefinitions", name="require-cost-center")
assert get_resource_scope(policy.id).management_group_id == "example-platform"
assert get_snapshot_value(policy.properties, "/policyRule/then/effect") == "audit"
```

Use the shared [management-group fixture](../../../samples/infra/snapshot-scopes/).
`require_resource(snapshot, id=resource_id)` is the alternative selector.
Types and IDs match case-insensitively; names match exactly. Type selection
returns an empty tuple when absent; required selection raises `ValueError` for
zero or multiple matches, mixed selectors, or empty arguments.

`get_resource_scope()` returns a frozen `ResourceScope` with `None` for absent
subscription/resource-group/management-group components. It supports tenant,
child, and extension IDs, raises `ValueError` for malformed IDs, and requires GUID
subscription IDs. Tenant IDs cannot be inferred from resource IDs.

`get_snapshot_value()` accepts decoded JSON mappings/lists and an RFC 6901 JSON
Pointer: `""` returns the input, `/items/0` indexes an array, and `~0`/`~1` escape
`~`/`/`. It preserves explicit `None`, raises `KeyError` for missing paths and
noncanonical indices, and raises `ValueError` for invalid syntax. Raw mappings
and resource models remain available.

`BicepTestSession` is intentionally offline-only. Use `LiveBicepTestSession` for Azure validation and deployment.

## Snapshot results

`snapshot()` returns an immutable `SnapshotResult` containing:

- `predicted_resources`: resources and resolved properties predicted for the deployment
- `outputs`: resolved deployment outputs
- `diagnostics`: Bicep compilation warnings and errors

Each resource exposes its name, type, API version, location, properties, and additional fields returned by Bicep.

## Live deployment testing

Use `LiveBicepTestSession` when a test must inspect real Azure resources or service responses. The live session owns the credential and forwards snapshot operations:

```python
from azure.identity import DefaultAzureCredential
from anthonycmartin.bicep_testing import (
	LiveBicepTestSession,
	ResourceGroupDeployOptions,
)

with LiveBicepTestSession.create("0.46.1", DefaultAzureCredential()) as session:
	validation = session.validate(
		ResourceGroupDeployOptions(
			file_path="infra/main.bicepparam",
			subscription_id=subscription_id,
			resource_group=resource_group,
		)
	)
	assert validation.is_valid

	with session.deploy(
		ResourceGroupDeployOptions(
			file_path="infra/main.bicepparam",
			subscription_id=subscription_id,
			resource_group=resource_group,
			parameter_overrides={"env": "integration"},
		)
	) as deployment:
		assert deployment.succeeded
		assert any(
			resource.type == "Microsoft.Storage/storageAccounts"
			for resource in deployment.resources
		)
```

Live tests require Azure credentials and permission to create and delete Deployment Stacks and their managed resources.

Deployment options are immutable dataclasses:

- `ResourceGroupDeployOptions(file_path, subscription_id, resource_group, location=None, stack_name=..., parameter_overrides=...)`
- `SubscriptionDeployOptions(file_path, subscription_id, location, stack_name=..., parameter_overrides=...)`
- `ManagementGroupDeployOptions(file_path, management_group_id, location, stack_name=..., parameter_overrides=...)`

`stack_name` defaults to `bicep-test-<32 lowercase hex characters>`.

`validate()` returns immutable `ValidateResult` with:

- `is_valid`
- `resources`
- `correlation_id`
- `error` (`OperationError` with `code`, `message`, `raw_data`)

`deploy()` returns immutable `DeployResult` for both success and post-submission Azure failures, with:

- `succeeded`
- `error`, `error_code`, `error_message`
- `outputs`
- `resources`

`DeployResult` is a context manager and supports idempotent teardown via `close()`. A `404` during deletion is treated as already removed.

Lower-level Bicep integrations can use the separately distributed [`anthonycmartin-bicep-rpc-client`](https://github.com/anthony-c-martin/bicep-testing/tree/main/packages/python/bicep_rpc_client).

## More information

The [live samples](../../../samples/python/test_deployment.py) include opt-in
identical redeployment with cleanup after both operations. Reuse the options
instance or explicitly supply the same `stack_name`.
Copy the [consumer authoring skill](../../../.github/skills/author-bicep-tests/)
into your repository for AI-generated tests. Automated scenario coverage is not
yet available.

- [Runnable pytest snapshot and deployment samples](https://github.com/anthony-c-martin/bicep-testing/tree/main/samples/python)
- [Exported API](https://github.com/anthony-c-martin/bicep-testing/blob/main/api/python/bicep-testing.txt)
- [Project repository](https://github.com/anthony-c-martin/bicep-testing)