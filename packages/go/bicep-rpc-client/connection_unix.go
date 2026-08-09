//go:build !windows

package biceprpcclient

import (
	"net"
	"os"
	"path/filepath"
)

func platformPipeName(name string) string {
	return filepath.Join(os.TempDir(), name+".sock")
}

func listenPipe(pipeName string) (net.Listener, func(), error) {
	listener, err := net.Listen("unix", pipeName)
	return listener, func() { _ = os.Remove(pipeName) }, err
}
