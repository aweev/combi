package auth

import (
	"fmt"
	"strings"
	"time"
)

type Channel string

type Challenge struct {
	Channel   Channel
	Reference string
	Payload   string
	ExpiresAt time.Time
}

type Provider interface {
	Send(destination string, payload string) (string, error)
}

const (
	ChannelSMS      Channel = "sms"
	ChannelWhatsApp Channel = "whatsapp"
	ChannelVoice    Channel = "voice"
	ChannelEmail    Channel = "email"
)

type Dispatcher struct {
	providers map[Channel]Provider
}

func NewDispatcher(providers map[Channel]Provider) *Dispatcher {
	return &Dispatcher{providers: providers}
}

func (d *Dispatcher) Dispatch(destination string, payload string, preferred []Channel) (*Challenge, error) {
	for _, ch := range preferred {
		provider, ok := d.providers[ch]
		if !ok {
			continue
		}
		reference, err := provider.Send(destination, payload)
		if err == nil {
			return &Challenge{Channel: ch, Reference: reference, Payload: payload, ExpiresAt: time.Now().Add(5 * time.Minute)}, nil
		}
	}

	for ch, provider := range d.providers {
		reference, err := provider.Send(destination, payload)
		if err == nil {
			return &Challenge{Channel: ch, Reference: reference, Payload: payload, ExpiresAt: time.Now().Add(5 * time.Minute)}, nil
		}
	}
	return nil, fmt.Errorf("auth: no available channel")
}

func FlashCallOTP(phoneNumber string, code string) string {
	return strings.TrimSpace(fmt.Sprintf("+%s#%s", strings.TrimSpace(phoneNumber), strings.TrimSpace(code)))
}

func SanitizeDestination(dest string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(dest)), "")
}
