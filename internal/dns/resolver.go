package dns

import (
	"net"
	"strings"
	"sync"
)

// Resolver maintains an in-memory mapping of container hostnames to IP addresses
type Resolver struct {
	mu       sync.RWMutex
	exact    map[string]net.IP   // exact domain -> IP
	wildcard map[string]net.IP   // suffix -> IP (e.g. "web.oops" -> IP)
	byID     map[string][]string // containerID -> []domains
}

// NewResolver creates an initialized thread-safe Resolver
func NewResolver() *Resolver {
	return &Resolver{
		exact:    make(map[string]net.IP),
		wildcard: make(map[string]net.IP),
		byID:     make(map[string][]string),
	}
}

// Register adds or updates a container's hostname to IP mapping
func (r *Resolver) Register(containerID, rawHostname string, ip net.IP) {
	if ip == nil {
		return
	}
	trimmed := strings.TrimSpace(strings.ToLower(rawHostname))
	if trimmed == "" || trimmed == "localhost" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Clean trailing dots
	trimmed = strings.TrimSuffix(trimmed, ".")

	if strings.HasPrefix(trimmed, ".") {
		suffix := strings.TrimPrefix(trimmed, ".")
		if suffix != "" {
			r.wildcard[suffix] = ip
			r.byID[containerID] = append(r.byID[containerID], "."+suffix)
		}
	} else {
		r.exact[trimmed] = ip
		r.byID[containerID] = append(r.byID[containerID], trimmed)
	}
}

// Unregister removes all hostnames associated with a container ID
func (r *Resolver) Unregister(containerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	domains, exists := r.byID[containerID]
	if !exists {
		return
	}

	for _, d := range domains {
		if strings.HasPrefix(d, ".") {
			suffix := strings.TrimPrefix(d, ".")
			delete(r.wildcard, suffix)
		} else {
			delete(r.exact, d)
		}
	}
	delete(r.byID, containerID)
}

// SwapStatic atomically replaces all static records with the new set
func (r *Resolver) SwapStatic(newRecords map[string]net.IP) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 1. Unregister previous static entries
	if domains, exists := r.byID["static"]; exists {
		for _, d := range domains {
			if strings.HasPrefix(d, ".") {
				suffix := strings.TrimPrefix(d, ".")
				delete(r.wildcard, suffix)
			} else {
				delete(r.exact, d)
			}
		}
		delete(r.byID, "static")
	}

	// 2. Register new static entries
	for rawHostname, ip := range newRecords {
		if ip == nil {
			continue
		}
		trimmed := strings.TrimSpace(strings.ToLower(rawHostname))
		trimmed = strings.TrimSuffix(trimmed, ".")
		if trimmed == "" || trimmed == "localhost" {
			continue
		}

		if strings.HasPrefix(trimmed, ".") {
			suffix := strings.TrimPrefix(trimmed, ".")
			if suffix != "" {
				r.wildcard[suffix] = ip
				r.byID["static"] = append(r.byID["static"], "."+suffix)
			}
		} else {
			r.exact[trimmed] = ip
			r.byID["static"] = append(r.byID["static"], trimmed)
		}
	}
}

// Resolve returns the matched IPv4 for a query domain, if registered
func (r *Resolver) Resolve(queryDomain string) (net.IP, bool) {
	query := strings.TrimSpace(strings.ToLower(queryDomain))
	query = strings.TrimSuffix(query, ".")
	if query == "" {
		return nil, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Exact match
	if ip, ok := r.exact[query]; ok {
		return ip, true
	}

	// 2. Wildcard match (matches apex "web.oops" or subdomains "*.web.oops")
	for suffix, ip := range r.wildcard {
		if query == suffix || strings.HasSuffix(query, "."+suffix) {
			return ip, true
		}
	}

	return nil, false
}
