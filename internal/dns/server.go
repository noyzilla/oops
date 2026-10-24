package dns

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// Server coordinates DNS query resolution and upstream forwarding
type Server struct {
	resolver       *Resolver
	upstreams      []string
	tld            string
	zoneForwarders map[string]string
	mu             sync.RWMutex
	client         *dns.Client
}

// ParseUpstreams parses comma-separated upstream DNS servers
func ParseUpstreams(raw string) []string {
	if raw == "" {
		raw = os.Getenv("OOPS_DNS_UPSTREAM")
		if raw == "" {
			raw = "1.1.1.1:53,8.8.8.8:53"
		}
	}

	parts := strings.Split(raw, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if !strings.Contains(trimmed, ":") {
			trimmed = trimmed + ":53"
		}
		result = append(result, trimmed)
	}

	if len(result) == 0 {
		return []string{"1.1.1.1:53", "8.8.8.8:53"}
	}
	return result
}

// NewServer creates a new DNS Server instance
func NewServer(resolver *Resolver, upstreams []string) *Server {
	if len(upstreams) == 0 {
		upstreams = ParseUpstreams("")
	}
	tld := os.Getenv("OOPS_DNS_TLD")
	if tld == "" {
		tld = "oops"
	}
	return &Server{
		resolver:       resolver,
		upstreams:      upstreams,
		tld:            tld,
		zoneForwarders: make(map[string]string),
		client:         &dns.Client{Timeout: 2 * time.Second},
	}
}

// SetTLD updates the authoritative TLD for this DNS instance
func (s *Server) SetTLD(tld string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tld = strings.TrimSpace(strings.ToLower(tld))
}

// RegisterZoneForwarder binds a domain zone suffix (e.g. "b9tpe") to a target DNS server (e.g. "10.240.0.2:53")
func (s *Server) RegisterZoneForwarder(zone, targetDNS string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleanZone := strings.TrimPrefix(strings.TrimSuffix(strings.ToLower(zone), "."), ".")
	s.zoneForwarders[cleanZone] = strings.TrimSpace(targetDNS)
}

// UnregisterZoneForwarder removes a zone forwarder binding
func (s *Server) UnregisterZoneForwarder(zone string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleanZone := strings.TrimPrefix(strings.TrimSuffix(strings.ToLower(zone), "."), ".")
	delete(s.zoneForwarders, cleanZone)
}

// GetZoneForwarders returns a snapshot of active zone forwarders
func (s *Server) GetZoneForwarders() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make(map[string]string)
	for k, v := range s.zoneForwarders {
		res[k] = v
	}
	return res
}

func (s *Server) isPrivateTLD(qName string) bool {
	s.mu.RLock()
	tld := s.tld
	s.mu.RUnlock()
	if tld == "" {
		tld = "oops"
	}

	if qName == "oops" || strings.HasSuffix(qName, ".oops") {
		return true
	}
	if qName == tld || strings.HasSuffix(qName, "."+tld) {
		return true
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	for zone := range s.zoneForwarders {
		if qName == zone || strings.HasSuffix(qName, "."+zone) {
			return true
		}
	}
	return false
}

func (s *Server) matchZoneForwarder(qName string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for zone, target := range s.zoneForwarders {
		if qName == zone || strings.HasSuffix(qName, "."+zone) {
			return target
		}
	}
	return ""
}

// ServeDNS handles incoming DNS queries
func (s *Server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	for _, q := range r.Question {
		qName := strings.TrimSuffix(strings.ToLower(q.Name), ".")

		if q.Qtype == dns.TypeA {
			if ip, ok := s.resolver.Resolve(q.Name); ok {
				rr := &dns.A{
					Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 5},
					A:   ip.To4(),
				}
				msg.Answer = append(msg.Answer, rr)
				_ = w.WriteMsg(msg)
				return
			}
		}

		// Check active remote zone forwarders
		if target := s.matchZoneForwarder(qName); target != "" {
			resp, _, err := s.client.Exchange(r, target)
			if err == nil && resp != nil {
				_ = w.WriteMsg(resp)
				return
			}
			// Remote target unreachable: fail-fast with NXDOMAIN (never leak to public upstream)
			msg.Rcode = dns.RcodeNameError
			_ = w.WriteMsg(msg)
			return
		}

		// Zero Public Leakage Guard: never leak private TLD queries to public upstreams
		if s.isPrivateTLD(qName) {
			msg.Rcode = dns.RcodeNameError
			_ = w.WriteMsg(msg)
			return
		}
	}

	// Unresolved public domain -> Forward to Upstream
	s.forwardUpstream(w, r)
}

func (s *Server) forwardUpstream(w dns.ResponseWriter, r *dns.Msg) {
	for _, upstream := range s.upstreams {
		resp, _, err := s.client.Exchange(r, upstream)
		if err == nil && resp != nil {
			_ = w.WriteMsg(resp)
			return
		}
	}

	// If all upstreams fail -> Return NXDOMAIN
	msg := new(dns.Msg)
	msg.SetRcode(r, dns.RcodeNameError)
	_ = w.WriteMsg(msg)
}

// StartDNSDaemon starts both UDP and TCP DNS servers in non-blocking goroutines
func StartDNSDaemon(ctx context.Context, listenAddr string, resolver *Resolver, upstreams []string) error {
	if listenAddr == "" {
		listenAddr = ":53"
	}

	srv := NewServer(resolver, upstreams)
	handler := dns.Handler(srv)

	udpServer := &dns.Server{Addr: listenAddr, Net: "udp", Handler: handler}
	tcpServer := &dns.Server{Addr: listenAddr, Net: "tcp", Handler: handler}

	go func() {
		log.Printf("[DNS] Starting UDP DNS daemon on %s (Upstreams: %v)", listenAddr, srv.upstreams)
		if err := udpServer.ListenAndServe(); err != nil {
			log.Printf("[DNS] Warning: UDP DNS server stopped: %v", err)
		}
	}()

	go func() {
		log.Printf("[DNS] Starting TCP DNS daemon on %s", listenAddr)
		if err := tcpServer.ListenAndServe(); err != nil {
			log.Printf("[DNS] Warning: TCP DNS server stopped: %v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		_ = udpServer.Shutdown()
		_ = tcpServer.Shutdown()
	}()

	return nil
}
