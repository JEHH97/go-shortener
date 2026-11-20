package domain

import (
	"context"
	"errors"
	"net"
	"testing"
)

type fakeResolver struct {
	ips []net.IPAddr
	err error
}

func (f fakeResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return f.ips, f.err
}

func TestValidateAlias(t *testing.T) {
	for _, value := range []string{"", "abc", "x_42-z", "01234567890123456789012345678901"} {
		if err := ValidateAlias(value); err != nil {
			t.Errorf("ValidateAlias(%q): %v", value, err)
		}
	}
	for _, value := range []string{"ab", "has space", "éclair", "a/b", "012345678901234567890123456789012"} {
		if err := ValidateAlias(value); !errors.Is(err, ErrInvalidAlias) {
			t.Errorf("ValidateAlias(%q) error = %v", value, err)
		}
	}
}

func TestValidateURL(t *testing.T) {
	ctx := context.Background()
	public := fakeResolver{ips: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}
	for _, raw := range []string{"https://example.com/path", "http://93.184.216.34"} {
		if err := ValidateURL(ctx, raw, public); err != nil {
			t.Errorf("expected valid URL %q: %v", raw, err)
		}
	}
	for _, raw := range []string{"", "ftp://example.com", "//example.com", "https://user:pass@example.com", "http://localhost", "http://node.localhost", "http://127.0.0.1", "http://10.2.3.4", "http://169.254.2.1", "http://[::1]", "http://[fd00::1]", "http://[::ffff:127.0.0.1]", "https://" + string(make([]byte, MaxURLLength+1))} {
		if err := ValidateURL(ctx, raw, public); !errors.Is(err, ErrInvalidURL) {
			t.Errorf("expected invalid URL %q, got %v", raw, err)
		}
	}
	if err := ValidateURL(ctx, "https://example.com", fakeResolver{ips: []net.IPAddr{{IP: net.ParseIP("192.168.1.1")}}}); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("private DNS answer accepted: %v", err)
	}
	if err := ValidateURL(ctx, "https://missing.example", fakeResolver{err: errors.New("dns down")}); !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("DNS failure accepted: %v", err)
	}
}
