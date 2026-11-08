module github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing

go 1.25.0

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.23.3
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armdeploymentstacks v1.0.1
	github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client v0.1.7
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.13.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

replace github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client => ../bicep-rpc-client
