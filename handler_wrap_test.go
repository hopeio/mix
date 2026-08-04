package mix

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpx "github.com/hopeio/gox/net/http"
)

type wrapEchoReq struct {
	Name string `json:"name"`
}

type wrapEchoResp struct {
	Echo string `json:"echo"`
}

func init() {
	RegisterErrCodeHttpStatus(InvalidArgument, http.StatusBadRequest)
	RegisterErrCodeHttpStatus(NotFound, http.StatusNotFound)
}

func parseWrappedErrResp(t *testing.T, body []byte) ErrResp {
	t.Helper()
	var wrap struct {
		Data ErrResp `json:"data"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		t.Fatalf("unmarshal body=%q: %v", string(body), err)
	}
	return wrap.Data
}

func TestHandlerWrap_SuccessJSON(t *testing.T) {
	h := HandlerWrap(func(_ ReqResp, req *wrapEchoReq) (*wrapEchoResp, *ErrResp) {
		return &wrapEchoResp{Echo: "hi " + req.Name}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":"mix"}`))
	req.Header.Set(httpx.HeaderContentType, httpx.ContentTypeJson)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get(httpx.HeaderContentType), "json") {
		t.Fatalf("content-type=%q", rec.Header().Get(httpx.HeaderContentType))
	}
	var body struct {
		Data wrapEchoResp `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Echo != "hi mix" {
		t.Fatalf("data=%+v", body.Data)
	}
}

func TestHandlerWrap_BindFailure400(t *testing.T) {
	h := HandlerWrap(func(_ ReqResp, req *wrapEchoReq) (*wrapEchoResp, *ErrResp) {
		return &wrapEchoResp{Echo: req.Name}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{bad`))
	req.Header.Set(httpx.HeaderContentType, httpx.ContentTypeJson)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
	var er ErrResp
	er = parseWrappedErrResp(t, rec.Body.Bytes())
	if er.Code != InvalidArgument {
		t.Fatalf("code=%d msg=%q", er.Code, er.Msg)
	}
}

func TestHandlerWrap_ServiceErrResp(t *testing.T) {
	h := HandlerWrap(func(_ ReqResp, _ *wrapEchoReq) (*wrapEchoResp, *ErrResp) {
		return nil, NotFound.Msg("gone")
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":"x"}`))
	req.Header.Set(httpx.HeaderContentType, httpx.ContentTypeJson)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
	er := parseWrappedErrResp(t, rec.Body.Bytes())
	if er.Code != NotFound || er.Msg != "gone" {
		t.Fatalf("err resp=%+v", er)
	}
}

func TestHandlerWrapCommon_SuccessJSON(t *testing.T) {
	h := HandlerWrapCommon(func(_ context.Context, req *wrapEchoReq) (*wrapEchoResp, error) {
		return &wrapEchoResp{Echo: req.Name}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":"common"}`))
	req.Header.Set(httpx.HeaderContentType, httpx.ContentTypeJson)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var body struct {
		Data wrapEchoResp `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Echo != "common" {
		t.Fatalf("data=%+v", body.Data)
	}
}

func TestHandlerWrapCommon_BindFailure400(t *testing.T) {
	h := HandlerWrapCommon(func(_ context.Context, req *wrapEchoReq) (*wrapEchoResp, error) {
		return &wrapEchoResp{Echo: req.Name}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`not-json`))
	req.Header.Set(httpx.HeaderContentType, httpx.ContentTypeJson)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandlerWrapCommon_ServiceError(t *testing.T) {
	h := HandlerWrapCommon(func(_ context.Context, _ *wrapEchoReq) (*wrapEchoResp, error) {
		return nil, PermissionDenied.Msg("denied")
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":"x"}`))
	req.Header.Set(httpx.HeaderContentType, httpx.ContentTypeJson)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	er := parseWrappedErrResp(t, rec.Body.Bytes())
	if er.Code != PermissionDenied || er.Msg != "denied" {
		t.Fatalf("err resp=%+v", er)
	}
}

func TestWrapContext_UnWrapContext(t *testing.T) {
	const marker = "wrap-marker"
	h := HandlerWrapCommon(func(ctx context.Context, req *wrapEchoReq) (*wrapEchoResp, error) {
		v := UnWrapContext(ctx)
		rr, ok := v.(ReqResp)
		if !ok {
			t.Fatalf("UnWrapContext type=%T", v)
		}
		if rr.Request == nil || rr.ResponseWriter == nil {
			t.Fatal("ReqResp missing request/response")
		}
		rr.ResponseWriter.Header().Set("X-Marker", marker)
		return &wrapEchoResp{Echo: req.Name}, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":"ctx"}`))
	req.Header.Set(httpx.HeaderContentType, httpx.ContentTypeJson)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Header().Get("X-Marker") != marker {
		t.Fatalf("marker=%q", rec.Header().Get("X-Marker"))
	}
}

func TestWrapContext_RoundTrip(t *testing.T) {
	payload := struct{ N int }{42}
	ctx := WrapContext(payload)
	got := UnWrapContext(ctx)
	if got != payload {
		t.Fatalf("got=%v want=%v", got, payload)
	}
}

func TestWrapContext_UnWrapContextNil(t *testing.T) {
	if UnWrapContext(context.Background()) != nil {
		t.Fatal("expected nil for unrelated context")
	}
}

func TestHandlerWrapCommon_PlainError(t *testing.T) {
	h := HandlerWrapCommon(func(_ context.Context, _ *wrapEchoReq) (*wrapEchoResp, error) {
		return nil, errors.New("plain failure")
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":"x"}`))
	req.Header.Set(httpx.HeaderContentType, httpx.ContentTypeJson)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	er := parseWrappedErrResp(t, rec.Body.Bytes())
	if er.Code != Unknown || er.Msg != "plain failure" {
		t.Fatalf("err resp=%+v", er)
	}
}

func TestHandlerWrap_ResponderBranch(t *testing.T) {
	h := HandlerWrap(func(_ ReqResp, req *wrapEchoReq) (*CommonAnyResp, *ErrResp) {
		return NewCommonAnyResp(Success, "", req.Name), nil
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"name":"resp"}`))
	req.Header.Set(httpx.HeaderContentType, httpx.ContentTypeJson)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "resp") {
		t.Fatalf("body=%q", rec.Body.String())
	}
}
