package biceptesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	biceprpcclient "github.com/anthony-c-martin/bicep-testing/packages/go/bicep-rpc-client"
)

// SnapshotMetadata describes the Azure deployment context used to evaluate a snapshot.
type SnapshotMetadata = biceprpcclient.SnapshotMetadata

type bicepClient interface {
	CompileParams(context.Context, biceprpcclient.CompileParamsRequest) (biceprpcclient.CompileParamsResponse, error)
	GetSnapshot(context.Context, biceprpcclient.GetSnapshotRequest) (biceprpcclient.GetSnapshotResponse, error)
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
	client, err := (biceprpcclient.Factory{}).Initialize(ctx, biceprpcclient.Configuration{
		BicepVersion: bicepVersion,
	})
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
	response, err := session.client.GetSnapshot(ctx, biceprpcclient.GetSnapshotRequest{
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
