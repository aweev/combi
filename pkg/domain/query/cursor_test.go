package query

import (
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	original := Cursor{
		SortValue: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC),
		ID:        "idn_01HXYZ",
	}

	encoded, err := EncodeCursor(original)
	if err != nil {
		t.Fatalf("EncodeCursor: %v", err)
	}
	decoded, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if !decoded.SortValue.Equal(original.SortValue) {
		t.Errorf("SortValue = %v, want %v", decoded.SortValue, original.SortValue)
	}
	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
}

func TestDecodeCursorEmptyIsStart(t *testing.T) {
	c, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("DecodeCursor(\"\"): %v", err)
	}
	if c != (Cursor{}) {
		t.Errorf("empty cursor should decode to zero value, got %+v", c)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	if _, err := DecodeCursor("!!not-base64!!"); err == nil {
		t.Error("invalid base64 should be rejected")
	}
	if _, err := DecodeCursor("aGVsbG8="); err == nil { // valid b64, invalid JSON payload
		t.Error("non-cursor JSON should be rejected")
	}
}

func TestCursorRequestEffectiveLimit(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, DefaultCursorLimit},
		{-5, DefaultCursorLimit},
		{1, 1},
		{50, 50},
		{100, 100},
		{500, MaxCursorLimit},
	}
	for _, tc := range cases {
		r := CursorRequest{Limit: tc.in}
		if got := r.EffectiveLimit(); got != tc.want {
			t.Errorf("EffectiveLimit(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestBuildCursorResult(t *testing.T) {
	items := []time.Time{time.Now(), time.Now()}
	last := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

	res, err := BuildCursorResult(items, true, last, "idn_last")
	if err != nil {
		t.Fatalf("BuildCursorResult: %v", err)
	}
	if !res.HasMore {
		t.Error("HasMore should be true")
	}
	if res.NextCursor == "" {
		t.Error("NextCursor must be set when HasMore")
	}

	// last page: no cursor
	res, err = BuildCursorResult(items, false, last, "idn_last")
	if err != nil {
		t.Fatalf("BuildCursorResult: %v", err)
	}
	if res.HasMore || res.NextCursor != "" {
		t.Errorf("last page should have no cursor, got %q", res.NextCursor)
	}
}
