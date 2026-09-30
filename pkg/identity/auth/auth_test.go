package auth_test

import (
	"testing"

	"github.com/combi/pkg/identity/auth"
)

type mockProvider struct {
	called   bool
	dest     string
	payload  string
	response string
	err      error
}

func (m *mockProvider) Send(destination string, payload string) (string, error) {
	m.called = true
	m.dest = destination
	m.payload = payload
	return m.response, m.err
}

func TestDispatcherUsesPreferredChannel(t *testing.T) {
	sms := &mockProvider{response: "ref-sms"}
	email := &mockProvider{response: "ref-email"}
	dispatcher := auth.NewDispatcher(map[auth.Channel]auth.Provider{
		auth.ChannelSMS:   sms,
		auth.ChannelEmail: email,
	})

	challenge, err := dispatcher.Dispatch("+1234567890", "otp123", []auth.Channel{auth.ChannelSMS})
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Channel != auth.ChannelSMS {
		t.Fatalf("expected sms, got %s", challenge.Channel)
	}
}

func TestDispatcherFallsBackWhenPreferredUnavailable(t *testing.T) {
	voice := &mockProvider{response: "ref-voice"}
	dispatcher := auth.NewDispatcher(map[auth.Channel]auth.Provider{
		auth.ChannelVoice: voice,
	})

	challenge, err := dispatcher.Dispatch("+1234567890", "otp456", []auth.Channel{auth.ChannelSMS})
	if err != nil {
		t.Fatal(err)
	}
	if challenge.Channel != auth.ChannelVoice {
		t.Fatalf("expected voice fallback, got %s", challenge.Channel)
	}
}

func TestFlashCallOTP(t *testing.T) {
	if got := auth.FlashCallOTP("254712345678", "1234"); got != "+254712345678#1234" {
		t.Fatalf("unexpected flash call string: %s", got)
	}
}

func TestSanitizeDestination(t *testing.T) {
	if got := auth.SanitizeDestination(" +254 712 345 678 "); got != "+254712345678" {
		t.Fatalf("sanitize failed: %s", got)
	}
}
