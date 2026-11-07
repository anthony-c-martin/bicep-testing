const path = require('node:path');
const { BicepTestSession, resourcesOfType, requireResource, getResourceScope } = require('@anthony-c-martin/bicep-testing');

const sampleDescribe = process.env.BICEP_TEST_VALIDATE_ONLY === '1' ? describe.skip : describe;
const infraPath = relativePath => path.resolve(__dirname, '../infra', relativePath);
const metadata = {
  subscriptionId: '28c9069e-23e8-47d2-b640-00d2e0f09616',
  resourceGroup: 'sample-rg',
  location: 'eastus',
};

sampleDescribe('testing the orders infrastructure without deploying it', () => {
  let session;

  beforeAll(async () => {
    session = await BicepTestSession.create('0.47.16');
  }, 60_000);

  afterAll(() => session?.dispose());

  test('storage and Key Vault meet the security baseline', async () => {
    // Context is unnecessary when the assertions only depend on explicit parameter values.
    const result = await session.snapshot(infraPath('security-baseline/secure.bicepparam'));
    expect(result.diagnostics).toHaveLength(0);

    const storage = requireResource(result, { type: 'Microsoft.Storage/storageAccounts', name: 'orderssecurestore' });
    const vault = requireResource(result, { type: 'Microsoft.KeyVault/vaults', name: 'orders-secure-kv' });
    expect(storage.properties).toEqual(expect.objectContaining({
      allowBlobPublicAccess: false,
      allowSharedKeyAccess: false,
      minimumTlsVersion: 'TLS1_2',
      publicNetworkAccess: 'Disabled',
      supportsHttpsTrafficOnly: true,
    }));
    expect(vault.properties).toEqual(expect.objectContaining({
      enablePurgeProtection: true,
      enableRbacAuthorization: true,
      publicNetworkAccess: 'Disabled',
      softDeleteRetentionInDays: 90,
    }));
  });

  test('development uses inexpensive storage and does not create an audit account', async () => {
    const result = await session.snapshot(infraPath('environment-topology/dev.bicepparam'), metadata);
    expect(result.diagnostics).toHaveLength(0);
    expect(resourcesOfType(result, 'Microsoft.Storage/storageAccounts')).toHaveLength(1);

    const primary = requireResource(result, { type: 'Microsoft.Storage/storageAccounts', name: 'ordersdevprimary' });
    expect(primary.sku.name).toBe('Standard_LRS');
    expect(primary.tags).toEqual(expect.objectContaining({ environment: 'dev', dataClassification: 'internal' }));
    expect(result.outputs.primaryStorageId).toBe(primary.id);
    expect(result.outputs.auditStorageId).toBeNull();
  });

  test('production uses resilient primary and audit storage', async () => {
    const result = await session.snapshot(infraPath('environment-topology/prod.bicepparam'), metadata);
    expect(result.diagnostics).toHaveLength(0);
    expect(resourcesOfType(result, 'Microsoft.Storage/storageAccounts')).toHaveLength(2);

    const primary = requireResource(result, { type: 'Microsoft.Storage/storageAccounts', name: 'ordersprodprimary' });
    const audit = requireResource(result, { type: 'Microsoft.Storage/storageAccounts', name: 'ordersprodaudit' });
    expect(primary.sku.name).toBe('Standard_ZRS');
    expect(audit.sku.name).toBe('Standard_GRS');
    expect(primary.tags).toEqual(expect.objectContaining({ environment: 'prod', dataClassification: 'confidential' }));
    expect(result.outputs.primaryStorageId).toBe(primary.id);
    expect(result.outputs.auditStorageId).toBe(audit.id);
  });

  test('private storage is connected to the data subnet and private DNS', async () => {
    const result = await session.snapshot(infraPath('private-network/main.bicepparam'), metadata);
    expect(result.diagnostics).toHaveLength(0);
    const network = requireResource(result, { id: result.outputs.networkIds.virtualNetworkId });
    const appSubnet = requireResource(result, { id: result.outputs.networkIds.appSubnetId });
    const dataSubnet = requireResource(result, { id: result.outputs.networkIds.dataSubnetId });
    const endpoint = requireResource(result, { id: result.outputs.networkIds.privateEndpointId });
    const storage = requireResource(result, { type: 'Microsoft.Storage/storageAccounts', name: 'ordersprivatestore' });
    const appNsg = requireResource(result, { type: 'Microsoft.Network/networkSecurityGroups', name: 'orders-app-nsg' });
    const dataNsg = requireResource(result, { type: 'Microsoft.Network/networkSecurityGroups', name: 'orders-data-nsg' });
    const dnsLink = requireResource(result, {
      type: 'Microsoft.Network/privateDnsZones/virtualNetworkLinks',
      name: 'privatelink.blob.core.windows.net/orders-vnet-link',
    });

    expect(network.properties.addressSpace.addressPrefixes).toEqual(['10.42.0.0/16']);
    expect(appSubnet.properties.addressPrefix).toBe('10.42.1.0/24');
    expect(appSubnet.properties.networkSecurityGroup.id).toBe(appNsg.id);
    expect(dataSubnet.properties.addressPrefix).toBe('10.42.2.0/24');
    expect(dataSubnet.properties.networkSecurityGroup.id).toBe(dataNsg.id);
    expect(dataSubnet.properties.privateEndpointNetworkPolicies).toBe('Disabled');
    expect(endpoint.properties.subnet.id).toBe(dataSubnet.id);
    const connection = endpoint.properties.privateLinkServiceConnections[0].properties;
    expect(connection.groupIds).toEqual(['blob']);
    expect(connection.privateLinkServiceId).toBe(storage.id);
    expect(dnsLink.properties.virtualNetwork.id).toBe(network.id);
  });

  test('the platform policy audits resources missing a cost-center tag', async () => {
    const result = await session.snapshot(infraPath('snapshot-scopes/management-group.bicepparam'), {
      managementGroupId: 'sample-platform',
      location: 'eastus',
    });
    expect(result.diagnostics).toHaveLength(0);
    const policy = requireResource(result, { type: 'Microsoft.Authorization/policyDefinitions', name: 'require-cost-center' });

    expect(getResourceScope(policy.id).managementGroupId).toBe('sample-platform');
    expect(policy.properties.policyRule.if).toEqual({ field: 'tags[costCenter]', exists: 'false' });
    expect(policy.properties.policyRule.then.effect).toBe('audit');
    expect(result.outputs.policyId).toBe(policy.id);
  });
});
