module github.com/anthony-c-martin/bicep-testing/packages/go

go 1.24

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.18.1
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armdeploymentstacks v1.0.1
	github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client v0.0.0
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
)

replace github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client => ./bicep-rpc-client
