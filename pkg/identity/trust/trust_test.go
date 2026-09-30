package trust_test

import (
	"testing"
	"time"

	"github.com/combi/pkg/identity/trust"
)

func TestNewVouchAndVerify(t *testing.T) {
	pub, priv, err := trust.NewKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	vouch, err := trust.NewVouch("issuer1", "subject1", trust.ClaimID, priv, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := vouch.Verify(pub); err != nil {
		t.Fatal(err)
	}
}

func TestTrustNodeScoring(t *testing.T) {
	node := trust.TrustNode{NodeID: "n1", Score: 40}
	score := node.AddWeight(30)
	newNode := trust.TrustNode{NodeID: node.NodeID, Score: score}
	if !newNode.IsTrusted(trust.TrustScore(70)) {
		t.Fatal("expected node trusted after adding weight")
	}
}

func TestQRCodeEncoding(t *testing.T) {
	data := map[string]string{"k": "v"}
	encoded, err := trust.EncodeToQRCode(data)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]string
	if err := trust.DecodeFromQRCode(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["k"] != "v" {
		t.Fatalf("unexpected decoded data: %v", decoded)
	}
}
