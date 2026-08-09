# Publishing packages

All packages are released together from one repository tag. Pushing a semantic-version tag such as `v0.1.0` starts `.github/workflows/publish.yml`, which verifies the tag against every package manifest, runs the package tests and public API checks, builds the artifacts, and publishes through protected GitHub environments.

The workflow delegates package behavior to `scripts/Publish.ps1`. Select a release unit with `-Package Validate`, `Node`, `DotNet`, `PowerShell`, `Python`, or `Go`, and pass the shared version with `-Version`. The `Python` selector builds both Python distributions together. Use `-SkipPublish` to run validation, tests, and packaging without uploading or creating tags:

```powershell
./scripts/Publish.ps1 -Package Node -Version 0.1.0 -SkipPublish
```

For Python, the script builds both distributions into `artifacts/python`, and one workflow action uploads them together so trusted publishing can use GitHub's OIDC identity.

To exercise the complete release pipeline without publishing, run the **Publish** workflow manually, enter the package version, and leave **dry_run** enabled. Dry-run jobs perform version checks, tests, API checks, and package builds, but skip registry authentication, uploads, and Go tag creation. Tag-triggered runs publish normally.

## One-time registry setup

Create the `npm`, `nuget`, `pypi`, and `psgallery` environments in the GitHub repository. Add required reviewers to each environment so that a tag cannot publish without approval.

| Environment | Registry configuration |
| --- | --- |
| `npm` | On npmjs.com, configure a trusted publisher for package `@anthonycmartin/bicep-testing`, this repository, workflow `publish.yml`, and environment `npm`. No long-lived token is required. |
| `nuget` | On nuget.org, configure a trusted publishing policy for package `AnthonyCMartin.BicepTesting`, this repository, workflow `publish.yml`, and environment `nuget`. Add the policy's nuget.org username as the GitHub environment variable `NUGET_USER`. |
| `pypi` | On PyPI, configure trusted publishers for `anthonycmartin-bicep-testing` and `bicep_rpc_client`, both using this repository, workflow `publish.yml`, and environment `pypi`. No long-lived token is required. |
| `psgallery` | Add a PowerShell Gallery API key as the GitHub environment secret `PSGALLERY_API_KEY`. Scope and rotate the key according to the Gallery account policy. |

The Go proxy requires path-prefixed tags for modules in subdirectories. The workflow creates those compatibility tags automatically from the repository release tag; release authors still create only one tag.

## Prepare a release

1. Update every package manifest to the same version and update dependency constraints in a pull request.
2. Add release notes to the pull request and update user documentation if installation or behavior changed.
3. Run the normal tests, public API check, and package build locally.
4. Merge the pull request before creating the tag. Tags must point to commits on `main`.
5. Create and push one annotated `vX.Y.Z` tag.
6. Approve the protected GitHub environment deployments and verify every package on its registry.

| Release unit | Version source |
| --- | --- |
| Node | `packages/node/package.json` and `package-lock.json` |
| .NET | `packages/dotnet/src/BicepTest/BicepTest.csproj` |
| PowerShell | `packages/powershell/AnthonyCMartin.BicepTesting/AnthonyCMartin.BicepTesting.psd1` |
| Python test library | `packages/python/pyproject.toml` |
| Python RPC client | `packages/python/bicep_rpc_client/pyproject.toml` |
| Go test library and RPC client | Repository tag, converted to path-prefixed tags by the workflow |

For example:

```sh
git switch main
git pull --ff-only
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

Registry versions are immutable. If publishing succeeds but a later verification step fails, fix the issue and release a new patch version rather than reusing the tag.

## Dependency order

The workflow publishes both Python distributions together and the .NET package before the PowerShell module. It creates both Go module tags after testing both modules. Keep the root Go module requirement aligned with the shared release version; the local `replace` directive supports monorepo development and is ignored by consumers.