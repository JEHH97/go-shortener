package redisstore

import (
	"testing"
)

func TestOptionsForRedisURLs(t *testing.T) {
	tests := []struct {
		name       string
		connection string
		wantAddr   string
		wantDB     int
		wantTLS    bool
	}{
		{name: "redis URL", connection: "redis://:url-secret@cache.example:6379/2", wantAddr: "cache.example:6379", wantDB: 2},
		{name: "rediss URL", connection: "rediss://:url-secret@cache.example:6380/3", wantAddr: "cache.example:6380", wantDB: 3, wantTLS: true},
		{name: "redis URL without path uses DB 0", connection: "redis://:url-secret@cache.example:6379", wantAddr: "cache.example:6379", wantDB: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options, err := optionsFor(test.connection, "legacy.example:6379", "legacy-secret")
			if err != nil {
				t.Fatal(err)
			}
			if options.Addr != test.wantAddr {
				t.Errorf("Addr = %q, want %q", options.Addr, test.wantAddr)
			}
			if options.DB != test.wantDB {
				t.Errorf("DB = %d, want %d", options.DB, test.wantDB)
			}
			if options.Password != "url-secret" {
				t.Errorf("Password did not come from URL")
			}
			if (options.TLSConfig != nil) != test.wantTLS {
				t.Errorf("TLSConfig present = %t, want %t", options.TLSConfig != nil, test.wantTLS)
			}
		})
	}
}

func TestOptionsForLegacyRedisSettings(t *testing.T) {
	options, err := optionsFor("", "legacy.example:6379", "legacy-secret")
	if err != nil {
		t.Fatal(err)
	}
	if options.Addr != "legacy.example:6379" || options.Password != "legacy-secret" || options.TLSConfig != nil {
		t.Fatalf("legacy options = %+v", options)
	}
}

func TestOptionsForRejectsInvalidRedisURL(t *testing.T) {
	for _, connection := range []string{"http://cache.example:6379", "redis://%zz"} {
		t.Run(connection, func(t *testing.T) {
			if _, err := optionsFor(connection, "legacy.example:6379", "legacy-secret"); err == nil {
				t.Fatalf("optionsFor(%q) unexpectedly succeeded", connection)
			}
		})
	}
}
