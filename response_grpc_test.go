package mix_test

import (
	"testing"

	"github.com/hopeio/mix"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// detail 用标准 google.rpc.ErrorInfo 携带 i18n 变量：code/msg 在 status 本身，
// data 放进 metadata。断言走 st.Proto().GetDetails()（真实上网络的 wire 层），
// 回归 Any 套 Any bug——WithDetails 传 anypb.Any 会被再包一层。
func TestErrRespGRPCStatusDetail(t *testing.T) {
	e := mix.NewErrResp(mix.InvalidArgument, "auth.err.thirdLogin", map[string]string{"type": "Apple", "x": "y"})
	st := e.GRPCStatus()
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v", st.Code())
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
	if got.Metadata["type"] != "Apple" || got.Metadata["x"] != "y" {
		t.Fatalf("metadata = %v", got.Metadata)
	}
	// grpc-go 的 Details() 能自动解出标准类型。
	if d, ok := st.Details()[0].(*errdetails.ErrorInfo); !ok || d.Metadata["type"] != "Apple" {
		t.Fatalf("decoded detail = %T", st.Details()[0])
	}
}

// data 为空时不带 details，保持原行为。
func TestErrRespGRPCStatusNoData(t *testing.T) {
	st := mix.InvalidArgument.Msg("auth.err.notActivated").GRPCStatus()
	if len(st.Details()) != 0 {
		t.Fatalf("details = %v", st.Details())
	}
	if status.Code(mix.InvalidArgument.Msg("x").GRPCStatus().Err()) != codes.InvalidArgument {
		t.Fatal("code")
	}
}
