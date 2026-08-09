import * as path from 'path';
import { BicepTestSession, SnapshotResult } from "../src";

const TENANT_ID = '00000000-0000-0000-0000-000000000000';
const SUBSCRIPTION_ID = '00000000-0000-0000-0000-000000000000';
const RESOURCE_GROUP = 'test-rg';
const LOCATION = 'eastus';
const DEPLOYMENT_NAME = 'test-deployment';

let tester: BicepTestSession;
let snapshot: SnapshotResult;

async function onBeforeAll() {
  tester = await BicepTestSession.create('0.43.1');
  snapshot = await tester.snapshot(
    path.join(__dirname, 'samples/snapshot/main.bicepparam'),
    TENANT_ID,
    SUBSCRIPTION_ID,
    RESOURCE_GROUP,
    LOCATION,
    DEPLOYMENT_NAME,
  );
}

function onAfterAll() {
  tester?.dispose();
}

beforeAll(onBeforeAll, 60000);
afterAll(onAfterAll);

describe("Snapshot helper", () => {
  it("should produce no diagnostics", () => {
    expect(snapshot.diagnostics).toHaveLength(0);
  });

  it("should contain exactly 2 storage accounts", () => {
    expect(snapshot.predictedResources.filter(r => r.type === 'Microsoft.Storage/storageAccounts')).toHaveLength(2);
  });

  it("should contain exactly 1 key vault", () => {
    expect(snapshot.predictedResources.filter(r => r.type === 'Microsoft.KeyVault/vaults')).toHaveLength(1);
  });

  it("should contain no virtual networks", () => {
    expect(snapshot.predictedResources.filter(r => r.type === 'Microsoft.Network/virtualNetworks')).toHaveLength(0);
  });

  it("all storage accounts should have public blob access disabled", () => {
    snapshot.predictedResources
      .filter(r => r.type === 'Microsoft.Storage/storageAccounts')
      .forEach(r => {
        expect(r.properties?.allowBlobPublicAccess).toBe(false);
      });
  });

  it("all storage accounts should enforce TLS 1.2", () => {
    snapshot.predictedResources
      .filter(r => r.type === 'Microsoft.Storage/storageAccounts')
      .forEach(r => {
        expect(r.properties?.minimumTlsVersion).toBe('TLS1_2');
      });
  });

  it("key vault should have soft delete enabled", () => {
    snapshot.predictedResources
      .filter(r => r.type === 'Microsoft.KeyVault/vaults')
      .forEach(r => {
        expect(r.properties?.enableSoftDelete).toBe(true);
      });
  });

  it("key vault should have a 90-day soft delete retention period", () => {
    snapshot.predictedResources
      .filter(r => r.type === 'Microsoft.KeyVault/vaults')
      .forEach(r => {
        expect(r.properties?.softDeleteRetentionInDays).toBe(90);
      });
  });

  it("all resources should be deployed to the expected location", () => {
    snapshot.predictedResources.forEach(r => {
      expect(r.location?.toLowerCase()).toBe(LOCATION.toLowerCase());
    });
  });

  it("all storage accounts should be deployed to the expected location", () => {
    snapshot.predictedResources
      .filter(r => r.type === 'Microsoft.Storage/storageAccounts')
      .forEach(r => {
        expect(r.location?.toLowerCase()).toBe(LOCATION.toLowerCase());
      });
  });

  it("specific storage accounts exist by name", () => {
    const storageNames = snapshot.predictedResources
      .filter(r => r.type === 'Microsoft.Storage/storageAccounts')
      .map(r => r.name);
    expect(storageNames).toContain('testprimary');
    expect(storageNames).toContain('testbackup');
  });

  it("primary storage account output should be set", () => {
    expect(snapshot.outputs['primaryStorageId']).toBeDefined();
  });

  it("primary storage account output should reference the correct resource", () => {
    expect(snapshot.outputs['primaryStorageId']).toBe(
      `/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP}/providers/Microsoft.Storage/storageAccounts/testprimary`,
    );
  });
});
