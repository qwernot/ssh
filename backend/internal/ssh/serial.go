package ssh

import (
	"fmt"
	"io"
	"sync"

	"go.bug.st/serial"
)

// SerialClient wraps a serial port connection
type SerialClient struct {
	port   serial.Port
	mu     sync.Mutex
	closed bool
}

// ConnectSerial opens a serial port
func ConnectSerial(portName string, baudRate int) (*SerialClient, error) {
	mode := &serial.Mode{
		BaudRate: baudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, fmt.Errorf("serial open %s: %w", portName, err)
	}

	return &SerialClient{port: port}, nil
}

func (s *SerialClient) Read(p []byte) (int, error) {
	if s.closed {
		return 0, io.EOF
	}
	return s.port.Read(p)
}

func (s *SerialClient) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, io.EOF
	}
	return s.port.Write(p)
}

func (s *SerialClient) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.port.Close()
}
