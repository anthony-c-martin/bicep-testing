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
	tenant_id="00000000-0000-0000-0000-000000000000",
	subscription_id="00000000-0000-0000-0000-000000000000",
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

## Snapshot results

`snapshot()` returns an immutable `SnapshotResult` containing:

- `predicted_resources`: resources and resolved properties predicted for the deployment
- `outputs`: resolved deployment outputs
- `diagnostics`: Bicep compilation warnings and errors

Each resource exposes its name, type, API version, location, properties, and additional fields returned by Bicep.

## Live deployment testing

Use `deploy()` only when a test must inspect a real Azure resource or service response. The result is also a context manager, so teardown runs when an assertion fails:

```python
import uuid

from azure.identity import DefaultAzureCredential

with session.deploy(
	DefaultAzureCredential(),
	"infra/main.bicepparam",
	subscription_id,
	resource_group,
	f"bicep-test-{uuid.uuid4().hex}",
	{"env": "integration"},
) as deployment:
	assert any(
		resource.type == "Microsoft.Storage/storageAccounts"
		for resource in deployment.resources
	)
```

Live tests require Azure credentials, an existing resource group, and permission to create and delete Deployment Stacks and their managed resources. Closing the result is idempotent and deletes the stack and all resources it manages. Use a unique stack name and keep the result in a context manager.

Lower-level Bicep integrations can use the separately distributed [`anthonycmartin-bicep-rpc-client`](https://github.com/anthony-c-martin/bicep-testing/tree/main/packages/python/bicep_rpc_client).

## More information

- [Runnable pytest snapshot and deployment samples](https://github.com/anthony-c-martin/bicep-testing/tree/main/samples/python)
- [Exported API](https://github.com/anthony-c-martin/bicep-testing/blob/main/api/python/bicep-testing.txt)
- [Project repository](https://github.com/anthony-c-martin/bicep-testing)