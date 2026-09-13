// internal/site/proxy.go

package site

import (
	"fmt"
	"net/netip"
	"regexp"
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
			// ParsePrefix rejects a zone itself, so only the address branch checks for one.
			if prefix.Addr().Is4In6() {
				return proxySet{}, mappedEntry(entry, prefix, true)
			}
			set.prefixes = append(set.prefixes, prefix.Masked())
			continue
		}

		addr, err := netip.ParseAddr(entry)
		if err != nil {
			return proxySet{}, fmt.Errorf("trusted_proxies: %q is neither an address nor a CIDR block", entry)
		}
		if addr.Zone() != "" {
			return proxySet{}, zonedEntry(entry)
		}
		if addr.Is4In6() {
			return proxySet{}, mappedEntry(entry, netip.PrefixFrom(addr, addr.BitLen()), false)
		}
		set.prefixes = append(set.prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return set, nil
}

// mappedEntry refuses an IPv4-mapped IPv6 entry and names the plain spelling to use.
func mappedEntry(entry string, prefix netip.Prefix, wasPrefix bool) error {
	plain := prefix.Addr().Unmap()
	switch {
	case !wasPrefix:
		return fmt.Errorf("trusted_proxies: %q is an IPv4 address written as IPv6; write it as %s", entry, plain)
	case prefix.Bits() >= 96:
		return fmt.Errorf("trusted_proxies: %q is an IPv4 block written as IPv6; write it as %s/%d", entry, plain, prefix.Bits()-96)
	default:
		return fmt.Errorf("trusted_proxies: %q spans the IPv4-mapped range; write it as a plain IPv4 address or CIDR", entry)
	}
}

// zonedEntry refuses an address carrying an IPv6 zone id, which gin cannot parse.
func zonedEntry(entry string) error {
	return fmt.Errorf("trusted_proxies: %q carries a zone id; write the address without it", entry)
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
func (s *Server) baseURL(c *gin.Context) (string, bool) {
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

	host := c.Request.Host
	if !hostName.MatchString(host) {
		return "", false
	}
	return scheme + "://" + host, true
}

// hostName is the host a served command may quote: net/http lets $ ( ) ' ; through, and root runs the script.
var hostName = regexp.MustCompile(`^([A-Za-z0-9._-]+|\[[0-9A-Fa-f:.]+])(:[0-9]{1,5})?$`)

// forwardedScheme takes the first value of a comma-joined header, http or https.
func forwardedScheme(header string) string {
	value := strings.ToLower(strings.TrimSpace(strings.Split(header, ",")[0]))
	if value == "http" || value == "https" {
		return value
	}
	return ""
}
