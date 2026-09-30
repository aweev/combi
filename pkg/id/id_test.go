package id_test

import (
	"strings"
	"testing"
	"time"

	"github.com/combi/pkg/id"
)

func TestSpecPrefixesRoundTrip(t *testing.T) {
	prefixes := []struct {
		name string
		p    string
	}{
		{"identity", id.PrefixIdentity},
		{"device", id.PrefixDevice},
		{"tenant", id.PrefixTenant},
		{"session", id.PrefixSession},
		{"notification", id.PrefixNotification},
		{"payment", id.PrefixPayment},
		{"event", id.PrefixEvent},
	}

	for _, tc := range prefixes {
		v, err := id.New(tc.p)
		if err != nil {
			t.Fatalf("%s: New(%q): %v", tc.name, tc.p, err)
		}
		if !strings.HasPrefix(v, tc.p+"_") {
			t.Fatalf("%s: %q lacks prefix %q", tc.name, v, tc.p)
		}
		if !id.Validate(v) {
			t.Fatalf("%s: Validate(%q) = false", tc.name, v)
		}
		gotPrefix, gotULID, err := id.Parse(v)
		if err != nil {
			t.Fatalf("%s: Parse(%q): %v", tc.name, v, err)
		}
		if gotPrefix != tc.p || len(gotULID) != 26 {
			t.Fatalf("%s: Parse → (%q, %q)", tc.name, gotPrefix, gotULID)
		}
		if !id.HasPrefix(v, tc.p) {
			t.Fatalf("%s: HasPrefix(%q, %q) = false", tc.name, v, tc.p)
		}
	}
}

func TestNewRejectsInvalidPrefixes(t *testing.T) {
	for _, bad := range []string{"", "x", "abcdefghi", "bad-prefix", "ID?", "no spaces"} {
		if _, err := id.New(bad); err == nil {
			t.Errorf("New(%q) accepted invalid prefix", bad)
		}
	}
}

func TestNewAcceptsCaseInsensitivePrefix(t *testing.T) {
	v, err := id.New("IDN")
	if err != nil {
		t.Fatalf("New(IDN): %v", err)
	}
	if !strings.HasPrefix(v, "idn_") {
		t.Fatalf("New(IDN) = %q, want canonical lowercase prefix", v)
	}
}

func TestULIDFormat(t *testing.T) {
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	for i := 0; i < 100; i++ {
		u, err := id.NewULID()
		if err != nil {
			t.Fatalf("NewULID: %v", err)
		}
		if len(u) != 26 {
			t.Fatalf("len(%q) = %d, want 26", u, len(u))
		}
		for _, r := range u {
			if !strings.ContainsRune(alphabet, r) {
				t.Fatalf("ULID %q contains non-alphabet rune %q", u, r)
			}
		}
	}
}

func TestULIDSortable(t *testing.T) {
	first, err := id.New(id.PrefixIdentity)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * time.Millisecond)
	second, err := id.New(id.PrefixIdentity)
	if err != nil {
		t.Fatal(err)
	}
	_, u1, _ := id.Parse(first)
	_, u2, _ := id.Parse(second)
	if u1 >= u2 {
		t.Fatalf("ULIDs must sort by time: %q >= %q", u1, u2)
	}
}

func TestValidateRejectsMalformed(t *testing.T) {
	for _, bad := range []string{
		"",                            // empty
		"01ARZ3NDEKTSV4RRFFQ69G5FAV",  // no prefix
		"idn_",                       // no ULID
		"idn_01ARZ3NDEKTSV4RRFFQ69G5FA", // 25 chars
		"idn_01ARZ3NDEKTSV4RRFFQ69G5FAVO", // 27 chars
		"idn_01ARZ3NDEKTSV4RRFFQ69G5FAI", // 'I' excluded from alphabet
		"idn_01ARZ3NDEKTSV4RRFFQ69G5FAU", // 'U' excluded from alphabet
		"bad_1_01ARZ3NDEKTSV4RRFFQ69G5FAV", // prefix with separator
	} {
		if id.Validate(bad) {
			t.Errorf("Validate(%q) = true, want false", bad)
		}
	}
}

func TestParseCanonicalizesCase(t *testing.T) {
	p, u, err := id.Parse("idn_01arz3ndektsv4rrffq69g5fav")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if p != "idn" || u != "01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Fatalf("Parse → (%q, %q), want canonical lowercase prefix + uppercase ULID", p, u)
	}
}

func TestMustNew(t *testing.T) {
	v := id.MustNew(id.PrefixPayment)
	if !id.Validate(v) {
		t.Fatal("MustNew produced an invalid id")
	}
}

func TestMustNewPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal(`MustNew("") must panic`)
		}
	}()
	id.MustNew("")
}

func TestMask(t *testing.T) {
	v := id.MustNew(id.PrefixIdentity)
	_, u, _ := id.Parse(v)
	want := id.PrefixIdentity + "_" + u[:6] + "…" + u[22:]

	if got := id.Mask(v); got != want {
		t.Fatalf("Mask = %q, want %q", got, want)
	}
	if id.Mask("garbage") != "" {
		t.Fatal(`Mask("garbage") must be ""`)
	}
}

func TestDID(t *testing.T) {
	v := id.MustNew(id.PrefixIdentity)
	did, err := id.DID(v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(did, "did:afros:idn_") {
		t.Fatalf("DID = %q, want did:afros:idn_ prefix", did)
	}
	if _, err := id.DID("not-an-id"); err == nil {
		t.Fatal("DID must reject invalid ids")
	}
}

func TestConcurrentNewUnique(t *testing.T) {
	const total = 200
	ids := make(chan string, total)
	perWorker := total / 8
	for i := 0; i < 8; i++ {
		go func() {
			for j := 0; j < perWorker; j++ {
				ids <- id.MustNew(id.PrefixEvent)
			}
		}()
	}

	seen := make(map[string]bool, total)
	for i := 0; i < total; i++ {
		v := <-ids
		if seen[v] {
			t.Fatalf("duplicate id %q", v)
		}
		seen[v] = true
	}
}