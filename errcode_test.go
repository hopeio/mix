package mix

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	httpx "github.com/hopeio/gox/net/http"
)

func TestErrCode_StringAndHelpers(t *testing.T) {
	if NotFound.String() != "NotFound" {
		t.Fatalf("String=%q", NotFound.String())
	}
	RegisterErrCode(1001, "Custom")
	if ErrCode(1001).String() != "Custom" {
		t.Fatalf("custom String=%q", ErrCode(1001).String())
	}
	er := NotFound.Msg("gone")
	if er.Code != NotFound || er.Msg != "gone" {
		t.Fatalf("Msg helper: %+v", er)
	}
	er2 := Internal.Wrap(errors.New("boom"))
	if er2.Code != Internal || er2.Msg != "boom" {
		t.Fatalf("Wrap: %+v", er2)
	}
}

func TestErrRespFrom(t *testing.T) {
	if ErrRespFrom(nil) != nil {
		t.Fatal("nil err -> nil")
	}
	orig := NewErrResp(NotFound, "x")
	if ErrRespFrom(orig) != orig {
		t.Fatal("want same *ErrResp")
	}
	via := ErrRespFrom(InvalidArgument)
	if via.Code != InvalidArgument {
		t.Fatalf("ErrCode path: %+v", via)
	}
	via2 := ErrRespFrom(errors.New("plain"))
	if via2.Code != Unknown || via2.Msg != "plain" {
		t.Fatalf("plain error: %+v", via2)
	}
}

func TestStatusFromErrCode_AndRespondHeaders(t *testing.T) {
	RegisterErrCodeHttpStatus(NotFound, http.StatusNotFound)
	if StatusFromErrCode(NotFound) != http.StatusNotFound {
		t.Fatal("registered status")
	}
	if StatusFromErrCode(Success) != http.StatusOK {
		t.Fatal("success default")
	}

	rec := httptest.NewRecorder()
	ctx := RespodWithErrHeader(t.Context())
	resp := NewErrResp(NotFound, "missing")
	if _, err := resp.Respond(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Header().Get(httpx.HeaderErrorCode) != "5" {
		t.Fatalf("Error-Code=%q", rec.Header().Get(httpx.HeaderErrorCode))
	}
	if rec.Header().Get(httpx.HeaderErrorMsg) != "missing" {
		t.Fatalf("Error-Msg=%q", rec.Header().Get(httpx.HeaderErrorMsg))
	}
}

func TestCommonResp_RespondSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := &CommonResp[string]{Code: Success, Data: "ok"}
	if _, err := resp.Respond(t.Context(), rec); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("empty body")
	}
}
