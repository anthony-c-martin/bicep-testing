package biceptest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthony-c-martin/bicep-test/packages/go/rpcclient"
)

// SnapshotMetadata describes the Azure deployment context used to evaluate a snapshot.
type SnapshotMetadata = rpcclient.SnapshotMetadata

type bicepClient interface {
	CompileParams(context.Context, rpcclient.CompileParamsRequest) (rpcclient.CompileParamsResponse, error)
	GetSnapshot(context.Context, rpcclient.GetSnapshotRequest) (rpcclient.GetSnapshotResponse, error)
	Close() error
}

// Tester invokes a pinned Bicep CLI to evaluate infrastructure snapshots.
type Tester struct {
	client bicepClient
}

// New installs the requested Bicep CLI version if needed and starts its RPC client.
func New(ctx context.Context, bicepVersion string) (*Tester, error) {
	if bicepVersion == "" {
		return nil, errors.New("Bicep version must not be empty")
	}
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("find home directory: %w", err)
	}
	installDirectory := filepath.Join(homeDirectory, ".bicep", "bin", "v"+bicepVersion)
	bicepPath, err := rpcclient.Install(ctx, installDirectory, bicepVersion)
	if err != nil {
		return nil, err
	}
	client, err := rpcclient.New(ctx, bicepPath)
	if err != nil {
		return nil, err
	}
	return &Tester{client: client}, nil
}

// Snapshot evaluates a Bicep parameters file without deploying it.
func (tester *Tester) Snapshot(
	ctx context.Context,
	filePath string,
	metadata SnapshotMetadata,
) (*SnapshotResult, error) {
	if filePath == "" {
		return nil, errors.New("Bicep parameters file path must not be empty")
	}
	absolutePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("resolve Bicep parameters file path: %w", err)
	}
	response, err := tester.client.GetSnapshot(ctx, rpcclient.GetSnapshotRequest{
		Path:     absolutePath,
		Metadata: metadata,
	})
	if err != nil {
		return nil, err
	}

	var snapshot SnapshotResult
	if err := json.Unmarshal([]byte(response.Snapshot), &snapshot); err != nil {
		return nil, fmt.Errorf("decode Bicep snapshot: %w", err)
	}
	return &snapshot, nil
}

// Close disconnects from the Bicep CLI and terminates its process.
func (tester *Tester) Close() error {
	if tester == nil || tester.client == nil {
		return nil
	}
	return tester.client.Close()
}
