# Bicep RPC Client for Python

An independent Python client for programmatically interacting with the [Bicep CLI](https://github.com/Azure/bicep) over JSON-RPC.

## Getting started

### Install the package

```sh
python -m pip install anthonycmartin-bicep-rpc-client==0.1.1
```

The distribution is named `anthonycmartin-bicep-rpc-client`; import its public API from `anthonycmartin.bicep_rpc_client`.

### Initialize the client

`BicepClientFactory.initialize()` downloads and caches the requested Bicep CLI version, starts `bicep jsonrpc --stdio`, and returns a client. Use the client as a context manager so the Bicep process is always terminated.

```python
from anthonycmartin.bicep_rpc_client import BicepClientFactory

with BicepClientFactory().initialize() as client:
    print(f"Bicep CLI version: {client.get_version()}")
```

With no configuration, the factory resolves and caches the latest Bicep CLI under `~/.bicep/bin`.

### Pin a specific Bicep version

Set `bicep_version` without a leading `v`. Versioned CLIs are cached separately.

```python
from anthonycmartin.bicep_rpc_client import BicepClientConfiguration, BicepClientFactory

client = BicepClientFactory().initialize(
    BicepClientConfiguration(bicep_version="0.46.1")
)
```

### Use an existing Bicep installation

Set `existing_cli_path` to skip downloading the CLI. This takes precedence over `bicep_version` and `cache_root`.

```python
client = BicepClientFactory().initialize(
    BicepClientConfiguration(existing_cli_path="/usr/local/bin/bicep")
)
```

### Use a custom cache directory

```python
client = BicepClientFactory().initialize(
    BicepClientConfiguration(
        bicep_version="0.46.1",
        cache_root="/var/cache/bicep",
    )
)
```

`BicepClientConfiguration` accepts `bicep_version`, `existing_cli_path`, and `cache_root`; every field is optional and paths may be strings or `pathlib.Path` values.

## Available operations

All operations communicate synchronously with one Bicep CLI process. Relative request paths are resolved to absolute paths before they are sent. Bicep compilation failures are returned in typed response diagnostics; transport failures raise standard Python exceptions and JSON-RPC failures raise `RpcError`.

### compile

`compile()` compiles a `.bicep` file into an ARM template JSON string.

```python
from anthonycmartin.bicep_rpc_client import CompileRequest

result = client.compile(CompileRequest("./main.bicep"))
if result.success:
    print(result.contents)
else:
    print(result.diagnostics)
```

`CompileRequest` contains `path`. `CompileResponse` contains `success`, `diagnostics`, and `contents`; `contents` is `None` when the CLI does not return compiled output.

### compile_params

`compile_params()` compiles a `.bicepparam` file into ARM deployment parameters and accepts JSON-serializable parameter overrides.

```python
from anthonycmartin.bicep_rpc_client import CompileParamsRequest

result = client.compile_params(
    CompileParamsRequest(
        "./main.bicepparam",
        parameter_overrides={
            "environment": "test",
            "instanceCount": 2,
        },
    )
)
if result.success:
    print(result.parameters)
    print(result.template)
    print(result.template_spec_id)
```

`CompileParamsRequest` contains `path` and `parameter_overrides`. `CompileParamsResponse` contains `success`, `diagnostics`, `parameters`, `template`, and `template_spec_id`. The three content fields may be `None`; `template_spec_id` is populated when the parameters file references a template spec.

### format

`format()` applies the standard Bicep formatter and returns the formatted source. It requires Bicep CLI 0.37.1 or later and does not write the file.

```python
from pathlib import Path

from anthonycmartin.bicep_rpc_client import FormatRequest

result = client.format(FormatRequest("./main.bicep"))
Path("./main.bicep").write_text(result.contents, encoding="utf-8")
```

`FormatRequest` contains `path`. `FormatResponse` contains `contents`.

### get_metadata

`get_metadata()` returns the parameters, outputs, exports, and file-level metadata declared by a Bicep file.

```python
from anthonycmartin.bicep_rpc_client import GetMetadataRequest

result = client.get_metadata(GetMetadataRequest("./main.bicep"))
for parameter in result.parameters:
    print(f"parameter {parameter['name']}")
for output in result.outputs:
    print(f"output {output['name']}")
for exported in result.exports:
    print(f"export {exported['name']}")
for item in result.metadata:
    print(f"metadata {item['name']}")
```

`GetMetadataRequest` contains `path`. `GetMetadataResponse` exposes `parameters`, `outputs`, `exports`, and `metadata` as tuples of JSON-shaped dictionaries so fields added by future Bicep versions remain available.

### get_file_references

`get_file_references()` returns every file referenced by a Bicep file, including modules, loaded files, and the entry point.

```python
from anthonycmartin.bicep_rpc_client import GetFileReferencesRequest

result = client.get_file_references(GetFileReferencesRequest("./main.bicep"))
for file_path in result.file_paths:
    print(file_path)
```

`GetFileReferencesRequest` contains `path`. `GetFileReferencesResponse` contains the `file_paths` tuple.

### get_deployment_graph

`get_deployment_graph()` returns resource nodes and dependency edges for visualization or graph analysis.

```python
from anthonycmartin.bicep_rpc_client import GetDeploymentGraphRequest

result = client.get_deployment_graph(GetDeploymentGraphRequest("./main.bicep"))
for node in result.nodes:
    print(f"node {node['name']} ({node['type']})")
for edge in result.edges:
    print(f"{edge['source']} -> {edge['target']}")
```

`GetDeploymentGraphRequest` contains `path`. `GetDeploymentGraphResponse` exposes `nodes` and `edges` as tuples of JSON-shaped dictionaries.

### get_snapshot

`get_snapshot()` evaluates a `.bicepparam` file using an Azure deployment context and returns a serialized deployment snapshot. It requires Bicep CLI 0.36.1 or later but does not deploy resources or require Azure credentials.

```python
from anthonycmartin.bicep_rpc_client import (
    GetSnapshotRequest,
    SnapshotExternalInput,
    SnapshotMetadata,
)

with BicepClientFactory().initialize(
    BicepClientConfiguration(bicep_version="0.46.1")
) as client:
    result = client.get_snapshot(
        GetSnapshotRequest(
            "./main.bicepparam",
            metadata=SnapshotMetadata(
                tenant_id="00000000-0000-0000-0000-000000000000",
                subscription_id="00000000-0000-0000-0000-000000000000",
                resource_group="my-resource-group",
                location="eastus",
                deployment_name="my-deployment",
            ),
            external_inputs=(
                SnapshotExternalInput(
                    kind="sys.envVar",
                    config="BUILD_ID",
                    value="1234",
                ),
            ),
        )
    )
    print(result.snapshot)
```

`SnapshotMetadata` accepts optional `tenant_id`, `subscription_id`, `resource_group`, `location`, and `deployment_name` values. `SnapshotExternalInput` contains `kind`, `value`, and optional `config`. `GetSnapshotRequest` contains `path`, `metadata`, and the `external_inputs` tuple. `GetSnapshotResponse` contains the JSON snapshot in `snapshot`.

### get_version

`get_version()` returns the connected Bicep CLI version. The first result is cached for the lifetime of the client.

```python
version = client.get_version()
print(version)
```

## Client lifecycle

`BicepClient` owns the Bicep subprocess. Prefer a `with` block, or call `close()` explicitly. `close()` first asks the process to terminate, waits up to five seconds, and then kills it if necessary. Calling `close()` after the process exits has no effect.

```python
client = BicepClientFactory().initialize(
    BicepClientConfiguration(bicep_version="0.46.1")
)
try:
    result = client.compile(CompileRequest("./main.bicep"))
finally:
    client.close()
```

`BicepClient(executable)` is also public for callers that already own Bicep installation and want to start the process directly. `BicepClientFactory.initialize()` is preferred because it locates or installs the CLI and verifies that the process responds to `get_version()` before returning.

## Error handling

JSON-RPC server errors raise `RpcError`, a `RuntimeError` subclass with `code`, `message`, and `data` attributes.

```python
from anthonycmartin.bicep_rpc_client import RpcError

try:
    result = client.compile(CompileRequest("./main.bicep"))
except RpcError as error:
    print(f"Bicep RPC error {error.code}: {error.message}")
    print(error.data)
```

Unsupported operation versions raise `RuntimeError`. A closed JSON-RPC output stream raises `EOFError`; download, process, serialization, and file errors retain their standard Python exception types.

Automatic downloads support Windows, Linux, and macOS on x64 and Arm64.

## Public API summary

| API | Purpose |
| --- | --- |
| `BicepClientConfiguration`, `BicepClientFactory.initialize()` | Select, cache, and start a Bicep CLI. |
| `BicepClient`, `BicepClient.close()` | Own a JSON-RPC connection and Bicep process. |
| `CompileRequest`, `CompileResponse`, `BicepClient.compile()` | Compile a `.bicep` file. |
| `CompileParamsRequest`, `CompileParamsResponse`, `BicepClient.compile_params()` | Compile a `.bicepparam` file with optional overrides. |
| `FormatRequest`, `FormatResponse`, `BicepClient.format()` | Format Bicep source. |
| `GetMetadataRequest`, `GetMetadataResponse`, `BicepClient.get_metadata()` | Read declarations and metadata. |
| `GetFileReferencesRequest`, `GetFileReferencesResponse`, `BicepClient.get_file_references()` | List referenced files. |
| `GetDeploymentGraphRequest`, `GetDeploymentGraphResponse`, `BicepClient.get_deployment_graph()` | Read resource graph nodes and edges. |
| `GetSnapshotRequest`, `SnapshotMetadata`, `SnapshotExternalInput`, `GetSnapshotResponse`, `BicepClient.get_snapshot()` | Evaluate a deployment snapshot. |
| `BicepClient.get_version()` | Read the connected CLI version. |
| `RpcError` | Inspect a JSON-RPC server error. |

All request, response, metadata, external-input, and configuration models are immutable slotted dataclasses.

This package is independent and non-official. Its API may change between releases.