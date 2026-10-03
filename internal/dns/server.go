package dns

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// Server coordinates DNS query resolution and upstream forwarding
type Server struct {
	resolver  *Resolver
	upstreams []string
	client    *dns.Client
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
	return &Server{
		resolver:  resolver,
		upstreams: upstreams,
		client:    &dns.Client{Timeout: 2 * time.Second},
	}
}

// ServeDNS handles incoming DNS queries
func (s *Server) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	for _, q := range r.Question {
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
	}

	// Unresolved query -> Forward to Upstream
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
