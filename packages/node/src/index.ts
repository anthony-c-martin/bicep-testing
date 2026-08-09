import { Bicep } from '@azure/bicep-rpc-client';
import { mkdir } from 'fs/promises';
import os from 'os';
import path from 'path';
import { TokenCredential } from '@azure/core-auth';
import { DeploymentParameter, DeploymentStacksClient } from '@azure/arm-resourcesdeploymentstacks';

export type DeployOptions = {
  filePath: string;
  subscriptionId: string;
  resourceGroup: string;
  stackName: string;
  parameterOverrides?: Record<string, unknown>;
};

export class BicepTestSession {
  constructor(private bicep: Bicep) {}

  public static async create(bicepVersion: string) {
    const basePath = path.join(os.homedir(), '.bicep', 'bin', `v${bicepVersion}`);
    await mkdir(basePath, { recursive: true });
    const bicepPath = await Bicep.install(basePath, bicepVersion);
    const bicep = await Bicep.initialize(bicepPath);

    return new BicepTestSession(bicep);
  }

  async snapshot(filePath: string, tenantId?: string, subscriptionId?: string, resourceGroup?: string, location?: string, deploymentName?: string): Promise<SnapshotResult> {
    const response = await this.bicep.getSnapshot({
      path: filePath,
      metadata: {
        tenantId: tenantId,
        location: location,
        subscriptionId: subscriptionId,
        resourceGroup: resourceGroup,
        deploymentName: deploymentName
      }
    });

    return JSON.parse(response.snapshot);
  }

  async deploy(credential: TokenCredential, options: DeployOptions): Promise<DeployResult> {
    const compilation = await this.bicep.compileParams({
      path: path.resolve(options.filePath),
      parameterOverrides: options.parameterOverrides ?? {},
    });
    if (!compilation.success || !compilation.template || !compilation.parameters) {
      const diagnostics = compilation.diagnostics
        .map(diagnostic => `${diagnostic.level} ${diagnostic.code}: ${diagnostic.message}`)
        .join('\n');
      throw new Error(`Bicep parameter compilation failed${diagnostics ? `:\n${diagnostics}` : '.'}`);
    }

    const template = JSON.parse(compilation.template) as Record<string, unknown>;
    const parameterFile = JSON.parse(compilation.parameters) as { parameters?: Record<string, DeploymentParameter> };
    const client = new DeploymentStacksClient(credential, options.subscriptionId);
    const stack = await client.deploymentStacks.beginCreateOrUpdateAtResourceGroupAndWait(options.resourceGroup, options.stackName, {
      properties: {
        template,
        parameters: parameterFile.parameters ?? {},
        actionOnUnmanage: {
          managementGroups: 'delete',
          resourceGroups: 'delete',
          resources: 'delete',
          resourcesWithoutDeleteSupport: 'fail',
        },
        denySettings: {
          mode: 'none'
        }
      }
    });

    const outputs = Object.fromEntries(
      Object.entries(stack.properties?.outputs ?? {}).map(([name, output]) => [
        name,
        output && typeof output === 'object' && 'value' in output ? output.value : output,
      ]),
    );
    const resources = (stack.properties?.resources ?? [])
      .filter((resource): resource is typeof resource & { id: string } => typeof resource.id === 'string')
      .map(resource => ({ id: resource.id, type: resource.type }));

    let teardownPromise: Promise<void> | undefined;
    return {
      outputs,
      resources,
      teardown: () => {
        teardownPromise ??= client.deploymentStacks.beginDeleteAtResourceGroupAndWait(
          options.resourceGroup,
          options.stackName,
          {
            unmanageActionManagementGroups: 'delete',
            unmanageActionResourceGroups: 'delete',
            unmanageActionResources: 'delete',
            unmanageActionResourcesWithoutDeleteSupport: 'fail',
          },
        );
        return teardownPromise;
      },
    };
  }

  dispose() {
    this.bicep.dispose();
  }
}

export type DeployResult = {
  readonly outputs: Record<string, unknown>;
  readonly resources: DeploymentResource[];
  teardown(): Promise<void>;
};

export type DeploymentResource = {
  id: string;
  type?: string;
};

export type SnapshotResource = {
  id: string;
  type: string;
  name: string;
  apiVersion: string;
  location?: string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  [key: string]: any;
};

export type SnapshotResult = {
  predictedResources: SnapshotResource[];
  diagnostics: string[];
  outputs: Record<string, unknown>;
};