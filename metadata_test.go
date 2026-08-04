package mix

import (
	"context"
	"testing"
)

func TestMetadata_SetGetDel(t *testing.T) {
	var m Metadata
	if m.Get("k") != nil {
		t.Fatal("empty Get should be nil")
	}
	m.Del("missing") // nil DataM must not panic

	m.Set("k", 1)
	if got := m.Get("k"); got != 1 {
		t.Fatalf("Get=%v want 1", got)
	}
	m.Del("k")
	if m.Get("k") != nil {
		t.Fatal("after Del Get should be nil")
	}
}

func TestMetadata_SetDataGetData(t *testing.T) {
	var m Metadata
	if m.GetData() != nil {
		t.Fatal("empty GetData should be nil")
	}
	m.SetData("payload")
	if got := m.GetData(); got != "payload" {
		t.Fatalf("GetData=%v want payload", got)
	}
}

func TestWithMetadata_GetMetadata(t *testing.T) {
	if GetMetadata(context.Background()) != nil {
		t.Fatal("no metadata expected")
	}
	md := &Metadata{Token: "t"}
	ctx := WithMetadata(context.Background(), md)
	got := GetMetadata(ctx)
	if got != md {
		t.Fatal("want same Metadata pointer")
	}
	if got.Token != "t" {
		t.Fatalf("Token=%q", got.Token)
	}
}
