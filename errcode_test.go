package mix

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	httpx "github.com/hopeio/gox/net/http"
	"google.golang.org/grpc/codes"
)

func TestErrCode_StringAndHelpers(t *testing.T) {
	if NotFound.String() != "NotFound" {
		t.Fatalf("String=%q", NotFound.String())
	}
	RegisterErrCode(1001, "Custom")
	if ErrCode(1001).String() != "Custom" {
		t.Fatalf("custom String=%q", ErrCode(1001).String())
	}
	er := NotFound.Msg("gone", nil)
	if er.Code != NotFound || er.Msg != "gone" {
		t.Fatalf("Msg helper: %+v", er)
	}
	er2 := Internal.Wrap(errors.New("boom"))
	if er2.Code != Internal || er2.Msg != "boom" {
		t.Fatalf("Wrap: %+v", er2)
	}
}

func TestRegisterErrCodeMap(t *testing.T) {
	RegisterErrCodeMap(map[int32]string{
		2001: "FromMap",
		2002: "AlsoMap",
	})
	if ErrCode(2001).String() != "FromMap" {
		t.Fatalf("2001 String=%q", ErrCode(2001).String())
	}
	if ErrCode(2002).String() != "AlsoMap" {
		t.Fatalf("2002 String=%q", ErrCode(2002).String())
	}
}

func TestErrCode_Error(t *testing.T) {
	if NotFound.Error() != "NotFound" {
		t.Fatalf("Error()=%q", NotFound.Error())
	}
	if ErrCode(9999).Error() != "Unknown Error, Code:9999" {
		t.Fatalf("unknown Error()=%q", ErrCode(9999).Error())
	}
}

func TestErrCodeLayout(t *testing.T) {
	c := ErrCode(3001)*100 + ErrCode(codes.PermissionDenied)
	if c.BizCode() != 3001 || c.GRPCCode() != codes.PermissionDenied {
		t.Fatalf("roundtrip: biz=%d grpc=%v", c.BizCode(), c.GRPCCode())
	}
	// 业务码以纯值注册过（RegisterErrCodeMap 注册 proto 枚举）时，composite 能拆出名字
	RegisterErrCode(3001, "CustomBiz")
	if c.String() != "CustomBiz" {
		t.Fatalf("composite String=%q", c.String())
	}
	// 纯 gRPC 错误：BizCode 为 0，GRPCCode 为自身
	if NotFound.BizCode() != 0 || NotFound.GRPCCode() != codes.NotFound {
		t.Fatal("pure grpc code layout")
	}
	// 未注册的 composite 保留完整码信息
	if (ErrCode(3999)*100 + ErrCode(codes.NotFound)).String() != "Unknown Error, Code:"+strconv.Itoa(3999*100+5) {
		t.Fatalf("unregistered String=%q", (ErrCode(3999)*100 + ErrCode(codes.NotFound)).String())
	}
}

func TestErrCode_GRPCStatus(t *testing.T) {
	st := InvalidArgument.GRPCStatus()
	if st.Code().String() != "InvalidArgument" {
		t.Fatalf("grpc code=%v", st.Code())
	}
	// msg 携带 composite 整值，纯 gRPC 错误时即 code 自身
	if st.Message() != "3" {
		t.Fatalf("grpc message=%q", st.Message())
	}
	// 业务码不再被当成非法 gRPC code 发出去
	custom := ErrCode(1001)*100 + ErrCode(codes.InvalidArgument)
	st2 := custom.GRPCStatus()
	if st2.Code() != codes.InvalidArgument {
		t.Fatalf("custom grpc code=%v", st2.Code())
	}
	if st2.Message() != strconv.Itoa(int(custom)) {
		t.Fatalf("custom grpc message=%q", st2.Message())
	}
}

func TestErrRespFrom(t *testing.T) {
	if ErrRespFrom(nil) != nil {
		t.Fatal("nil err -> nil")
	}
	orig := NewErrResp(NotFound, "x", nil)
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
	resp := NewErrResp(NotFound, "missing", nil)
	if _, err := resp.Respond(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Header().Get(httpx.HeaderErrorCode) != "5" {
		t.Fatalf("Error-Code=%q", rec.Header().Get(httpx.HeaderErrorCode))
	}
	if rec.Header().Get(httpx.HeaderErrorMsg) != "" || rec.Header().Get(httpx.HeaderGrpcStatus) != "" {
		t.Fatal("only Error-Code header expected")
	}
}

func TestErrResp_RespondAlwaysWritesHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := NewErrResp(NotFound, "auth.err.missing", map[string]string{"id": "1"})
	if _, err := resp.Respond(t.Context(), rec); err != nil {
		t.Fatal(err)
	}
	if rec.Header().Get(httpx.HeaderErrorCode) != "5" {
		t.Fatalf("Error-Code=%q", rec.Header().Get(httpx.HeaderErrorCode))
	}
	// 只写 Error-Code：msg 走响应体（JSON msg / ErrorInfo.reason）。
	if rec.Header().Get(httpx.HeaderErrorMsg) != "" || rec.Header().Get(httpx.HeaderGrpcStatus) != "" {
		t.Fatal("only Error-Code header expected")
	}
	// composite：Error-Code 整值，msg 仍在 body
	rec2 := httptest.NewRecorder()
	resp2 := NewErrResp(ErrCode(1001)*100+ErrCode(codes.PermissionDenied), "auth.err.missing", nil)
	if _, err := resp2.Respond(t.Context(), rec2); err != nil {
		t.Fatal(err)
	}
	if rec2.Header().Get(httpx.HeaderErrorCode) != strconv.Itoa(1001*100+7) {
		t.Fatalf("composite Error-Code=%q", rec2.Header().Get(httpx.HeaderErrorCode))
	}
	if rec2.Header().Get(httpx.HeaderGrpcStatus) != "" {
		t.Fatalf("composite Grpc-Status=%q", rec2.Header().Get(httpx.HeaderGrpcStatus))
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
