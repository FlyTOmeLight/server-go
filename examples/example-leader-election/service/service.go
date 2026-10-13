package service

import (
	"context"
	"fmt"
	"time"

	"github.com/FlyTOmeLight/server-go/examples/example-leader-election/internal/app/leadership"
	"github.com/FlyTOmeLight/server-go/pkg/election"
)

type Config struct {
	AppName, Kubeconfig, Namespace, ID string
	LeaseDuration                      time.Duration
	Local                              bool
}

type Service struct{ election election.LeaderElection }

func New(cfg Config) *Service {
	var e election.LeaderElection
	if cfg.Local {
		e = election.NewDummyAppElection(cfg.ID)
	} else {
		e = election.NewLeaderElection(
			cfg.AppName,
			election.WithKubeConfigFilePath(cfg.Kubeconfig),
			election.WithLeaseDuration(cfg.LeaseDuration),
			election.WithNamespace(cfg.Namespace),
			election.WithIdentify(cfg.ID),
		)
	}
	e.RunOnLeader(leadership.Task)
	e.WithNewLeaderCallback(func(id string) { fmt.Println("new leader:", id) })
	e.WithStartedLeadingCallback(func(context.Context) { fmt.Println("I am leader") })
	e.WithStoppedLeadingCallback(func() { fmt.Println("leader is lost:", e.GetLeader()) })
	return &Service{election: e}
}

// Run waits until ctx is canceled and all leader tasks have returned.
func (s *Service) Run(ctx context.Context) { s.election.Run(ctx) }
