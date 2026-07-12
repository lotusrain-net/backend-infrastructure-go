package requestmeta_test

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"backend-infrastructure-go/internal/modules/audit"
	"backend-infrastructure-go/internal/modules/audit/requestmeta"
)

func TestMiddlewareInjectsTrustedRequestMetadataIntoContext(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.1:443"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	request.Header.Set("User-Agent", "test-client/1.0")
	var got audit.RequestMetadata
	handler := requestmeta.Middleware(requestmeta.Config{TrustedProxies: []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
	}})(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		got = audit.RequestMetadataFromContext(request.Context())
	}))

	handler.ServeHTTP(httptest.NewRecorder(), request)
	if got.IPAddress == nil || got.IPAddress.String() != "203.0.113.9" || got.UserAgent != "test-client/1.0" {
		t.Fatalf("request metadata = %#v", got)
	}
}

func TestExtractUsesForwardedClientFromTrustedProxyChain(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "10.0.0.1:443"
	request.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.2")
	request.Header.Set("User-Agent", "test-client/1.0")
	config := requestmeta.Config{TrustedProxies: []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
	}}

	got := requestmeta.Extract(request, config)
	if got.IPAddress == nil || got.IPAddress.String() != "203.0.113.9" || got.UserAgent != "test-client/1.0" {
		t.Fatalf("Extract() = %#v", got)
	}
}

func TestExtractIgnoresSpoofedForwardingHeaderFromUntrustedPeer(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "198.51.100.20:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")

	got := requestmeta.Extract(request, requestmeta.Config{TrustedProxies: []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
	}})
	if got.IPAddress == nil || got.IPAddress.String() != "198.51.100.20" {
		t.Fatalf("Extract() IP = %v", got.IPAddress)
	}
}

func TestExtractFallsBackToPeerWhenForwardedChainIsMalformed(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "10.0.0.1:443"
	request.Header.Set("X-Forwarded-For", "not-an-ip")

	got := requestmeta.Extract(request, requestmeta.Config{TrustedProxies: []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
	}})
	if got.IPAddress == nil || got.IPAddress.String() != "10.0.0.1" {
		t.Fatalf("Extract() IP = %v", got.IPAddress)
	}
}
