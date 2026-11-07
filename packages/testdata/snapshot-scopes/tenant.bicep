targetScope = 'tenant'

resource policy 'Microsoft.Authorization/policyDefinitions@2023-04-01' = {
  name: 'require-cost-center'
  properties: {
    policyType: 'Custom'
    mode: 'All'
    policyRule: {
      if: {
        field: 'tags[costCenter]'
        exists: 'false'
      }
      then: {
        effect: 'audit'
      }
    }
  }
}

output tenantId string = tenant().tenantId
