# Python packages

The Python implementation is published as two independent distributions with symmetric project layouts.

| Project | Distribution | Import |
| --- | --- | --- |
| [Bicep Testing Framework](bicep_testing/README.md) | `anthonycmartin-bicep-testing` | `anthonycmartin.bicep_testing` |
| [Bicep RPC Client](bicep_rpc_client/README.md) | `anthonycmartin-bicep-rpc-client` | `anthonycmartin.bicep_rpc_client` |

The testing framework depends on the RPC client, while the RPC client can also be used independently for lower-level Bicep integrations. Both distributions participate in the `anthonycmartin` namespace package.

## Development

From the repository root:

```sh
python -m pip install -e "./packages/python/bicep_rpc_client[test]" -e "./packages/python/bicep_testing[test]"
python -m pytest packages/python/bicep_testing/tests packages/python/bicep_rpc_client/tests
python packages/python/scripts/public_api.py --check
```
