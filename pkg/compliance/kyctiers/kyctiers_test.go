package kyctiers_test

import (
	"testing"

	"github.com/combi/pkg/compliance/kyctiers"
)

func TestKYCTier_CM_Level1_RequiresNationalID(t *testing.T) {
	tier, ok := kyctiers.For("CM", 1)
	if !ok {
		t.Fatal("Expected CM Tier 1 to exist")
	}
	found := false
	for _, doc := range tier.RequiredDocuments {
		if doc == "national_id_front" {
			found = true
		}
	}
	if !found {
		t.Error("CM Tier 1 should require national_id_front")
	}
}

func TestKYCTier_NG_Level1_RequiresBVN(t *testing.T) {
	tier, ok := kyctiers.For("NG", 1)
	if !ok {
		t.Fatal("Expected NG Tier 1 to exist")
	}
	found := false
	for _, doc := range tier.RequiredDocuments {
		if doc == "bvn_number" {
			found = true
		}
	}
	if !found {
		t.Error("NG Tier 1 should require bvn_number")
	}
}

func TestKYCTier_ET_Level1_RequiresKebeleID(t *testing.T) {
	tier, ok := kyctiers.For("ET", 1)
	if !ok {
		t.Fatal("Expected ET Tier 1 to exist")
	}
	found := false
	for _, doc := range tier.RequiredDocuments {
		if doc == "kebele_id" {
			found = true
		}
	}
	if !found {
		t.Error("ET Tier 1 should require kebele_id")
	}
}

func TestKYCTier_Level3_AutoApprovable_IsFalse(t *testing.T) {
	tier, ok := kyctiers.For("CM", 3)
	if !ok {
		t.Fatal("Expected CM Tier 3 to exist")
	}
	if tier.AutoApprovable {
		t.Error("Tier 3 should never be auto-approvable (always manual review)")
	}
}
