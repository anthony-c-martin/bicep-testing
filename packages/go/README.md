# Go modules

The Go implementation is published as two independent sibling modules.

| Project | Module |
| --- | --- |
| [Bicep Testing Framework](bicep-testing/README.md) | `github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing` |
| [Bicep RPC Client](bicep-rpc-client/README.md) | `github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client` |

The testing framework depends on the RPC client, while the RPC client can also be used independently for lower-level Bicep integrations.

## Development

From the repository root:

```sh
cd packages/go/bicep-testing
go test ./...
go run ./internal/apidoc --check

cd ../bicep-rpc-client
go test ./...
```
