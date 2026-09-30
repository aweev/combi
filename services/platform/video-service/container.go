package video

import (
	"github.com/jmoiron/sqlx"
	"github.com/combi/pkg/domain/realtime"
	"github.com/combi/pkg/persistence/postgres"
	"github.com/combi/pkg/log"
	"github.com/combi/video/domain"
	"github.com/combi/video/service"
)

type Container struct {
	Service    domain.Service
	Repository domain.Repository
	Automation *AutomationSubscriber
}

func NewContainer(db *sqlx.DB, logger log.Logger, iceServers []domain.IceServer, hub realtime.Hub) *Container {
	repo := postgres.NewVideoRepository(db, logger)
	svc := service.New(repo, logger, iceServers)
	automation := NewAutomationSubscriber(hub, logger)

	return &Container{
		Service:    svc,
		Repository: repo,
		Automation: automation,
	}
}
