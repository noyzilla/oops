package dns

import (
	"net"
	"testing"

	"github.com/miekg/dns"
)

type mockResponseWriter struct {
	msg *dns.Msg
}

func (m *mockResponseWriter) LocalAddr() net.Addr  { return &net.UDPAddr{} }
func (m *mockResponseWriter) RemoteAddr() net.Addr { return &net.UDPAddr{} }
func (m *mockResponseWriter) WriteMsg(msg *dns.Msg) error {
	m.msg = msg
	return nil
}
func (m *mockResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (m *mockResponseWriter) Close() error                { return nil }
func (m *mockResponseWriter) TsigStatus() error           { return nil }
func (m *mockResponseWriter) TsigTimersOnly(bool)         {}
func (m *mockResponseWriter) Hijack()                     {}

func TestServer_TLDAndZoneForwarders(t *testing.T) {
	resolver := NewResolver()
	srv := NewServer(resolver, []string{"1.1.1.1:53"})

	srv.SetTLD("b9tpe")
	if srv.tld != "b9tpe" {
		t.Fatalf("expected tld b9tpe, got %s", srv.tld)
	}

	srv.RegisterZoneForwarder("anthole", "10.240.0.2:53")
	srv.RegisterZoneForwarder(".tunnel.oops.", "10.240.0.3:53")

	fwd := srv.GetZoneForwarders()
	if fwd["anthole"] != "10.240.0.2:53" {
		t.Fatalf("expected anthole forwarder, got %s", fwd["anthole"])
	}
	if fwd["tunnel.oops"] != "10.240.0.3:53" {
		t.Fatalf("expected tunnel.oops forwarder, got %s", fwd["tunnel.oops"])
	}

	srv.UnregisterZoneForwarder("anthole")
	fwd = srv.GetZoneForwarders()
	if _, ok := fwd["anthole"]; ok {
		t.Fatalf("expected anthole to be unregistered")
	}
}

func TestServer_ZeroPublicLeakageGuard(t *testing.T) {
	resolver := NewResolver()
	srv := NewServer(resolver, []string{"1.1.1.1:53"})
	srv.SetTLD("b9tpe")

	// Queries ending in .oops or .b9tpe should fail-fast with NXDOMAIN when unmapped
	testCases := []string{
		"unknown.oops.",
		"sub.unknown.oops.",
		"mysql.b9tpe.",
		"grafana.b9tpe.",
	}

	for _, domain := range testCases {
		req := new(dns.Msg)
		req.SetQuestion(domain, dns.TypeA)

		w := &mockResponseWriter{}
		srv.ServeDNS(w, req)

		if w.msg == nil {
			t.Fatalf("expected response message for %s, got nil", domain)
		}
		if w.msg.Rcode != dns.RcodeNameError {
			t.Fatalf("expected NXDOMAIN (RcodeNameError) for %s, got %d", domain, w.msg.Rcode)
		}
	}
}

func TestServer_ResolveLocalRecord(t *testing.T) {
	resolver := NewResolver()
	resolver.Register("c1", "mysql.b9tpe", net.ParseIP("10.240.0.5"))
	resolver.Register("c2", "*.wild.b9tpe", net.ParseIP("10.240.0.6"))

	srv := NewServer(resolver, []string{"1.1.1.1:53"})
	srv.SetTLD("b9tpe")

	// Exact match
	req := new(dns.Msg)
	req.SetQuestion("mysql.b9tpe.", dns.TypeA)

	w := &mockResponseWriter{}
	srv.ServeDNS(w, req)

	if w.msg == nil {
		t.Fatalf("expected response message, got nil")
	}
	if len(w.msg.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(w.msg.Answer))
	}

	aRecord, ok := w.msg.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected *dns.A record, got %T", w.msg.Answer[0])
	}
	if !aRecord.A.Equal(net.ParseIP("10.240.0.5")) {
		t.Fatalf("expected 10.240.0.5, got %s", aRecord.A.String())
	}

	// Wildcard match
	reqWild := new(dns.Msg)
	reqWild.SetQuestion("api.wild.b9tpe.", dns.TypeA)

	wWild := &mockResponseWriter{}
	srv.ServeDNS(wWild, reqWild)
	if len(wWild.msg.Answer) != 1 {
		t.Fatalf("expected 1 wildcard answer, got %d", len(wWild.msg.Answer))
	}
	aWild := wWild.msg.Answer[0].(*dns.A)
	if !aWild.A.Equal(net.ParseIP("10.240.0.6")) {
		t.Fatalf("expected 10.240.0.6 for wildcard, got %s", aWild.A.String())
	}
}
