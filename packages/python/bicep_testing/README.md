# Bicep Testing Framework for Python

Test the resources, outputs, and diagnostics produced by a Bicep deployment without deploying to Azure. The package can also run opt-in integration tests against a real Azure Deployment Stack when an assertion requires live resources.

This is an independent, non-official project.

## Requirements

- Python 3.11 or later
- A `.bicepparam` entry point for the deployment under test

## Installation

```sh
python -m pip install anthonycmartin-bicep-testing==0.1.4
```

## Snapshot testing

Use `BicepTestSession` as a context manager so its Bicep process is always closed:

```python
from anthonycmartin.bicep_testing import BicepTestSession, SnapshotMetadata

metadata = SnapshotMetadata(
	tenant_id="ddbe463a-0554-485d-b589-0b17d60cd38b",
	subscription_id="28c9069e-23e8-47d2-b640-00d2e0f09616",
	resource_group="example-rg",
	location="eastus",
	deployment_name="example-deployment",
)

with BicepTestSession.create("0.46.1") as session:
	snapshot = session.snapshot("infra/main.bicepparam", metadata)

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

- [Runnable pytest snapshot and deployment samples](https://github.com/anthony-c-martin/bicep-testing/tree/main/samples/python)
- [Exported API](https://github.com/anthony-c-martin/bicep-testing/blob/main/api/python/bicep-testing.txt)
- [Project repository](https://github.com/anthony-c-martin/bicep-testing)