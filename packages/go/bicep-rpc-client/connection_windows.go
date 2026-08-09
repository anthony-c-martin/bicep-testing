//go:build windows

package biceprpcclient

import (
	"net"

	"github.com/Microsoft/go-winio"
)

func platformPipeName(name string) string {
	return `\\.\pipe\` + name
}

func listenPipe(pipeName string) (net.Listener, func(), error) {
	listener, err := winio.ListenPipe(pipeName, &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;OW)",
		InputBufferSize:    64 * 1024,
		OutputBufferSize:   64 * 1024,
	})
	return listener, func() {}, err
}
