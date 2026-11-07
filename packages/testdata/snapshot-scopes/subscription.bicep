targetScope = 'subscription'

resource group 'Microsoft.Resources/resourceGroups@2024-03-01' = {
  name: 'snapshot-rg'
  location: 'eastus'
}

output scopeId string = subscription().id
