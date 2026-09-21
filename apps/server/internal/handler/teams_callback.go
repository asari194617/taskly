package handler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// Callback destinations must be configured explicitly. An integration secret
// authenticates the caller, but must not grant access to arbitrary destinations.
func validateTeamsCallbackURL(raw, allowedOrigins string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return errors.New("callback must be an HTTPS URL without credentials or fragment")
	}
	if net.ParseIP(u.Hostname()) != nil {
		return errors.New("IP-literal callbacks are not allowed")
	}
	for _, entry := range strings.Split(allowedOrigins, ",") {
		origin, err := url.Parse(strings.TrimSpace(entry))
		if err != nil || origin.Scheme != "https" || origin.Hostname() == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
			continue
		}
		if strings.EqualFold(u.Host, origin.Host) {
			return nil
		}
	}
	return errors.New("callback origin is not allowed")
}

var nonPublicTeamsNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001::/32"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
}

func isPublicTeamsIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return false
	}
	for _, network := range nonPublicTeamsNetworks {
		if network.Contains(addr) {
			return false
		}
	}
	return true
}

type teamsIPResolver func(context.Context, string) ([]net.IPAddr, error)

func newTeamsCallbackClient(resolve teamsIPResolver) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		// Never let HTTP_PROXY route callbacks around destination validation.
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := resolve(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("callback DNS lookup failed: %w", err)
			}
			if len(ips) == 0 {
				return nil, errors.New("callback DNS lookup returned no addresses")
			}
			for _, ip := range ips {
				if !isPublicTeamsIP(ip.IP) {
					return nil, errors.New("callback resolved to a non-public address")
				}
			}
			// Connect to the validated IP, not the hostname again: a second DNS
			// lookup here would reintroduce a DNS-rebinding window.
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
	}
	return &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
