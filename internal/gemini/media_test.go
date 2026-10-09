package gemini

import (
	"encoding/json"
	"testing"
)

func TestExtractTurnMedia(t *testing.T) {
	payload := []any{
		nil,
		[]any{"cid", "rid"},
		nil,
		nil,
		[]any{[]any{
			"rc_1",
			[]any{"a red square"},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			[]any{nil, nil, nil, nil, nil, nil, nil, []any{[]any{
				[]any{nil, nil, nil, []any{nil, 1, "out.jpg", "https://lh3.googleusercontent.com/gg-dl/OPAQUE", nil, "x", nil, nil, nil, nil, nil, "image/jpeg"}},
				[]any{"http://googleusercontent.com/image_generation_content/1"},
			}}},
		}},
	}
	for len(payload) < 43 {
		payload = append(payload, nil)
	}
	payload[42] = "3.6 Flash"
	inner, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := json.Marshal([]any{[]any{"wrb.fr", nil, string(inner)}})
	if err != nil {
		t.Fatal(err)
	}
	turn, err := ExtractTurn(string(frame))
	if err != nil {
		t.Fatal(err)
	}
	if turn.Text != "a red square" {
		t.Fatalf("text %q", turn.Text)
	}
	if turn.ServedModel != "3.6 Flash" {
		t.Fatalf("served %q", turn.ServedModel)
	}
	if len(turn.Media) != 1 || turn.Media[0].MIME != "image/jpeg" {
		t.Fatalf("media %#v", turn.Media)
	}
}
