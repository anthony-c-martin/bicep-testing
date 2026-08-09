@description('Short lowercase name for the workload.')
param workloadName string

@description('Deployment environment represented by this topology.')
param environmentName 'dev' | 'prod'

@description('Azure region used by all resources.')
param location string = resourceGroup().location

var isProduction = environmentName == 'prod'
var commonTags = {
  environment: environmentName
  workload: workloadName
  dataClassification: isProduction ? 'confidential' : 'internal'
}

resource primaryStorage 'Microsoft.Storage/storageAccounts@2023-05-01' = {
  name: '${workloadName}${environmentName}primary'
  location: location
  tags: commonTags
  sku: {
    name: isProduction ? 'Standard_ZRS' : 'Standard_LRS'
  }
  kind: 'StorageV2'
  properties: {
    allowBlobPublicAccess: false
    minimumTlsVersion: 'TLS1_2'
    supportsHttpsTrafficOnly: true
  }
}

resource auditStorage 'Microsoft.Storage/storageAccounts@2023-05-01' = if (isProduction) {
  name: '${workloadName}${environmentName}audit'
  location: location
  tags: commonTags
  sku: {
    name: 'Standard_GRS'
  }
  kind: 'StorageV2'
  properties: {
    allowBlobPublicAccess: false
    minimumTlsVersion: 'TLS1_2'
    supportsHttpsTrafficOnly: true
  }
}

output primaryStorageId string = primaryStorage.id
output auditStorageId string? = isProduction ? auditStorage.id : null
