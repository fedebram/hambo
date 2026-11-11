package noiseconn

import (
	"bytes"
	"testing"
)

func TestPreludeJoin(t *testing.T) {
	var buf bytes.Buffer
	id := []byte{1, 2, 3, 4, 5, 6}

	if err := writePrelude(&buf, modeJoin, id); err != nil {
		t.Fatal(err)
	}

	mode, gotID, raw, err := readPrelude(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if mode != modeJoin {
		t.Fatalf("mode: got %d, want %d", mode, modeJoin)
	}
	if !bytes.Equal(gotID, id) {
		t.Fatalf("id: got %v, want %v", gotID, id)
	}
	if len(raw) != 7 {
		t.Fatalf("raw: got %d bytes, want 7", len(raw))
	}
}
