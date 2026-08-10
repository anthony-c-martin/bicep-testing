import { TokenCredential } from '@azure/core-auth';
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
export type DeployOptions = ResourceGroupDeployOptions | SubscriptionDeployOptions | ManagementGroupDeployOptions;
export declare class BicepTestSession {
    private bicep;
    private constructor();
    static create(bicepVersion: string): Promise<BicepTestSession>;
    snapshot(filePath: string, tenantId?: string, subscriptionId?: string, resourceGroup?: string, location?: string, deploymentName?: string): Promise<SnapshotResult>;
    dispose(): void;
}
export declare class LiveBicepTestSession {
    private readonly session;
    private readonly credential;
    private constructor();
    static create(bicepVersion: string, credential: TokenCredential): Promise<LiveBicepTestSession>;
    snapshot(filePath: string, tenantId?: string, subscriptionId?: string, resourceGroup?: string, location?: string, deploymentName?: string): Promise<SnapshotResult>;
    validate(options: DeployOptions): Promise<ValidateResult>;
    deploy(options: DeployOptions): Promise<DeployResult>;
    dispose(): void;
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
    [key: string]: any;
};
export type SnapshotResult = {
    predictedResources: SnapshotResource[];
    diagnostics: string[];
    outputs: Record<string, unknown>;
};
export {};
