module github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing

go 1.25.0

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.22.0
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armdeploymentstacks v1.0.1
	github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client v0.1.4
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.12.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client => ../bicep-rpc-client
