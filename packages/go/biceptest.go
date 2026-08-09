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

// Session owns a pinned Bicep CLI used to evaluate and deploy infrastructure under test.
type Session struct {
	client bicepClient
}

// NewSession installs the requested Bicep CLI version if needed and starts its RPC client.
func NewSession(ctx context.Context, bicepVersion string) (*Session, error) {
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
	return &Session{client: client}, nil
}

// Snapshot evaluates a Bicep parameters file without deploying it.
func (session *Session) Snapshot(
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
	response, err := session.client.GetSnapshot(ctx, rpcclient.GetSnapshotRequest{
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
func (session *Session) Close() error {
	if session == nil || session.client == nil {
		return nil
	}
	return session.client.Close()
}
