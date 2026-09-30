package datefmt_test

import (
	"strings"
	"testing"
	"time"

	"github.com/combi/pkg/i18n/datefmt"
	"github.com/combi/pkg/i18n/locale"
)

func TestToEthiopicBoundary(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want datefmt.EthiopicDate
	}{
		{
			name: "Ethiopian New Year 2016",
			date: time.Date(2023, time.September, 12, 0, 0, 0, 0, time.UTC),
			want: datefmt.EthiopicDate{Year: 2016, Month: 1, Day: 1},
		},
		{
			name: "Last day of Ethiopian year 2015",
			date: time.Date(2023, time.September, 11, 0, 0, 0, 0, time.UTC),
			want: datefmt.EthiopicDate{Year: 2015, Month: 13, Day: 6},
		},
		{
			name: "Ethiopian new year 2017",
			date: time.Date(2024, time.September, 11, 0, 0, 0, 0, time.UTC),
			want: datefmt.EthiopicDate{Year: 2017, Month: 1, Day: 1},
		},
	}

	for _, tc := range tests {
		got := datefmt.ToEthiopic(tc.date)
		if got != tc.want {
			t.Fatalf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestFormatDateUsesEthiopianCalendarAndGeezMonths(t *testing.T) {
	got := datefmt.FormatDate(time.Date(2023, time.September, 12, 0, 0, 0, 0, time.UTC), locale.AmET)
	want := "1 መስከረም 2016"
	if got != want {
		t.Fatalf("FormatDate = %q, want %q", got, want)
	}

	got = datefmt.FormatDate(time.Date(2023, time.September, 12, 0, 0, 0, 0, time.UTC), locale.TiET)
	if got != want {
		t.Fatalf("FormatDate for TiET = %q, want %q", got, want)
	}
}

func TestFormatEthiopianTime(t *testing.T) {
	if got := datefmt.FormatEthiopianTime(time.Date(2024, time.June, 14, 7, 0, 0, 0, time.UTC)); got != "01:00 ጠዋት" {
		t.Fatalf("FormatEthiopianTime = %q, want %q", got, "01:00 ጠዋት")
	}
	if got := datefmt.FormatEthiopianTime(time.Date(2024, time.June, 14, 20, 0, 0, 0, time.UTC)); got != "02:00 ማታ" {
		t.Fatalf("FormatEthiopianTime = %q, want %q", got, "02:00 ማታ")
	}
}

func TestFormatEthiopianDateTime(t *testing.T) {
	dt := time.Date(2023, time.September, 12, 7, 30, 0, 0, time.UTC)
	got := datefmt.FormatEthiopianDateTime(dt, locale.AmET)
	if !strings.Contains(got, "1 መስከረም 2016") || !strings.Contains(got, "01:30 ጠዋት") {
		t.Fatalf("FormatEthiopianDateTime = %q, want to contain both date and time", got)
	}
}
