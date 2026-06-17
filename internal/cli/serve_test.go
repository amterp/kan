package cli

import (
	"net"
	"testing"
)

func TestIsPortAvailable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	if !isPortAvailable("127.0.0.1", port) {
		t.Errorf("expected port %d to be available after closing", port)
	}
}

func TestIsPortAvailable_InUse(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	if isPortAvailable("127.0.0.1", port) {
		t.Errorf("expected port %d to be unavailable while listener is open", port)
	}
}

func TestFindAvailablePort_SkipsInUsePort(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	got := findAvailablePort("127.0.0.1", port)
	if got == port {
		t.Errorf("findAvailablePort returned in-use port %d", port)
	}
	if !isPortAvailable("127.0.0.1", got) {
		t.Errorf("expected returned port %d to be available", got)
	}
}
