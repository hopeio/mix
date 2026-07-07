/*
 * Copyright 2024 hopeio. All rights reserved.
 * Licensed under the MIT License that can be found in the LICENSE file.
 * @Created by jyb
 */

package mix

import (
	"context"
	"net"
	"net/http"
	"net/http/httptrace"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/hopeio/gox/log"
	httpx "github.com/hopeio/gox/net/http"
	"github.com/quic-go/quic-go"
	"github.com/rs/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

func NewServer(options ...Option) *Server {
	s := &Server{}
	s.Init()
	for _, option := range options {
		option(s)
	}
	return s
}

func (s *Server) Run() {
	baseCtx := context.Background()
	// Handler SIGINT (CTRL+C) gracefully.
	sigCtx, stop := signal.NotifyContext(baseCtx, // kill -SIGINT XXXX 或 Ctrl+c
		syscall.SIGINT, // register that too, it should be ok
		// os.Kill等同于syscall.Kill
		syscall.SIGKILL, // register that too, it should be ok
		// kill -SIGTERM XXXX
		syscall.SIGTERM,
	)
	defer stop()

	grpcServer := s.grpcHandler()
	httpHandler := s.httpHandler()

	// cors
	if s.Cors.Enabled {
		httpHandler = cors.New(s.Cors.Options).Handler(httpHandler)
	}

	// Set up OpenTelemetry.
	if s.Otel.Enabled {
		http.DefaultClient = &http.Client{
			Transport: otelhttp.NewTransport(
				http.DefaultTransport,
				otelhttp.WithClientTrace(func(ctx context.Context) *httptrace.ClientTrace {
					return otelhttptrace.NewClientTrace(ctx)
				}),
			),
		}
		shutdownFunc, err := setupOTelSDK(sigCtx)
		if err != nil {
			log.Fatal(err)
		}
		if shutdownFunc != nil {
			defer shutdownFunc(sigCtx)
		}
		s.tracer = otel.Tracer(ScopeName)
		s.meter = otel.Meter(ScopeName)
	}

	onePort := s.Grpc.Addr == "" || s.Grpc.Addr == s.Addr
	handler := httpx.UseMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		md := Metadata{
			Request:        r,
			ResponseWriter: w,
			RequestAt:      time.Now(),
		}
		r = r.WithContext(WithMetadata(r.Context(), &md))
		if onePort && strings.HasPrefix(r.Header.Get(httpx.HeaderContentType), httpx.ContentTypeGrpc) {
			 if r.ProtoMajor == 2 && grpcServer != nil {
				md.RequestType = RequestTypeGrpc
				grpcServer.ServeHTTP(w, r)
			} else {
				http.NotFound(w, r)
			}
		} else {
			httpHandler.ServeHTTP(w, r)
		}
	}), s.Middlewares...)

	s.Server.Handler = handler
	
	if s.Server.BaseContext == nil {
		s.Server.BaseContext = func(_ net.Listener) context.Context {
			return sigCtx
		}
	}

	// 为了提供grpc服务,默认启用http2
	if s.Server.TLSConfig == nil{
		s.Server.Protocols = new(http.Protocols)
		s.Server.Protocols.SetHTTP1(true)
		s.Server.Protocols.SetUnencryptedHTTP2(true)
	}

	srvErr := make(chan error, 1)

	if s.Grpc.Addr != "" && s.Grpc.Addr != s.Addr {
		go func() {
			log.Infof("grpc listening: %s", s.Grpc.Addr)
			listener, err := net.Listen("tcp", s.Grpc.Addr)
			if err != nil {
				srvErr <- err
				return
			}
			srvErr <- grpcServer.Serve(listener)
		}()
	}

	if s.HTTP3.Enabled {
		s.HTTP3.Handler = handler
		if s.HTTP3.ConnContext == nil {
			s.HTTP3.ConnContext = func(ctx context.Context, c *quic.Conn) context.Context {
				return sigCtx
			}
		}
		go func() {
			log.Infof("http3 listening: %s", s.HTTP3.Addr)
			if s.HTTP3.CertFile != "" && s.HTTP3.KeyFile != "" {
				srvErr <- s.HTTP3.ListenAndServeTLS(s.CertFile, s.KeyFile)
			} else {
				srvErr <- s.HTTP3.ListenAndServe()
			}
		}()
	}
	go func() {
		log.Infof("listening: %s", s.Addr)
		if s.CertFile != "" && s.KeyFile != "" {
			srvErr <- s.ListenAndServeTLS(s.CertFile, s.KeyFile)
		} else {
			srvErr <- s.ListenAndServe()
		}
	}()

	go func() {
		log.Infof("internal listening: %s", s.InternalServer.Addr)
		s.InternalHandler()
		if s.InternalServer.BaseContext == nil {
			s.InternalServer.BaseContext = func(_ net.Listener) context.Context {
				return sigCtx
			}
			s.InternalServer.Handler = http.DefaultServeMux
		}
		srvErr <- s.InternalServer.ListenAndServe()
	}()

	// Wait for interruption.
	select {
	case err := <-srvErr:
		// Error when starting HTTP server.
		log.Fatalf("failed to serve: %v", err)
	case <-sigCtx.Done():
		// Wait for first CTRL+C.
		// Stop receiving signal notifications as soon as possible.
		stop()
		log.Debug("stop server")
	}

	//服务关闭
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	if err := s.Shutdown(sigCtx); err != nil {
		log.Error(err)
	}
}

func (s *Server) WithContext(ctx context.Context) *Server {
	s.BaseContext = ctx
	return s
}
