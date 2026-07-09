package mix

import (
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestCommonResp(t *testing.T) {
	data, _ := (&CommonProtoResp[*wrapperspb.BoolValue]{
		Code: 1,
		Msg:  "1",
	}).MarshalProto()
	t.Log(data)
	var resp CommonProtoResp[*wrapperspb.BoolValue]
	resp.UnmarshalProto(data)
	t.Log(resp)
}
