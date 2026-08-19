package mix_test

import (
	"testing"

	"github.com/hopeio/mix"
	responsepb "github.com/hopeio/protobuf/response"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// 用 pb 版 ErrResp 验证 mix 手写的 detail wire 编码可被正确解码
// （mix 依赖 hopeio/protobuf 会成环，只能在 external test 包里做交叉验证）。
// 注意断言走 st.Proto().GetDetails()（真实上网络的 wire 层）而非 st.Details()：
// 回归 Any 套 Any bug——WithDetails 会把 anypb.Any 再包一层，
// 外层 typeUrl 变成 google.protobuf.Any，客户端就认不出了。
// detail 只携带 data，code/msg 由 status 本身承载，不重复传输。
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
	if wire[0].TypeUrl != "type.googleapis.com/response.ErrResp" {
		t.Fatalf("typeUrl = %s", wire[0].TypeUrl)
	}
	var got responsepb.ErrResp
	if err := proto.Unmarshal(wire[0].Value, &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != 0 || got.Msg != "" ||
		got.Data["type"] != "Apple" || got.Data["x"] != "y" {
		t.Fatalf("decoded = %+v", &got)
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
