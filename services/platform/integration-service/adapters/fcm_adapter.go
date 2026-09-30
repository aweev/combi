package adapters

import (
	"context"
	"fmt"

	"github.com/combi/pkg/domain/notification"
	"github.com/combi/pkg/log"
)

type FCMAdapter struct {
	apiKey string
	logger log.Logger
}

func NewFCMAdapter(creds map[string]string, logger log.Logger) (*FCMAdapter, error) {
	apiKey, ok := creds["api_key"]
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("missing 'api_key' for FCM provider")
	}
	return &FCMAdapter{
		apiKey: apiKey,
		logger: logger.With(log.String("provider", "FCM")),
	}, nil
}

func (a *FCMAdapter) Send(ctx context.Context, req notification.SendPushRequest) (*notification.SendPushResult, error) {
	// Placeholder: In a real app, use "firebase.google.com/go/messaging"
	a.logger.Info("sending push notification via FCM",
		log.String("device_token", req.DeviceToken),
		log.String("title", req.Title),
	)

	// Simulate success
	return &notification.SendPushResult{
		ProviderID: "msg-" + req.ReferenceID, // Mock ID
	}, nil
}
