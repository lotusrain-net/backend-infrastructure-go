package requestmeta

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	"backend-infrastructure-go/internal/modules/audit"
)

type Config struct {
	TrustedProxies []netip.Prefix
}

func Extract(request *http.Request, config Config) audit.RequestMetadata {
	if request == nil {
		return audit.RequestMetadata{}
	}
	peer, ok := parseAddress(request.RemoteAddr)
	metadata := audit.RequestMetadata{UserAgent: request.UserAgent()}
	if !ok {
		return metadata
	}
	client := peer
	if trusted(peer, config.TrustedProxies) {
		if forwarded, valid := forwardedChain(request.Header.Get("X-Forwarded-For")); valid {
			for index := len(forwarded) - 1; index >= 0; index-- {
				client = forwarded[index]
				if !trusted(client, config.TrustedProxies) {
					break
				}
			}
		}
	}
	metadata.IPAddress = &client
	return metadata
}

func parseAddress(value string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		value = host
	}
	address, err := netip.ParseAddr(strings.TrimSpace(value))
	return address.Unmap(), err == nil
}

func forwardedChain(value string) ([]netip.Addr, bool) {
	if strings.TrimSpace(value) == "" {
		return nil, false
	}
	parts := strings.Split(value, ",")
	addresses := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		address, err := netip.ParseAddr(strings.TrimSpace(part))
		if err != nil {
			return nil, false
		}
		addresses = append(addresses, address.Unmap())
	}
	return addresses, true
}

func trusted(address netip.Addr, prefixes []netip.Prefix) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
