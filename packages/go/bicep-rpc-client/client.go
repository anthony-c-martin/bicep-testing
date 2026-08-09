// Package biceprpcclient installs and communicates with the Bicep CLI over JSON-RPC.
package biceprpcclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maximumMessageSize = 64 * 1024 * 1024

// Client communicates with one Bicep CLI process. Calls on a Client are safe for concurrent use.
type Client struct {
	connection net.Conn
	reader     *bufio.Reader
	process    *processHandle
	callMutex  sync.Mutex
	closeOnce  sync.Once
	nextID     int64
	version    string
}

// Configuration controls Bicep CLI selection and caching.
type Configuration struct {
	BicepVersion    string
	ExistingCLIPath string
	CacheRoot       string
}

// Factory installs or locates the Bicep CLI and initializes clients.
type Factory struct{}

// Initialize creates a client using an existing CLI or a cached requested version.
func (Factory) Initialize(ctx context.Context, configuration Configuration) (*Client, error) {
	bicepPath := configuration.ExistingCLIPath
	if bicepPath == "" {
		cacheRoot := configuration.CacheRoot
		if cacheRoot == "" {
			homeDirectory, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("find home directory: %w", err)
			}
			cacheRoot = filepath.Join(homeDirectory, ".bicep", "bin")
		}
		versionDirectory := "latest"
		if configuration.BicepVersion != "" {
			versionDirectory = "v" + configuration.BicepVersion
		}
		var err error
		bicepPath, err = Install(ctx, filepath.Join(cacheRoot, versionDirectory), configuration.BicepVersion)
		if err != nil {
			return nil, err
		}
	}
	return New(ctx, bicepPath)
}

// RPCError is an error returned by the Bicep JSON-RPC server.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (err *RPCError) Error() string {
	return fmt.Sprintf("Bicep RPC error %d: %s", err.Code, err.Message)
}

// New starts the Bicep CLI at bicepPath and validates its RPC protocol version.
func New(ctx context.Context, bicepPath string) (*Client, error) {
	if bicepPath == "" {
		return nil, errors.New("Bicep executable path must not be empty")
	}

	connection, process, err := openConnection(ctx, bicepPath)
	if err != nil {
		return nil, err
	}
	client := &Client{
		connection: connection,
		reader:     bufio.NewReader(connection),
		process:    process,
	}

	version, err := client.Version(ctx)
	if err != nil {
		client.Close()
		return nil, err
	}
	if !versionAtLeast(version, "0.25.3") {
		client.Close()
		return nil, fmt.Errorf("Bicep CLI version %s is not supported; version 0.25.3 or later is required", version)
	}
	return client, nil
}

// Close disconnects from the Bicep CLI and terminates its process.
func (client *Client) Close() error {
	var closeErr error
	client.closeOnce.Do(func() {
		closeErr = client.connection.Close()
		client.process.Close()
	})
	return closeErr
}

// Version returns the Bicep CLI version.
func (client *Client) Version(ctx context.Context) (string, error) {
	if client.version != "" {
		return client.version, nil
	}
	var response struct {
		Version string `json:"version"`
	}
	if err := client.call(ctx, "bicep/version", struct{}{}, &response); err != nil {
		return "", err
	}
	client.version = response.Version
	return client.version, nil
}

// Compile compiles a Bicep file into an ARM template.
func (client *Client) Compile(ctx context.Context, request CompileRequest) (CompileResponse, error) {
	var response CompileResponse
	if err := client.call(ctx, "bicep/compile", request, &response); err != nil {
		return CompileResponse{}, err
	}
	return response, nil
}

// CompileParams compiles a Bicep parameters file into deployable ARM JSON.
func (client *Client) CompileParams(ctx context.Context, request CompileParamsRequest) (CompileParamsResponse, error) {
	var response CompileParamsResponse
	if err := client.call(ctx, "bicep/compileParams", request, &response); err != nil {
		return CompileParamsResponse{}, err
	}
	return response, nil
}

// Format formats a Bicep file using the Bicep CLI formatter.
func (client *Client) Format(ctx context.Context, request FormatRequest) (FormatResponse, error) {
	if err := client.requireVersion(ctx, "0.37.1", "Format"); err != nil {
		return FormatResponse{}, err
	}
	var response FormatResponse
	if err := client.call(ctx, "bicep/format", request, &response); err != nil {
		return FormatResponse{}, err
	}
	return response, nil
}

// GetMetadata returns parameters, outputs, exports, and metadata from a Bicep file.
func (client *Client) GetMetadata(ctx context.Context, request GetMetadataRequest) (GetMetadataResponse, error) {
	var response GetMetadataResponse
	if err := client.call(ctx, "bicep/getMetadata", request, &response); err != nil {
		return GetMetadataResponse{}, err
	}
	return response, nil
}

// GetFileReferences returns every file referenced by a Bicep file.
func (client *Client) GetFileReferences(ctx context.Context, request GetFileReferencesRequest) (GetFileReferencesResponse, error) {
	var response GetFileReferencesResponse
	if err := client.call(ctx, "bicep/getFileReferences", request, &response); err != nil {
		return GetFileReferencesResponse{}, err
	}
	return response, nil
}

// GetDeploymentGraph returns the resource dependency graph for a Bicep file.
func (client *Client) GetDeploymentGraph(ctx context.Context, request GetDeploymentGraphRequest) (GetDeploymentGraphResponse, error) {
	var response GetDeploymentGraphResponse
	if err := client.call(ctx, "bicep/getDeploymentGraph", request, &response); err != nil {
		return GetDeploymentGraphResponse{}, err
	}
	return response, nil
}

// GetSnapshot returns a deployment snapshot for a Bicep parameters file.
func (client *Client) GetSnapshot(ctx context.Context, request GetSnapshotRequest) (GetSnapshotResponse, error) {
	if err := client.requireVersion(ctx, "0.36.1", "GetSnapshot"); err != nil {
		return GetSnapshotResponse{}, err
	}

	var response GetSnapshotResponse
	if err := client.call(ctx, "bicep/getSnapshot", request, &response); err != nil {
		return GetSnapshotResponse{}, err
	}
	return response, nil
}

func (client *Client) requireVersion(ctx context.Context, minimum, operation string) error {
	version, err := client.Version(ctx)
	if err != nil {
		return err
	}
	if !versionAtLeast(version, minimum) {
		return fmt.Errorf("Bicep CLI version %s or later is required for %s; detected %s", minimum, operation, version)
	}
	return nil
}

func (client *Client) call(ctx context.Context, method string, params, result any) error {
	client.callMutex.Lock()
	defer client.callMutex.Unlock()

	client.nextID++
	id := client.nextID
	requestBody, err := json.Marshal(struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int64  `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return fmt.Errorf("encode Bicep RPC request: %w", err)
	}

	stopCancellation := context.AfterFunc(ctx, func() {
		_ = client.connection.SetDeadline(time.Now())
	})
	defer func() {
		stopCancellation()
		_ = client.connection.SetDeadline(time.Time{})
	}()
	if deadline, ok := ctx.Deadline(); ok {
		if err := client.connection.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set Bicep RPC deadline: %w", err)
		}
	}

	var framed bytes.Buffer
	fmt.Fprintf(&framed, "Content-Length: %d\r\n\r\n", len(requestBody))
	framed.Write(requestBody)
	if _, err := io.Copy(client.connection, &framed); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("write Bicep RPC request: %w", err)
	}

	for {
		responseBody, err := client.readMessage()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		var response struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Result  json.RawMessage `json:"result"`
			Error   *RPCError       `json:"error"`
		}
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return fmt.Errorf("decode Bicep RPC response: %w", err)
		}
		var responseID int64
		if len(response.ID) == 0 || json.Unmarshal(response.ID, &responseID) != nil || responseID != id {
			continue
		}
		if response.Error != nil {
			return response.Error
		}
		if err := json.Unmarshal(response.Result, result); err != nil {
			return fmt.Errorf("decode Bicep RPC result: %w", err)
		}
		return nil
	}
}

func (client *Client) readMessage() ([]byte, error) {
	contentLength := -1
	for {
		line, err := client.reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read Bicep RPC header: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		name, value, found := strings.Cut(line, ":")
		if found && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			contentLength, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("read Bicep RPC Content-Length: %w", err)
			}
		}
	}
	if contentLength < 0 {
		return nil, errors.New("Bicep RPC response did not include Content-Length")
	}
	if contentLength > maximumMessageSize {
		return nil, fmt.Errorf("Bicep RPC response exceeds %d bytes", maximumMessageSize)
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(client.reader, body); err != nil {
		return nil, fmt.Errorf("read Bicep RPC body: %w", err)
	}
	return body, nil
}

func versionAtLeast(actual, minimum string) bool {
	actualParts := strings.Split(actual, ".")
	minimumParts := strings.Split(minimum, ".")
	for index := 0; index < len(minimumParts); index++ {
		actualPart := 0
		if index < len(actualParts) {
			actualPart, _ = strconv.Atoi(numericPrefix(actualParts[index]))
		}
		minimumPart, _ := strconv.Atoi(numericPrefix(minimumParts[index]))
		if actualPart != minimumPart {
			return actualPart > minimumPart
		}
	}
	return true
}

func numericPrefix(value string) string {
	for index, character := range value {
		if character < '0' || character > '9' {
			return value[:index]
		}
	}
	return value
}
