package limits_test

import (
	"testing"

	"github.com/combi/pkg/compliance/limits"
	"github.com/shopspring/decimal"
)

func TestPaymentLimit_CM_Level1_Daily200USD(t *testing.T) {
	limit, ok := limits.For("CM", 1)
	if !ok {
		t.Fatal("Expected CM limit level 1 to exist")
	}
	expected := decimal.NewFromInt(200)
	if !limit.DailySendUSD.Equal(expected) {
		t.Errorf("Expected 200 USD daily limit, got %s", limit.DailySendUSD.String())
	}
}

func TestPaymentLimit_CM_Level1_NoCrossBorder(t *testing.T) {
	limit, ok := limits.For("CM", 1)
	if !ok {
		t.Fatal("Expected CM limit level 1 to exist")
	}
	if !limit.CrossBorderPct.IsZero() {
		t.Errorf("Expected 0 cross border allowance, got %s", limit.CrossBorderPct.String())
	}
}

func TestPaymentLimit_NG_Level3_Unlimited(t *testing.T) {
	limit, ok := limits.For("NG", 3)
	if !ok {
		t.Fatal("Expected NG limit level 3 to exist")
	}
	if !limit.Unlimited {
		t.Error("Expected NG Tier 3 to be marked as unlimited")
	}
}
