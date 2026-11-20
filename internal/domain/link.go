package domain

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const MaxURLLength = 2048

var (
	ErrInvalidURL   = errors.New("invalid destination URL")
	ErrInvalidAlias = errors.New("alias must be 3-32 letters, numbers, '-' or '_'")
	aliasPattern    = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)
)

type Resolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type Link struct {
	Code      string
	URL       string
	CreatedAt time.Time
	ExpiresAt *time.Time
}

func ValidateAlias(alias string) error {
	if alias != "" && !aliasPattern.MatchString(alias) {
		return ErrInvalidAlias
	}
	return nil
}

func ValidateURL(ctx context.Context, raw string, resolver Resolver) error {
	if raw == "" || len(raw) > MaxURLLength {
		return ErrInvalidURL
	}
	u, err := url.ParseRequestURI(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
		return ErrInvalidURL
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") {
		return ErrInvalidURL
	}
	if ip := net.ParseIP(host); ip != nil {
		if isBlockedIP(ip) {
			return ErrInvalidURL
		}
		return nil
	}
	if strings.Contains(host, ":") || strings.ContainsAny(host, " \t\r\n") {
		return ErrInvalidURL
	}
	addresses, err := resolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return fmt.Errorf("%w: hostname cannot be resolved", ErrInvalidURL)
	}
	for _, address := range addresses {
		if isBlockedIP(address.IP) {
			return ErrInvalidURL
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
	}
	return ip == nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}
