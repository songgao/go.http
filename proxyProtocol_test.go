package http

import (
	"errors"
	proxyProtocol "github.com/racker/go-proxy-protocol"

	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"testing"
	"time"
)

var (
	fixtureTCP4             = "PROXY TCP4 127.0.0.1 127.0.0.2 65533 65534\r\n"
	fixtureTCP6             = "PROXY TCP6 2001:4801:7817:72:d4d9:211d:ff10:1631 2001:4801:7817:72:d4d9:211d:ff10:1632 65533 65534\r\n"
	fixtureHTTPRequest      = "GET / HTTP/1.1\r\n\r\n"
	fixtureInvalidProxyLine = "There is no spoon."
)

func fixtureTCP6_is_expected(pl *proxyProtocol.ProxyLine) bool {
	if pl.Protocol != proxyProtocol.TCP6 {
		return false
	}
	ipSrc, _ := net.ResolveIPAddr("ip", "2001:4801:7817:72:d4d9:211d:ff10:1631")
	if pl.SrcAddr.String() != ipSrc.String() {
		return false
	}
	ipDst, _ := net.ResolveIPAddr("ip", "2001:4801:7817:72:d4d9:211d:ff10:1632")
	if pl.DstAddr.String() != ipDst.String() {
		return false
	}
	if pl.SrcPort != 65533 || pl.DstPort != 65534 {
		return false
	}
	return true
}

func fixtureTCP4_is_expected(pl *proxyProtocol.ProxyLine) bool {
	if pl.Protocol != proxyProtocol.TCP4 {
		return false
	}
	ipSrc, _ := net.ResolveIPAddr("ip", "127.0.0.1")
	ipDst, _ := net.ResolveIPAddr("ip", "127.0.0.2")
	if pl.SrcAddr.String() != ipSrc.String() {
		return false
	}
	if pl.DstAddr.String() != ipDst.String() {
		return false
	}
	if pl.SrcPort != 65533 || pl.DstPort != 65534 {
		return false
	}
	return true
}

func TestPPParseTCP4(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("/", func(rsp ResponseWriter, req *Request) {
		if req.ProxyLine == nil {
			t.Fatal("ProxyLine is nil")
		} else if !fixtureTCP4_is_expected(req.ProxyLine) {
			t.Fatalf("Wrong ProxyLine parsed: %+q\n", req.ProxyLine)
		} else {
			io.WriteString(rsp, "OK")
		}
	})

	server := Server{Addr: ":9999|P", Handler: mux}
	addr, err := server.parseAddr()
	if err != nil {
		t.Fatalf("Error parsing Addr: %v\n", err)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Error listening on %v: %v\n", addr, err)
	}
	go server.Serve(listener)

	err = makeTestRequest("localhost:9999", fixtureTCP4)
	if err != nil {
		t.Fatalf("Error in making test request with PROXY line: %v\n", err)
	}
	listener.Close()
}

func TestPPParseTCP6(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("/", func(rsp ResponseWriter, req *Request) {
		if req.ProxyLine == nil {
			t.Fatal("ProxyLine is nil")
		} else if !fixtureTCP6_is_expected(req.ProxyLine) {
			t.Fatalf("Wrong ProxyLine parsed: %+q\n", req.ProxyLine)
		} else {
			io.WriteString(rsp, "OK")
		}
	})

	server := Server{Addr: ":9999|P", Handler: mux}
	addr, err := server.parseAddr()
	if err != nil {
		t.Fatalf("Error parsing Addr: %v\n", err)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Error listening on %v: %v\n", addr, err)
	}
	go server.Serve(listener)

	err = makeTestRequest("localhost:9999", fixtureTCP6)
	if err != nil {
		t.Fatalf("Error in making test request with PROXY line: %v\n", err)
	}
	listener.Close()
}

func TestPPNoProxy(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("/", func(rsp ResponseWriter, req *Request) {
		if req.ProxyLine != nil {
			t.Fatal("ProxyLine is not nil")
		}
	})

	server := Server{Addr: ":9999", Handler: mux}
	addr, err := server.parseAddr()
	if err != nil {
		t.Fatalf("Error parsing Addr: %v\n", err)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Error listening on %v: %v\n", addr, err)
	}
	go server.Serve(listener)

	if makeTestRequest("localhost:9999", fixtureTCP4) == nil {
		t.Fatalf("PROXY line went through to a server that doesn't support proxy protocol.\n")
	}
	if makeTestRequest("localhost:9999", "") != nil {
		t.Fatalf("Normal request without PROXY line failed to a server that doesn't support proxy protocol.\n")
	}
	listener.Close()
}

func TestPPInvalidProxy(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("/", func(rsp ResponseWriter, req *Request) {
		if req.ProxyLine == nil {
			t.Fatal("ProxyLine is nil")
		} else if !fixtureTCP6_is_expected(req.ProxyLine) {
			t.Fatalf("Wrong ProxyLine parsed: %+q\n", req.ProxyLine)
		} else {
			io.WriteString(rsp, "OK")
		}
	})

	server := Server{Addr: ":9999|P", Handler: mux}
	addr, err := server.parseAddr()
	if err != nil {
		t.Fatalf("Error parsing Addr: %v\n", err)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Error listening on %v: %v\n", addr, err)
	}
	go server.Serve(listener)

	if makeTestRequest("localhost:9999", fixtureInvalidProxyLine) == nil {
		t.Fatalf("Invalid PROXY line went through.\n")
	}
	listener.Close()
}

func TestPPInterleave(t *testing.T) {
	mux := NewServeMux()
	mux.HandleFunc("/", func(rsp ResponseWriter, req *Request) {
		if req.ProxyLine == nil {
			t.Fatal("ProxyLine is nil")
		} else if !fixtureTCP6_is_expected(req.ProxyLine) {
			t.Fatalf("Wrong ProxyLine parsed: %+q\n", req.ProxyLine)
		} else {
			io.WriteString(rsp, "OK")
		}
	})

	server := Server{Addr: ":9999|P", Handler: mux}
	addr, err := server.parseAddr()
	if err != nil {
		t.Fatalf("Error parsing Addr: %v\n", err)
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("Error listening on %v: %v\n", addr, err)
	}
	go server.Serve(listener)

	err = makeTestRequest("localhost:9999", fixtureTCP6)
	if err != nil {
		t.Fatalf("Error in making test request with PROXY line: %v\n", err)
	}

	err = makeTestRequest("localhost:9999", "")
	if err == nil {
		t.Fatalf("Normal request went through to a proxy protocol enabled server.\n")
	}

	listener.Close()
}

func makeTestRequest(addr string, proxyLine string) error {
	time.Sleep(time.Millisecond * 500)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	_, err = io.WriteString(conn, proxyLine)
	if err != nil {
		return err
	}

	c := httputil.NewClientConn(conn, nil)
	req, _ := http.NewRequest("GET", "/", nil)
	err = c.Write(req)
	if err != nil {
		return err
	}
	resp, err := c.Read(req)
	if err != nil && err != httputil.ErrPersistEOF {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("Status is not OK")
	}
	return nil
}
