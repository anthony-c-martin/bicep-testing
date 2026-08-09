@description('Unique lowercase alphanumeric prefix for globally named test resources.')
param resourcePrefix string

@description('Azure region used by all resources.')
param location string = resourceGroup().location

@description('Creates an additional account used to demonstrate stack reconciliation.')
param includeAuditStorage bool = false

var shortPrefix = take(toLower(resourcePrefix), 8)
var nameSuffix = uniqueString(resourcePrefix, resourceGroup().id)

resource primaryStorage 'Microsoft.Storage/storageAccounts@2023-05-01' = {
  name: '${shortPrefix}pri${nameSuffix}'
  location: location
  tags: {
    purpose: 'bicep-testing-sample'
  }
  sku: {
    name: 'Standard_LRS'
  }
  kind: 'StorageV2'
  properties: {
    allowBlobPublicAccess: false
    allowSharedKeyAccess: false
    minimumTlsVersion: 'TLS1_2'
    publicNetworkAccess: 'Disabled'
    supportsHttpsTrafficOnly: true
  }
}

resource auditStorage 'Microsoft.Storage/storageAccounts@2023-05-01' = if (includeAuditStorage) {
  name: '${shortPrefix}aud${nameSuffix}'
  location: location
  tags: {
    purpose: 'bicep-testing-sample'
  }
  sku: {
    name: 'Standard_LRS'
  }
  kind: 'StorageV2'
  properties: {
    allowBlobPublicAccess: false
    allowSharedKeyAccess: false
    minimumTlsVersion: 'TLS1_2'
    publicNetworkAccess: 'Disabled'
    supportsHttpsTrafficOnly: true
  }
}

output primaryStorageId string = primaryStorage.id
output primaryStorageName string = primaryStorage.name
output primaryBlobEndpoint string = primaryStorage.properties.primaryEndpoints.blob
output auditStorageId string? = includeAuditStorage ? auditStorage.id : null
