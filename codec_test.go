package mix

import (
	"encoding/json"
	"testing"

	httpx "github.com/hopeio/gox/net/http"
)

func TestJsonMarshal(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	data, ct, err := JsonMarshal(t.Context(), &payload{Name: "mix"})
	if err != nil {
		t.Fatal(err)
	}
	if ct != httpx.ContentTypeJson {
		t.Fatalf("content-type=%q", ct)
	}
	var got payload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "mix" {
		t.Fatalf("got=%+v", got)
	}
}

func TestJsonMarshal_Error(t *testing.T) {
	_, ct, err := JsonMarshal(t.Context(), make(chan int))
	if err == nil {
		t.Fatal("expected marshal error")
	}
	if ct != httpx.ContentTypeText {
		t.Fatalf("content-type=%q", ct)
	}
}
