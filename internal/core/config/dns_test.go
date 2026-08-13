package config

import "testing"

func TestCloudflareZonesAreRequired(t *testing.T) {
	t.Parallel()

	if err := validateControllerDNS(&ControllerDNSConfig{Cloudflare: &CloudflareDNSConfig{}}); err == nil {
		t.Fatal("expected controller DNS Cloudflare zones to be required")
	}
	if err := validateControllerCloudflareTunnel(&ControllerCloudflareTunnelConfig{
		AccountID: "account",
		APIToken:  "token",
		Tunnels: map[string]ControllerCloudflareTunnel{
			"edge": {TunnelID: "tunnel"},
		},
	}, nil); err == nil {
		t.Fatal("expected Cloudflare Tunnel zones to be required")
	}
}

func TestResolveDNSProviderUsesMostSpecificZone(t *testing.T) {
	controller := &ControllerConfig{DNS: &ControllerDNSConfig{
		Cloudflare: &CloudflareDNSConfig{Zones: []string{"example.com"}},
		Route53:    &Route53DNSConfig{Zones: []string{"sub.example.com"}},
	}}

	provider, err := controller.ResolveDNSProvider("app.sub.example.com")
	if err != nil {
		t.Fatalf("resolve DNS provider: %v", err)
	}
	if provider != "route53" {
		t.Fatalf("expected route53, got %q", provider)
	}
}

func TestResolveDNSProviderRejectsEqualSpecificity(t *testing.T) {
	controller := &ControllerConfig{DNS: &ControllerDNSConfig{
		Cloudflare: &CloudflareDNSConfig{Zones: []string{"example.com"}},
		Route53:    &Route53DNSConfig{Zones: []string{"example.com."}},
	}}

	if _, err := controller.ResolveDNSProvider("app.example.com"); err == nil {
		t.Fatal("expected equal-specificity provider error")
	}
}
