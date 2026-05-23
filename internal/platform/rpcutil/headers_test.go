package rpcutil

import "testing"

func TestNormalizeStaticHeaders(t *testing.T) {
	t.Parallel()

	headers, err := NormalizeStaticHeaders(map[string]string{" cf-access-client-id ": " id "})
	if err != nil {
		t.Fatalf("NormalizeStaticHeaders returned error: %v", err)
	}
	if got := headers["Cf-Access-Client-Id"]; got != "id" {
		t.Fatalf("header = %q", got)
	}
}

func TestNormalizeStaticHeadersRejectsReservedHeader(t *testing.T) {
	t.Parallel()

	_, err := NormalizeStaticHeaders(map[string]string{"Authorization": "Bearer token"})
	if err == nil {
		t.Fatalf("expected reserved header error")
	}
}
