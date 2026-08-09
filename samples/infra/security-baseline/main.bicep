@description('Short lowercase prefix used to name resources.')
param resourcePrefix string

@description('Azure region used by all resources.')
param location string = resourceGroup().location

@description('Enables the security baseline. False is used only to demonstrate regression detection.')
param enforceSecurity bool = true

resource storage 'Microsoft.Storage/storageAccounts@2023-05-01' = {
  name: '${resourcePrefix}securestore'
  location: location
  sku: {
    name: 'Standard_LRS'
  }
  kind: 'StorageV2'
  properties: {
    allowBlobPublicAccess: !enforceSecurity
    allowSharedKeyAccess: !enforceSecurity
    minimumTlsVersion: enforceSecurity ? 'TLS1_2' : 'TLS1_0'
    publicNetworkAccess: enforceSecurity ? 'Disabled' : 'Enabled'
    supportsHttpsTrafficOnly: enforceSecurity
  }
}

resource keyVault 'Microsoft.KeyVault/vaults@2023-07-01' = {
  name: '${resourcePrefix}-secure-kv'
  location: location
  properties: {
    enablePurgeProtection: enforceSecurity
    enableRbacAuthorization: enforceSecurity
    enableSoftDelete: true
    publicNetworkAccess: enforceSecurity ? 'Disabled' : 'Enabled'
    softDeleteRetentionInDays: enforceSecurity ? 90 : 7
    sku: {
      family: 'A'
      name: 'standard'
    }
    tenantId: tenant().tenantId
  }
}

output storageId string = storage.id
output keyVaultId string = keyVault.id
