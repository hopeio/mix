package mix

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hopeio/gox/log"
	"go.opentelemetry.io/otel/baggage"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type RequestType int

const (
	RequestTypeHttp RequestType = iota
	RequestTypeGrpc
)

// Metadata is the per-request snapshot carried through the context, spanning
// the HTTP and gRPC serving paths.
//
// Concurrency contract: fields are written once during request assembly
// (inside mix handlers, before the request is served) and are read-only
// afterwards. Data/DataM are the exception — they carry values across
// goroutines during a request and MUST go through the lock-protected
// Set/Get/SetData/GetData/DataAs methods. AccessLogFields is read by the
// access logger when the request completes; append to it before the handler
// returns. Exported fields are deliberate: like http.Request, this is a data
// record, not an invariant-bearing type, and getters over reference-typed
// fields (Request, IncomingMD, slices, maps) would provide no real
// encapsulation while forcing defensive copies on hot paths.
type Metadata struct {
	sync.RWMutex
	Logger                *log.Logger
	Data                  any
	DataM                 map[any]any
	TraceId               string
	RequestType           RequestType
	Token                 string
	AuthRaw               []byte
	AuthID                string
	Request               *http.Request
	ResponseWriter        http.ResponseWriter
	RequestAt             time.Time
	IncomingMD            metadata.MD
	ServerTransportStream grpc.ServerTransportStream
	// PeerAddr is the remote connection address captured during gRPC
	// request assembly; fallback when no proxy headers are present.
	PeerAddr        string
	AccessLogFields []zap.Field
	Baggage         baggage.Baggage
}

func (m *Metadata) Get(key any) any {
	m.RLock()
	defer m.RUnlock()
	if m.DataM == nil {
		return nil
	}
	return m.DataM[key]
}

func (m *Metadata) Set(key, value any) {
	m.Lock()
	defer m.Unlock()
	if m.DataM == nil {
		m.DataM = make(map[any]any)
	}
	m.DataM[key] = value
}

func (m *Metadata) Del(key any) {
	m.Lock()
	defer m.Unlock()
	if m.DataM == nil {
		return
	}
	delete(m.DataM, key)
}

func (m *Metadata) GetData() any {
	m.RLock()
	defer m.RUnlock()
	return m.Data
}

func (m *Metadata) SetData(value any) {
	m.Lock()
	defer m.Unlock()
	m.Data = value
}

func (m *Metadata) DataAs[T any]() (T, bool) {
	m.RLock()
	defer m.RUnlock()
	v, ok := m.Data.(T)
	return v, ok
}

type metadataKey struct{}

var MetadataKey = metadataKey{}

func WithMetadata(ctx context.Context, metadata *Metadata) context.Context {
	return context.WithValue(ctx, MetadataKey, metadata)
}

func GetMetadata(ctx context.Context) *Metadata {
	metadata, ok := ctx.Value(MetadataKey).(*Metadata)
	if !ok {
		return nil
	}
	return metadata
}

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
