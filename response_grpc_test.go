package mix_test

import (
	"strconv"
	"testing"

	"github.com/hopeio/mix"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// detail 用标准 google.rpc.ErrorInfo 携带 i18n key（reason）与占位符变量（metadata）：
// status.code 是低两位（<100）标准枚举，status.msg 是 composite 整值。
// 断言走 st.Proto().GetDetails()（真实上网络的 wire 层），回归 Any 套 Any bug——
// WithDetails 传 anypb.Any 会被再包一层。
func TestErrRespGRPCStatusDetail(t *testing.T) {
	e := mix.NewErrResp(mix.InvalidArgument, "auth.err.thirdLogin", map[string]string{"type": "Apple", "x": "y"})
	st := e.GRPCStatus()
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v", st.Code())
	}
	if st.Message() != "3" {
		t.Fatalf("message = %q", st.Message())
	}
	wire := st.Proto().GetDetails()
	if len(wire) != 1 {
		t.Fatalf("details = %d", len(wire))
	}
	if wire[0].TypeUrl != "type.googleapis.com/google.rpc.ErrorInfo" {
		t.Fatalf("typeUrl = %s", wire[0].TypeUrl)
	}
	var got errdetails.ErrorInfo
	if err := proto.Unmarshal(wire[0].Value, &got); err != nil {
		t.Fatal(err)
	}
	if got.Reason != "auth.err.thirdLogin" {
		t.Fatalf("reason = %q", got.Reason)
	}
	if got.Metadata["type"] != "Apple" || got.Metadata["x"] != "y" {
		t.Fatalf("metadata = %v", got.Metadata)
	}
	// grpc-go 的 Details() 能自动解出标准类型。
	if d, ok := st.Details()[0].(*errdetails.ErrorInfo); !ok || d.Metadata["type"] != "Apple" {
		t.Fatalf("decoded detail = %T", st.Details()[0])
	}
}

func TestErrRespFromGRPCStatusDetails(t *testing.T) {
	src := mix.NewErrResp(mix.InvalidArgument, "auth.err.thirdLogin", map[string]string{"type": "Apple"})
	got := mix.ErrRespFrom(src.GRPCStatus().Err())
	if got.Code != mix.InvalidArgument || got.Msg != "auth.err.thirdLogin" || got.Data["type"] != "Apple" {
		t.Fatalf("got=%+v", got)
	}
}

// composite 业务码经 gRPC 通道往返：code 拆低两位（<100）、msg 放整值、reason 放 i18n key。
func TestErrRespCompositeRoundtrip(t *testing.T) {
	code := mix.ErrCode(1001)*100 + mix.ErrCode(codes.PermissionDenied)
	e := mix.NewErrResp(code, "auth.err.notActivated", map[string]string{"a": "b"})
	st := e.GRPCStatus()
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("code = %v", st.Code())
	}
	if st.Message() != strconv.Itoa(int(code)) {
		t.Fatalf("message = %q", st.Message())
	}
	got := mix.ErrRespFrom(st.Err())
	if got.Code != code || got.Msg != "auth.err.notActivated" || got.Data["a"] != "b" {
		t.Fatalf("got=%+v", got)
	}
}

// msg/data 全空时不带 details，保持轻量。
func TestErrRespGRPCStatusNoDetail(t *testing.T) {
	st := mix.NewErrResp(mix.Internal, "", nil).GRPCStatus()
	if len(st.Details()) != 0 {
		t.Fatalf("details = %v", st.Details())
	}
}

// data 为空但 msg 非空：reason（i18n key）也要传给客户端。
func TestErrRespGRPCStatusReasonWithoutData(t *testing.T) {
	st := mix.InvalidArgument.Msg("auth.err.notActivated", nil).GRPCStatus()
	details := st.Details()
	if len(details) != 1 {
		t.Fatalf("details = %v", details)
	}
	if d, ok := details[0].(*errdetails.ErrorInfo); !ok || d.Reason != "auth.err.notActivated" {
		t.Fatalf("detail = %+v", details[0])
	}
	if status.Code(mix.InvalidArgument.Msg("x", nil).GRPCStatus().Err()) != codes.InvalidArgument {
		t.Fatal("code")
	}
}
