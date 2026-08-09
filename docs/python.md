# Python

The Python package provides typed helpers for testing the predicted resources, outputs, and diagnostics of a Bicep deployment without deploying to Azure.

## Requirements

- Python 3.11 or later
- A `.bicepparam` entry point for the Bicep deployment under test

## Installation

```sh
python -m pip install anthonycmartin-bicep-testing==0.1.2
```

## Usage

Use `BicepTestSession` as a context manager so the Bicep JSON-RPC process is always closed:

```python
from anthonycmartin.bicep_testing import BicepTestSession, SnapshotMetadata

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

## JSON-RPC client

Most tests should use `BicepTestSession`. Lower-level integrations can install the standalone distribution with `python -m pip install anthonycmartin-bicep-rpc-client==0.1.2` and import `BicepClientFactory`, `BicepClientConfiguration`, and typed request models from `anthonycmartin.bicep_rpc_client`. The factory can download a pinned Bicep CLI or connect through an existing CLI path. The returned client owns the process until `close` is called and supports context-manager cleanup.

```python
from anthonycmartin.bicep_rpc_client import (
    BicepClientConfiguration,
    BicepClientFactory,
    CompileRequest,
)

factory = BicepClientFactory()
with factory.initialize(BicepClientConfiguration(bicep_version="0.46.1")) as client:
    result = client.compile(CompileRequest("infra/main.bicep"))
    if result.success:
        print(result.contents)
```

Use `existing_cli_path` instead of `bicep_version` to connect through an existing installation. Typed operations include `compile`, `compile_params`, `format`, `get_metadata`, `get_file_references`, `get_deployment_graph`, `get_snapshot`, and cached `get_version`.

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

## Public APIs

The exported high-level API is available in [`api/python/bicep-testing.txt`](../api/python/bicep-testing.txt), and the standalone transport API is available in [`api/python/bicep_rpc_client.txt`](../api/python/bicep_rpc_client.txt).