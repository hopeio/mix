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

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// ClientIP returns the client's real IP. Proxy headers win over the
// connection address; only the first entry is taken so a forged
// X-Forwarded-For chain is not passed through wholesale.
func ClientIP(ctx context.Context) string {
	if md := GetMetadata(ctx); md != nil {
		if md.Request != nil {
			// Plain HTTP serving path: headers are canonical, address is
			// RemoteAddr.
			if v := firstIP(md.Request.Header.Values("X-Forwarded-For")); v != "" {
				return v
			}
			if v := firstIP(md.Request.Header.Values("X-Real-Ip")); v != "" {
				return v
			}
			return hostOnly(md.Request.RemoteAddr)
		}
		if len(md.IncomingMD) != 0 {
			if v := firstIP(md.IncomingMD.Get("x-forwarded-for")); v != "" {
				return v
			}
			if v := firstIP(md.IncomingMD.Get("x-real-ip")); v != "" {
				return v
			}
		}
	} else {
		// No request snapshot in the context: fall back to raw gRPC state.
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if v := firstIP(md.Get("x-forwarded-for")); v != "" {
				return v
			}
			if v := firstIP(md.Get("x-real-ip")); v != "" {
				return v
			}
		}
	}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return hostOnly(p.Addr.String())
	}
	return ""
}

// ClientUA returns the client User-Agent on either serving path.
func ClientUA(ctx context.Context) string {
	if md := GetMetadata(ctx); md != nil {
		if md.Request != nil {
			return strings.TrimSpace(md.Request.UserAgent())
		}
		if len(md.IncomingMD) != 0 {
			return firstNonEmpty(md.IncomingMD.Get("user-agent"))
		}
	} else if md, ok := metadata.FromIncomingContext(ctx); ok {
		return firstNonEmpty(md.Get("user-agent"))
	}
	return ""
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
