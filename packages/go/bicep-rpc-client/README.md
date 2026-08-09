# bicep-rpc-client

An independent Go client for the Bicep CLI JSON-RPC API.

```sh
go get github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client@v0.1.0
```

```go
package main

import (
	"context"
	"fmt"

	biceprpcclient "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client"
)

func main() {
	ctx := context.Background()
	client, err := (biceprpcclient.Factory{}).Initialize(ctx, biceprpcclient.Configuration{
		BicepVersion: "0.46.1",
	})
	if err != nil {
		panic(err)
	}
	defer client.Close()

	result, err := client.Compile(ctx, biceprpcclient.CompileRequest{Path: "main.bicep"})
	if err != nil {
		panic(err)
	}
	if result.Success {
		fmt.Println(result.Contents)
	}
}
```

The factory downloads and caches a requested Bicep CLI version under `~/.bicep/bin`, or connects through `ExistingCLIPath`. The client supports typed compile, format, metadata, file-reference, deployment-graph, snapshot, and version operations. This module is independent and non-official; its API may change between releases.