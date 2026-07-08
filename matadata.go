package mix

import (
	"context"
	"net/http"
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
	AccessLogFields       []zap.Field
	Bagage                baggage.Baggage // can not edit
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

func (m *Metadata) Get(key any) any {
	m.RLock()
	defer m.RUnlock()
	if m.DataM == nil {
		return nil
	}
	return m.DataM[key]
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
