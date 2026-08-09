@description('Short lowercase prefix used to name resources.')
param resourcePrefix string

@description('Azure region used by all resources.')
param location string = resourceGroup().location

@description('Private DNS zone for the storage blob endpoint in the target cloud.')
param blobPrivateDnsZoneName string

resource appNsg 'Microsoft.Network/networkSecurityGroups@2023-11-01' = {
  name: '${resourcePrefix}-app-nsg'
  location: location
}

resource dataNsg 'Microsoft.Network/networkSecurityGroups@2023-11-01' = {
  name: '${resourcePrefix}-data-nsg'
  location: location
}

resource virtualNetwork 'Microsoft.Network/virtualNetworks@2023-11-01' = {
  name: '${resourcePrefix}-vnet'
  location: location
  properties: {
    addressSpace: {
      addressPrefixes: [
        '10.42.0.0/16'
      ]
    }
  }
}

resource appSubnet 'Microsoft.Network/virtualNetworks/subnets@2023-11-01' = {
  parent: virtualNetwork
  name: 'app'
  properties: {
    addressPrefix: '10.42.1.0/24'
    networkSecurityGroup: {
      id: appNsg.id
    }
  }
}

resource dataSubnet 'Microsoft.Network/virtualNetworks/subnets@2023-11-01' = {
  parent: virtualNetwork
  name: 'data'
  properties: {
    addressPrefix: '10.42.2.0/24'
    networkSecurityGroup: {
      id: dataNsg.id
    }
    privateEndpointNetworkPolicies: 'Disabled'
  }
}

resource storage 'Microsoft.Storage/storageAccounts@2023-05-01' = {
  name: '${resourcePrefix}privatestore'
  location: location
  sku: {
    name: 'Standard_LRS'
  }
  kind: 'StorageV2'
  properties: {
    allowBlobPublicAccess: false
    minimumTlsVersion: 'TLS1_2'
    publicNetworkAccess: 'Disabled'
    supportsHttpsTrafficOnly: true
  }
}

resource privateDnsZone 'Microsoft.Network/privateDnsZones@2020-06-01' = {
  name: blobPrivateDnsZoneName
  location: 'global'
}

resource dnsLink 'Microsoft.Network/privateDnsZones/virtualNetworkLinks@2020-06-01' = {
  parent: privateDnsZone
  name: '${resourcePrefix}-vnet-link'
  location: 'global'
  properties: {
    registrationEnabled: false
    virtualNetwork: {
      id: virtualNetwork.id
    }
  }
}

resource privateEndpoint 'Microsoft.Network/privateEndpoints@2023-11-01' = {
  name: '${resourcePrefix}-storage-pe'
  location: location
  properties: {
    privateLinkServiceConnections: [
      {
        name: 'blob'
        properties: {
          groupIds: [
            'blob'
          ]
          privateLinkServiceId: storage.id
        }
      }
    ]
    subnet: {
      id: dataSubnet.id
    }
  }
}

resource dnsZoneGroup 'Microsoft.Network/privateEndpoints/privateDnsZoneGroups@2023-11-01' = {
  parent: privateEndpoint
  name: 'default'
  properties: {
    privateDnsZoneConfigs: [
      {
        name: 'blob'
        properties: {
          privateDnsZoneId: privateDnsZone.id
        }
      }
    ]
  }
}

output networkIds object = {
  appSubnetId: appSubnet.id
  dataSubnetId: dataSubnet.id
  privateEndpointId: privateEndpoint.id
  virtualNetworkId: virtualNetwork.id
}
