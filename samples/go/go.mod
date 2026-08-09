module github.com/anthony-c-martin/bicep-test/samples/go

go 1.24

require github.com/anthony-c-martin/bicep-test/packages/go v0.0.0

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.18.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armdeploymentstacks v1.0.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
)

replace github.com/anthony-c-martin/bicep-test/packages/go => ../../packages/go
