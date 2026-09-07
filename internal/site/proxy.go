// internal/site/proxy.go

package site

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
)

// proxySet is the set of peers whose X-Forwarded-Proto the site believes.
type proxySet struct{ prefixes []netip.Prefix }

// newProxySet accepts the two spellings trusted_proxies takes: address and CIDR.
func newProxySet(entries []string) (proxySet, error) {
	// gin parses the same field with its own rules, so an entry the two read
	// differently is refused here rather than believed by only one of them.
	set := proxySet{prefixes: make([]netip.Prefix, 0, len(entries))}
	for _, entry := range entries {
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if prefix.Addr().Is4In6() {
				return proxySet{}, mappedEntry(entry, prefix.Addr())
			}
			set.prefixes = append(set.prefixes, prefix.Masked())
			continue
		}

		addr, err := netip.ParseAddr(entry)
		if err != nil {
			return proxySet{}, fmt.Errorf("trusted_proxies: %q is neither an address nor a CIDR block", entry)
		}
		if addr.Is4In6() {
			return proxySet{}, mappedEntry(entry, addr)
		}
		set.prefixes = append(set.prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return set, nil
}

// mappedEntry names the plain spelling of an IPv4-mapped IPv6 entry.
func mappedEntry(entry string, addr netip.Addr) error {
	return fmt.Errorf("trusted_proxies: %q is an IPv4 address written as IPv6; write it as %s", entry, addr.Unmap())
}

// has reports whether ip is one of the peers this site believes.
func (s proxySet) has(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}

	addr = addr.Unmap()
	for _, prefix := range s.prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// baseURL is the public origin the printed commands point at.
func (s *Server) baseURL(c *gin.Context) string {
	// TLS terminates at the proxy, so the scheme can only come from the
	// forwarded header, and only from a peer in trusted_proxies.
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if s.trusted.has(c.RemoteIP()) {
		if forwarded := forwardedScheme(c.GetHeader("X-Forwarded-Proto")); forwarded != "" {
			scheme = forwarded
		}
	}
	return scheme + "://" + c.Request.Host
}

// forwardedScheme takes the first value of a comma-joined header, http or https.
func forwardedScheme(header string) string {
	value := strings.ToLower(strings.TrimSpace(strings.Split(header, ",")[0]))
	if value == "http" || value == "https" {
		return value
	}
	return ""
}
