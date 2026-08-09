# Bicep RPC Client for Python

An independent Python client for the Bicep CLI JSON-RPC API. Install the `anthonycmartin-bicep-rpc-client` distribution and import its public API from `anthonycmartin.bicep_rpc_client`.

```python
from anthonycmartin.bicep_rpc_client import (
    BicepClientConfiguration,
    BicepClientFactory,
    CompileRequest,
)

with BicepClientFactory().initialize(
    BicepClientConfiguration(bicep_version="0.46.1")
) as client:
    result = client.compile(CompileRequest("main.bicep"))
    if result.success:
        print(result.contents)
```

The factory downloads and caches a requested Bicep CLI version under `~/.bicep/bin`, or connects through `existing_cli_path`. The client supports typed compile, format, metadata, file-reference, deployment-graph, snapshot, and version operations. This package is independent and non-official; its API may change between releases.