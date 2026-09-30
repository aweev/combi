package adapters

import (
	"context"
	"encoding/json"
	"fmt"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/log"
)

type VapidAdapter struct {
	privateKey string
	publicKey  string
	email      string
	logger     log.Logger
}

func NewVapidAdapter(creds map[string]string, logger log.Logger) (*VapidAdapter, error) {
	priv, ok1 := creds["private_key"]
	pub, ok2 := creds["public_key"]
	email, ok3 := creds["subject_email"]

	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf("missing VAPID credentials (private_key, public_key, subject_email)")
	}

	return &VapidAdapter{
		privateKey: priv,
		publicKey:  pub,
		email:      email,
		logger:     logger.With(log.String("provider", "VAPID")),
	}, nil
}

func (a *VapidAdapter) Send(ctx context.Context, req notification.SendWebPushRequest) (*notification.SendResult, error) {
	// 1. Decode Subscription (stored as JSON string in DB/Request)
	var sub webpush.Subscription
	if err := json.Unmarshal([]byte(req.Subscription), &sub); err != nil {
		return nil, fmt.Errorf("invalid web push subscription format: %w", err)
	}

	// 2. Payload
	payload, _ := json.Marshal(map[string]string{
		"title": req.Title,
		"body":  req.Body,
		"icon":  req.Icon,
		"url":   req.Link,
	})

	// 3. Send
	resp, err := webpush.SendNotification(payload, &sub, &webpush.Options{
		Subscriber:      a.email,
		VAPIDPublicKey:  a.publicKey,
		VAPIDPrivateKey: a.privateKey,
		TTL:             30,
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 410 {
		// The subscription is no longer valid (user revoked permission)
		return nil, notification.ErrDeliveryFailed // Caller should mark subscription inactive
	}

	return &notification.SendResult{
		ProviderID: "webpush-ok",
	}, nil
}
