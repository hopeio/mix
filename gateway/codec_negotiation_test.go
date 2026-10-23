package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpx "github.com/hopeio/gox/net/http"
	"github.com/hopeio/mix"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestWantsJSON(t *testing.T) {
	cases := []struct {
		accept string
		want   bool
	}{
		{"", false},
		{"*/*", false},
		{"application/protobuf", false},
		{"application/protobuf, application/json", false},
		{"application/json", true},
		{"application/json, text/plain, */*", true},
	}
	for _, c := range cases {
		r, _ := http.NewRequest(http.MethodGet, "/", nil)
		if c.accept != "" {
			r.Header.Set("Accept", c.accept)
		}
		if got := wantsJSON(r); got != c.want {
			t.Errorf("Accept=%q wantsJSON=%v want %v", c.accept, got, c.want)
		}
	}
}

func TestMarshalProtobufDefault(t *testing.T) {
	msg := wrapperspb.String("ok")
	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Accept", "application/protobuf")
	data, ct, err := marshalForRequest(r, msg)
	if err != nil {
		t.Fatal(err)
	}
	if ct != httpx.ContentTypeProtobuf {
		t.Fatalf("content-type %q", ct)
	}
	var got wrapperspb.StringValue
	if err := proto.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Value != "ok" {
		t.Fatalf("value %q", got.Value)
	}
}

func TestMarshalJSONFallback(t *testing.T) {
	msg := wrapperspb.String("ok")
	r, _ := http.NewRequest(http.MethodPost, "/", nil)
	r.Header.Set("Accept", "application/json")
	data, ct, err := marshalForRequest(r, msg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ct, "json") {
		t.Fatalf("content-type %q", ct)
	}
	if !strings.Contains(string(data), `"data"`) {
		t.Fatalf("json envelope missing: %s", data)
	}
}

func TestMarshalErrRespProtobuf(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/protobuf")
	data, ct, err := marshalForRequest(r, mix.NewErrResp(1001, "auth.err.notActivated", map[string]string{"type": "Apple"}))
	if err != nil {
		t.Fatal(err)
	}
	if ct != httpx.ContentTypeProtobuf {
		t.Fatalf("content-type %q", ct)
	}
	var ei errdetails.ErrorInfo
	if err := proto.Unmarshal(data, &ei); err != nil {
		t.Fatal(err)
	}
	if ei.Reason != "auth.err.notActivated" || ei.Metadata["type"] != "Apple" {
		t.Fatalf("ErrorInfo=%v", &ei)
	}
}

func TestMarshalErrRespJSON(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/json")
	data, ct, err := marshalForRequest(r, mix.NewErrResp(1001, "auth.err.thirdLogin", map[string]string{"type": "Apple"}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ct, "json") {
		t.Fatalf("content-type %q", ct)
	}
	var got mix.ErrResp
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != 1001 || got.Msg != "auth.err.thirdLogin" || got.Data["type"] != "Apple" {
		t.Fatalf("json ErrResp=%+v body=%s", got, data)
	}
}

func TestHandleErrorProtobufHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	r, _ := http.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/protobuf")
	HandleError(rec, r, mix.NewErrResp(3, "auth.err.thirdLogin", map[string]string{"type": "Apple"}))
	if rec.Header().Get(mix.HeaderErrorCode) != "3" {
		t.Fatalf("Error-Code=%q", rec.Header().Get(mix.HeaderErrorCode))
	}
	var ei errdetails.ErrorInfo
	if err := proto.Unmarshal(rec.Body.Bytes(), &ei); err != nil {
		t.Fatal(err)
	}
	if ei.Reason != "auth.err.thirdLogin" || ei.Metadata["type"] != "Apple" {
		t.Fatalf("ErrorInfo=%v", &ei)
	}
}
