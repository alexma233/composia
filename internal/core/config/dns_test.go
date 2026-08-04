package config

import "testing"

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
