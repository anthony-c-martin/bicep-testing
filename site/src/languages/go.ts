import { fakeSubscriptionId, fakeTenantId, repositoryUrl } from "./constants";
import type { LanguageSample } from "./types";

export const go: LanguageSample = {
  id: "go",
  label: "Go",
  highlightLanguage: "go",
  packageManager: "Go modules",
  runtime: "Go 1.25+",
  install:
    "go get github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing",
  registry: "pkg.go.dev",
  packageUrl:
    "https://pkg.go.dev/github.com/anthony-c-martin/bicep-testing/packages/go/bicep-testing",
  guideUrl: `${repositoryUrl}/blob/main/packages/go/bicep-testing/README.md`,
  testCommand: "go test ./...",
  offlineStarter: `func TestTemplateCompilesWithoutDiagnostics(t *testing.T) {
  ctx := context.Background()
  session, err := biceptesting.NewSession(ctx, "0.46.1")
  if err != nil { t.Fatal(err) }
  t.Cleanup(func() { _ = session.Close() })

  snapshot, err := session.Snapshot(ctx, "infra/main.bicepparam",
    biceptesting.SnapshotMetadata{
      TenantID:       "${fakeTenantId}",
      SubscriptionID: "${fakeSubscriptionId}",
      ResourceGroup:  "example-rg",
      Location:       "eastus",
    })
  if err != nil { t.Fatal(err) }
  if len(snapshot.Diagnostics) != 0 {
    t.Fatalf("got diagnostics: %v", snapshot.Diagnostics)
  }
}`,
  liveValidateStarter: `credential, err := azidentity.NewDefaultAzureCredential(nil)
if err != nil { t.Fatal(err) }

session, err := biceptesting.NewLiveSession(ctx, "0.46.1", credential)
if err != nil { t.Fatal(err) }
defer session.Close()

validation, err := session.Validate(ctx, biceptesting.DeployOptions{
  FilePath:       "infra/main.bicepparam",
  SubscriptionID: os.Getenv("AZURE_SUBSCRIPTION_ID"),
  ResourceGroup:  os.Getenv("AZURE_RESOURCE_GROUP"),
})
if err != nil { t.Fatal(err) }
if !validation.IsValid { t.Fatal(validation.ErrorMessage) }`,
  liveDeployStarter: `credential, err := azidentity.NewDefaultAzureCredential(nil)
if err != nil { t.Fatal(err) }

session, err := biceptesting.NewLiveSession(ctx, "0.46.1", credential)
if err != nil { t.Fatal(err) }
defer session.Close()

deployment, err := session.Deploy(ctx, biceptesting.DeployOptions{
  FilePath:       "infra/main.bicepparam",
  SubscriptionID: os.Getenv("AZURE_SUBSCRIPTION_ID"),
  ResourceGroup:  os.Getenv("AZURE_RESOURCE_GROUP"),
})
if err != nil { t.Fatal(err) }
defer func() {
  if err := deployment.Teardown(context.Background()); err != nil {
    t.Errorf("teardown: %v", err)
  }
}()
if !deployment.Succeeded { t.Fatal(deployment.ErrorMessage) }`,
  offlineSampleUrl: `${repositoryUrl}/blob/main/samples/go/snapshot_test.go`,
  liveSampleUrl: `${repositoryUrl}/blob/main/samples/go/deployment_test.go`,
};
