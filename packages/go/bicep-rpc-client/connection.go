package biceprpcclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os/exec"
	"sync"
)

type processHandle struct {
	command *exec.Cmd
	wait    <-chan error
	close   sync.Once
}

func (process *processHandle) Close() {
	process.close.Do(func() {
		if process.command.Process != nil {
			_ = process.command.Process.Kill()
		}
		<-process.wait
	})
}

type acceptResult struct {
	connection net.Conn
	err        error
}

func openConnection(ctx context.Context, bicepPath string) (net.Conn, *processHandle, error) {
	pipeName, err := randomPipeName()
	if err != nil {
		return nil, nil, err
	}

	listener, cleanup, err := listenPipe(pipeName)
	if err != nil {
		return nil, nil, fmt.Errorf("listen on Bicep RPC pipe: %w", err)
	}
	defer cleanup()

	command := exec.Command(bicepPath, "jsonrpc", "--pipe", pipeName)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		listener.Close()
		return nil, nil, fmt.Errorf("start Bicep RPC process: %w", err)
	}

	wait := make(chan error, 1)
	go func() {
		wait <- command.Wait()
	}()
	process := &processHandle{command: command, wait: wait}

	accepted := make(chan acceptResult, 1)
	go func() {
		connection, err := listener.Accept()
		accepted <- acceptResult{connection: connection, err: err}
	}()

	select {
	case result := <-accepted:
		_ = listener.Close()
		if result.err != nil {
			process.Close()
			return nil, nil, fmt.Errorf("accept Bicep RPC connection: %w", result.err)
		}
		return result.connection, process, nil
	case err := <-wait:
		_ = listener.Close()
		message := stderr.String()
		if message == "" {
			message = err.Error()
		}
		return nil, nil, fmt.Errorf("Bicep RPC process exited before connecting: %s", message)
	case <-ctx.Done():
		_ = listener.Close()
		process.Close()
		return nil, nil, fmt.Errorf("connect to Bicep RPC process: %w", ctx.Err())
	}
}

func randomPipeName() (string, error) {
	randomBytes := make([]byte, 21)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate Bicep RPC pipe name: %w", err)
	}
	return platformPipeName("bicep-" + hex.EncodeToString(randomBytes) + "-sock"), nil
}
