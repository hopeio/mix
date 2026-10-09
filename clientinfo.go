/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

// Client request context extraction: IP and User-Agent from incoming
// gRPC metadata (grpc-gateway forwards HTTP headers into metadata keys).
package mix

import (
	"context"
	"net"
	"strings"

	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// ClientIP returns the client's real IP: prefers the reverse-proxy headers
// injected by grpc-gateway, falls back to the gRPC peer TCP address. Only the
// first IP is taken so a forged chain is not passed through wholesale.
//
// Shared by callers that need traceability: content publishing, admin audit.
func ClientIP(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// grpc-gateway lowercases HTTP headers into metadata keys.
		if v := firstIP(md.Get("x-forwarded-for")); v != "" {
			return v
		}
		if v := firstIP(md.Get("x-real-ip")); v != "" {
			return v
		}
	}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		if host, _, err := net.SplitHostPort(p.Addr.String()); err == nil {
			return host
		}
		return p.Addr.String()
	}
	return ""
}

// ClientUA returns the client User-Agent (injected by grpc-gateway).
func ClientUA(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		for _, v := range md.Get("user-agent") {
			v = strings.TrimSpace(v)
			if v != "" {
				return v
			}
		}
	}
	return ""
}

func firstIP(v []string) string {
	for _, s := range v {
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
