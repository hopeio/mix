package gateway

import (
	"testing"

	httpx "github.com/hopeio/gox/net/http"
	"github.com/hopeio/mix"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
)

func TestProtobufMarshal_ErrRespErrorInfo(t *testing.T) {
	er := mix.NewErrResp(mix.InvalidArgument, "auth.err.thirdLogin", map[string]string{"type": "Apple"})
	data, ct, err := ProtobufMarshal(t.Context(), er)
	if err != nil {
		t.Fatal(err)
	}
	if ct != httpx.ContentTypeProtobuf {
		t.Fatalf("content-type=%q", ct)
	}
	var ei errdetails.ErrorInfo
	if err := proto.Unmarshal(data, &ei); err != nil {
		t.Fatal(err)
	}
	if ei.Reason != "auth.err.thirdLogin" || ei.Metadata["type"] != "Apple" {
		t.Fatalf("ErrorInfo=%+v", ei)
	}
}
