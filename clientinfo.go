/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

// Client request context extraction: IP and User-Agent, covering both
// serving paths — gRPC (grpc-gateway forwards HTTP headers into metadata
// keys) and plain net/http handlers (via Metadata.Request).
package mix

import (
	"context"
	"net"
	"strings"
)

// ClientIP returns the client's real IP. Proxy headers win over the
// connection address; only the first entry is taken so a forged
// X-Forwarded-For chain is not passed through wholesale.
func (m *Metadata) ClientIP() string {
	if m == nil {
		return ""
	}
	if m.Request != nil {
		// Plain HTTP serving path: headers are canonical, address is
		// RemoteAddr.
		if v := firstIP(m.Request.Header.Values("X-Forwarded-For")); v != "" {
			return v
		}
		if v := firstIP(m.Request.Header.Values("X-Real-Ip")); v != "" {
			return v
		}
		return hostOnly(m.Request.RemoteAddr)
	}
	if len(m.IncomingMD) != 0 {
		if v := firstIP(m.IncomingMD.Get("x-forwarded-for")); v != "" {
			return v
		}
		if v := firstIP(m.IncomingMD.Get("x-real-ip")); v != "" {
			return v
		}
	}
	return hostOnly(m.PeerAddr)
}

// ClientUA returns the client User-Agent on either serving path.
func (m *Metadata) ClientUA() string {
	if m == nil {
		return ""
	}
	if m.Request != nil {
		return strings.TrimSpace(m.Request.UserAgent())
	}
	if len(m.IncomingMD) != 0 {
		return firstNonEmpty(m.IncomingMD.Get("user-agent"))
	}
	return ""
}

// ClientIP is a context convenience wrapper around Metadata.ClientIP.
func ClientIP(ctx context.Context) string {
	return GetMetadata(ctx).ClientIP()
}

// ClientUA is a context convenience wrapper around Metadata.ClientUA.
func ClientUA(ctx context.Context) string {
	return GetMetadata(ctx).ClientUA()
}

func firstIP(values []string) string {
	for _, s := range values {
		s = strings.TrimSpace(s)
		if i := strings.IndexByte(s, ','); i > 0 {
			s = strings.TrimSpace(s[:i])
		}
		if s != "" {
			return s
		}
	}
	return ""
}

func firstNonEmpty(values []string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

func hostOnly(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}
