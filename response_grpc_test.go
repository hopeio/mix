package mix_test

import (
	"testing"

	"github.com/hopeio/mix"
	responsepb "github.com/hopeio/protobuf/response"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// 用 pb 版 ErrResp 验证 mix 手写的 detail wire 编码可被正确解码
// （mix 依赖 hopeio/protobuf 会成环，只能在 external test 包里做交叉验证）。
func TestErrRespGRPCStatusDetail(t *testing.T) {
	e := mix.NewErrResp(mix.InvalidArgument, "auth.err.thirdLogin", map[string]string{"type": "Apple", "x": "y"})
	st := e.GRPCStatus()
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code = %v", st.Code())
	}
	details := st.Details()
	if len(details) != 1 {
		t.Fatalf("details = %d", len(details))
	}
	any, ok := details[0].(*anypb.Any)
	if !ok {
		t.Fatalf("detail type %T", details[0])
	}
	if any.TypeUrl != "type.googleapis.com/response.ErrResp" {
		t.Fatalf("typeUrl = %s", any.TypeUrl)
	}
	var got responsepb.ErrResp
	if err := proto.Unmarshal(any.Value, &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != int32(mix.InvalidArgument) || got.Msg != "auth.err.thirdLogin" ||
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
