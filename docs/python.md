# Python

The Python package provides typed helpers for testing the predicted resources, outputs, and diagnostics of a Bicep deployment without deploying to Azure.

## Requirements

- Python 3.11 or later
- A `.bicepparam` entry point for the Bicep deployment under test

## Installation

Until the package is published to PyPI, install it from a local checkout:

```sh
python -m pip install -e ./packages/python
```

## Usage

Use `BicepTestSession` as a context manager so the Bicep JSON-RPC process is always closed:

```python
from bicep_test import BicepTestSession, SnapshotMetadata

metadata = SnapshotMetadata(
    subscription_id="00000000-0000-0000-0000-000000000000",
    resource_group="my-resource-group",
    location="eastus",
    deployment_name="my-deployment",
)

with BicepTestSession.create("0.43.1") as session:
    snapshot = session.snapshot("infra/main.bicepparam", metadata)

storage_accounts = [
    resource
    for resource in snapshot.predicted_resources
    if resource.type == "Microsoft.Storage/storageAccounts"
]
assert storage_accounts
assert all(
    resource.properties["allowBlobPublicAccess"] is False
    for resource in storage_accounts
)
```

`BicepTestSession.create` downloads the requested Bicep CLI version into `~/.bicep/bin` and reuses it on later runs. Snapshot tests do not require Azure credentials or an Azure subscription.

## Snapshot result

`SnapshotResult` contains immutable tuples of `predicted_resources` and `diagnostics`, plus the resolved `outputs`. Each `SnapshotResource` exposes its identity, type, API version, location, properties, and any additional fields returned by Bicep.

## Live deployment tests

Use `deploy` with an Azure credential when a test needs real resources or service behavior. The deployment result is also a context manager, so cleanup runs even when an assertion fails:

```python
from azure.identity import DefaultAzureCredential

with session.deploy(
    DefaultAzureCredential(),
    "infra/main.bicepparam",
    subscription_id,
    resource_group,
    f"storage-test-{uuid.uuid4().hex}",
) as deployment:
    assert any(
        resource.type == "Microsoft.Storage/storageAccounts"
        for resource in deployment.resources
    )
    assert requests.get(deployment.outputs["endpoint"], timeout=30).status_code == 200
```

`DeployResult` exposes normalized outputs and immutable managed-resource data. `close()` is idempotent and deletes the Deployment Stack and its managed resources. Live tests require an existing resource group, Azure credentials, and deployment/deletion permissions.

## Sample

See the runnable [pytest sample](../samples/python/test_snapshot.py) for a complete consumer test using the shared example infrastructure.

## Public API

The complete exported Python API is available in [`api/python/bicep-test.txt`](../api/python/bicep-test.txt).