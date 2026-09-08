package mix

import (
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestCommonResp(t *testing.T) {
	// Code!=0 with Msg and Data: the old encoder silently dropped Data.
	data, err := (&CommonProtoResp[*wrapperspb.BoolValue]{
		Code: 1,
		Msg:  "1",
		Data: wrapperspb.Bool(true),
	}).MarshalProto()
	if err != nil {
		t.Fatal(err)
	}
	var resp CommonProtoResp[*wrapperspb.BoolValue]
	if err := resp.UnmarshalProto(data); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 1 || resp.Msg != "1" || resp.Data == nil || !resp.Data.Value {
		t.Fatalf("roundtrip mismatch: %+v", resp)
	}

	// Code==0 with Data only.
	data, err = (&CommonProtoResp[*wrapperspb.BoolValue]{
		Data: wrapperspb.Bool(false),
	}).MarshalProto()
	if err != nil {
		t.Fatal(err)
	}
	var resp2 CommonProtoResp[*wrapperspb.BoolValue]
	if err := resp2.UnmarshalProto(data); err != nil {
		t.Fatal(err)
	}
	if resp2.Data == nil || resp2.Data.Value {
		t.Fatalf("roundtrip mismatch: %+v", resp2)
	}

	// Empty input must not panic.
	var resp3 CommonProtoResp[*wrapperspb.BoolValue]
	if err := resp3.UnmarshalProto(nil); err != nil {
		t.Fatal(err)
	}
}
