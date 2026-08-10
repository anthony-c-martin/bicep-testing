import { Bicep } from '@azure/bicep-rpc-client';
import { mkdir } from 'fs/promises';
import os from 'os';
import path from 'path';
import { TokenCredential } from '@azure/core-auth';
import {
  DeploymentParameter,
  DeploymentStack,
  DeploymentStacksClient,
  DeploymentStackValidateResult,
  ErrorDetail,
} from '@azure/arm-resourcesdeploymentstacks';
import { randomUUID } from 'crypto';

type DeployOptionsBase = {
  filePath: string;
  stackName?: string;
  parameterOverrides?: Record<string, unknown>;
};

export type ResourceGroupDeployOptions = DeployOptionsBase & {
  subscriptionId: string;
  resourceGroup: string;
  managementGroupId?: never;
  location?: string;
};

export type SubscriptionDeployOptions = DeployOptionsBase & {
  subscriptionId: string;
  resourceGroup?: never;
  managementGroupId?: never;
  location: string;
};

export type ManagementGroupDeployOptions = DeployOptionsBase & {
  managementGroupId: string;
  subscriptionId?: never;
  resourceGroup?: never;
  location: string;
};

export type DeployOptions =
  | ResourceGroupDeployOptions
  | SubscriptionDeployOptions
  | ManagementGroupDeployOptions;

type DeploymentTarget =
  | { scope: 'resourceGroup'; subscriptionId: string; resourceGroup: string; location?: string }
  | { scope: 'subscription'; subscriptionId: string; location: string }
  | { scope: 'managementGroup'; managementGroupId: string; location: string };

type NormalizedDeployOptions = {
  filePath: string;
  stackName: string;
  parameterOverrides: Record<string, unknown>;
  target: DeploymentTarget;
};

const deleteOptions = {
  unmanageActionManagementGroups: 'delete' as const,
  unmanageActionResourceGroups: 'delete' as const,
  unmanageActionResources: 'delete' as const,
  unmanageActionResourcesWithoutDeleteSupport: 'fail' as const,
};

export class BicepTestSession {
  private constructor(private bicep: Bicep) {}

  public static async create(bicepVersion: string): Promise<BicepTestSession> {
    if (!bicepVersion?.trim()) {
      throw new TypeError('bicepVersion is required.');
    }

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
        tenantId,
        location,
        subscriptionId,
        resourceGroup,
        deploymentName,
      },
    });

    return JSON.parse(response.snapshot);
  }

  /** @internal */
  async compileDeployment(options: NormalizedDeployOptions): Promise<DeploymentStack> {
    const compilation = await this.bicep.compileParams({
      path: path.resolve(options.filePath),
      parameterOverrides: options.parameterOverrides,
    });
    if (!compilation.success || !compilation.template || !compilation.parameters) {
      const diagnostics = compilation.diagnostics
        .map(diagnostic => `${diagnostic.level} ${diagnostic.code}: ${diagnostic.message}`)
        .join('\n');
      throw new Error(`Bicep parameter compilation failed${diagnostics ? `:\n${diagnostics}` : '.'}`);
    }

    const template = JSON.parse(compilation.template) as Record<string, unknown>;
    const parameterFile = JSON.parse(compilation.parameters) as {
      parameters?: Record<string, DeploymentParameter>;
    };
    return {
      location: options.target.location,
      properties: {
        template,
        parameters: parameterFile.parameters ?? {},
        actionOnUnmanage: {
          managementGroups: 'delete',
          resourceGroups: 'delete',
          resources: 'delete',
          resourcesWithoutDeleteSupport: 'fail',
        },
        denySettings: { mode: 'none' },
      },
    };
  }

  dispose(): void {
    this.bicep.dispose();
  }
}

export class LiveBicepTestSession {
  private constructor(
    private readonly session: BicepTestSession,
    private readonly credential: TokenCredential,
  ) {}

  public static async create(
    bicepVersion: string,
    credential: TokenCredential,
  ): Promise<LiveBicepTestSession> {
    if (!credential) {
      throw new TypeError('credential is required.');
    }

    return new LiveBicepTestSession(
      await BicepTestSession.create(bicepVersion),
      credential,
    );
  }

  snapshot(filePath: string, tenantId?: string, subscriptionId?: string, resourceGroup?: string, location?: string, deploymentName?: string): Promise<SnapshotResult> {
    return this.session.snapshot(
      filePath,
      tenantId,
      subscriptionId,
      resourceGroup,
      location,
      deploymentName,
    );
  }

  async validate(options: DeployOptions): Promise<ValidateResult> {
    const normalized = normalizeDeployOptions(options);
    const stack = await this.session.compileDeployment(normalized);
    const client = createDeploymentStacksClient(this.credential, normalized.target);
    const validation = await validateStack(client, normalized, stack);

    return toValidateResult(validation);
  }

  async deploy(options: DeployOptions): Promise<DeployResult> {
    const normalized = normalizeDeployOptions(options);
    const stack = await this.session.compileDeployment(normalized);
    const client = createDeploymentStacksClient(this.credential, normalized.target);
    const teardown = createTeardown(client, normalized);

    try {
      const deployedStack = await deployStack(client, normalized, stack);
      return toDeployResult(deployedStack, teardown);
    } catch (error) {
      return toDeployResult(undefined, teardown, toOperationError(error));
    }
  }

  dispose(): void {
    this.session.dispose();
  }
}

function normalizeDeployOptions(options: DeployOptions): NormalizedDeployOptions {
  if (!options || !options.filePath?.trim()) {
    throw new TypeError('filePath is required.');
  }
  if (options.stackName !== undefined && !options.stackName.trim()) {
    throw new TypeError('stackName must not be empty.');
  }

  const stackName = options.stackName ?? `bicep-test-${randomUUID().replace(/-/g, '')}`;
  const parameterOverrides = options.parameterOverrides ?? {};
  const managementGroupId = options.managementGroupId;
  const subscriptionId = options.subscriptionId;
  const resourceGroup = options.resourceGroup;

  if (managementGroupId !== undefined) {
    if (!managementGroupId.trim()) {
      throw new TypeError('managementGroupId must not be empty.');
    }
    if (subscriptionId !== undefined || resourceGroup !== undefined) {
      throw new TypeError('subscriptionId and resourceGroup must not be set with managementGroupId.');
    }
    if (!options.location?.trim()) {
      throw new TypeError('location is required for management-group deployments.');
    }

    return {
      filePath: options.filePath,
      stackName,
      parameterOverrides,
      target: { scope: 'managementGroup', managementGroupId, location: options.location },
    };
  }

  if (!subscriptionId?.trim()) {
    throw new TypeError('subscriptionId is required.');
  }
  if (resourceGroup !== undefined) {
    if (!resourceGroup.trim()) {
      throw new TypeError('resourceGroup must not be empty.');
    }
    return {
      filePath: options.filePath,
      stackName,
      parameterOverrides,
      target: { scope: 'resourceGroup', subscriptionId, resourceGroup, location: options.location },
    };
  }

  if (!options.location?.trim()) {
    throw new TypeError('location is required for subscription deployments.');
  }

  return {
    filePath: options.filePath,
    stackName,
    parameterOverrides,
    target: { scope: 'subscription', subscriptionId, location: options.location },
  };
}

function createDeploymentStacksClient(
  credential: TokenCredential,
  target: DeploymentTarget,
): DeploymentStacksClient {
  return target.scope === 'managementGroup'
    ? new DeploymentStacksClient(credential)
    : new DeploymentStacksClient(credential, target.subscriptionId);
}

async function deployStack(
  client: DeploymentStacksClient,
  options: NormalizedDeployOptions,
  stack: DeploymentStack,
): Promise<DeploymentStack> {
  switch (options.target.scope) {
    case 'resourceGroup':
      return client.deploymentStacks
        .createOrUpdateAtResourceGroup(options.target.resourceGroup, options.stackName, stack)
        .pollUntilDone();
    case 'subscription':
      return client.deploymentStacks
        .createOrUpdateAtSubscription(options.stackName, stack)
        .pollUntilDone();
    case 'managementGroup':
      return client.deploymentStacks
        .createOrUpdateAtManagementGroup(options.target.managementGroupId, options.stackName, stack)
        .pollUntilDone();
  }
}

async function validateStack(
  client: DeploymentStacksClient,
  options: NormalizedDeployOptions,
  stack: DeploymentStack,
): Promise<DeploymentStackValidateResult> {
  switch (options.target.scope) {
    case 'resourceGroup':
      return client.deploymentStacks
        .validateStackAtResourceGroup(options.target.resourceGroup, options.stackName, stack)
        .pollUntilDone();
    case 'subscription':
      return client.deploymentStacks
        .validateStackAtSubscription(options.stackName, stack)
        .pollUntilDone();
    case 'managementGroup':
      return client.deploymentStacks
        .validateStackAtManagementGroup(options.target.managementGroupId, options.stackName, stack)
        .pollUntilDone();
  }
}

function createTeardown(
  client: DeploymentStacksClient,
  options: NormalizedDeployOptions,
): () => Promise<void> {
  let teardownPromise: Promise<void> | undefined;
  return () => {
    teardownPromise ??= deleteStack(client, options).catch(error => {
      if (getStatusCode(error) !== 404) {
        throw error;
      }
    });
    return teardownPromise;
  };
}

async function deleteStack(
  client: DeploymentStacksClient,
  options: NormalizedDeployOptions,
): Promise<void> {
  switch (options.target.scope) {
    case 'resourceGroup':
      return client.deploymentStacks
        .deleteAtResourceGroup(options.target.resourceGroup, options.stackName, deleteOptions)
        .pollUntilDone();
    case 'subscription':
      return client.deploymentStacks
        .deleteAtSubscription(options.stackName, deleteOptions)
        .pollUntilDone();
    case 'managementGroup':
      return client.deploymentStacks
        .deleteAtManagementGroup(options.target.managementGroupId, options.stackName, deleteOptions)
        .pollUntilDone();
  }
}

function toValidateResult(validation: DeploymentStackValidateResult): ValidateResult {
  const error = validation.error ? fromErrorDetail(validation.error) : undefined;
  return {
    isValid: error === undefined,
    resources: toResources(validation.properties?.validatedResources),
    correlationId: validation.properties?.correlationId,
    error,
  };
}

function toDeployResult(
  stack: DeploymentStack | undefined,
  teardown: () => Promise<void>,
  error?: OperationError,
): DeployResult {
  const outputs = Object.fromEntries(
    Object.entries(stack?.properties?.outputs ?? {}).map(([name, output]) => [
      name,
      output && typeof output === 'object' && 'value' in output ? output.value : output,
    ]),
  );

  return {
    succeeded: error === undefined,
    error,
    errorCode: error?.code,
    errorMessage: error?.message,
    outputs,
    resources: toResources(stack?.properties?.resources),
    teardown,
  };
}

function toResources(
  resources: ReadonlyArray<{ id?: string; type?: string }> | undefined,
): DeploymentResource[] {
  return (resources ?? [])
    .filter((resource): resource is { id: string; type?: string } => typeof resource.id === 'string')
    .map(resource => ({ id: resource.id, type: resource.type }));
}

function fromErrorDetail(error: ErrorDetail): OperationError {
  return { code: error.code, message: error.message, rawData: { ...error } };
}

function toOperationError(error: unknown): OperationError {
  const rawData = getRawErrorData(error);
  const serviceError = isRecord(rawData.error) ? rawData.error : rawData;
  return {
    code: asString(serviceError.code) ?? asString(getRecordValue(error, 'code')) ?? getErrorName(error),
    message: asString(serviceError.message) ?? getErrorMessage(error),
    rawData,
  };
}

function getRawErrorData(error: unknown): Record<string, unknown> {
  const bodyAsText = getRecordValue(getRecordValue(error, 'response'), 'bodyAsText');
  if (typeof bodyAsText === 'string') {
    try {
      const parsed = JSON.parse(bodyAsText) as unknown;
      if (isRecord(parsed)) {
        return parsed;
      }
    } catch {
      // Fall back to the structured SDK error below.
    }
  }

  const details = getRecordValue(error, 'details');
  if (isRecord(details)) {
    return details;
  }

  return { code: asString(getRecordValue(error, 'code')) ?? getErrorName(error), message: getErrorMessage(error) };
}

function getStatusCode(error: unknown): number | undefined {
  const statusCode = getRecordValue(error, 'statusCode');
  if (typeof statusCode === 'number') {
    return statusCode;
  }

  const responseStatus = getRecordValue(getRecordValue(error, 'response'), 'status');
  return typeof responseStatus === 'number' ? responseStatus : undefined;
}

function getErrorName(error: unknown): string {
  return error instanceof Error ? error.name : 'Error';
}

function getErrorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function getRecordValue(value: unknown, key: string): unknown {
  return isRecord(value) ? value[key] : undefined;
}

function asString(value: unknown): string | undefined {
  return typeof value === 'string' ? value : undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

export type OperationError = {
  readonly code?: string;
  readonly message?: string;
  readonly rawData: Readonly<Record<string, unknown>>;
};

export type ValidateResult = {
  readonly isValid: boolean;
  readonly resources: DeploymentResource[];
  readonly correlationId?: string;
  readonly error?: OperationError;
};

export type DeployResult = {
  readonly succeeded: boolean;
  readonly error?: OperationError;
  readonly errorCode?: string;
  readonly errorMessage?: string;
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