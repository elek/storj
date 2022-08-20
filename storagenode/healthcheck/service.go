package healthcheck

import (
	"context"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/reputation"
	"time"
)

var (
	// HealthCheckErr defines sno service error.
	HealthCheckErr = errs.Class("healthcheck")

	mon = monkit.Package()
)

// Service is handling storage node estimation payouts logic.
//
// architecture: Service
type Service struct {
	reputationDB reputation.DB
}

// NewService returns new instance of Service.
func NewService(reputationDB reputation.DB) *Service {
	return &Service{
		reputationDB: reputationDB,
	}
}

type Health struct {
	Statuses []SatelliteHealthStatus
	Help     string
	Healthy  bool
}

type SatelliteHealthStatus struct {
	OnlineScore    float64
	SatelliteID    storj.NodeID
	DisqualifiedAt *time.Time
	SuspendedAt    *time.Time
}

func (s *Service) GetHealth(ctx context.Context) (h Health, err error) {
	stats, err := s.reputationDB.All(ctx)
	if err != nil {
		return h, HealthCheckErr.Wrap(err)
	}
	for _, stat := range stats {
		if stat.DisqualifiedAt == nil || stat.SuspendedAt == nil || stat.OnlineScore > 0.9 {
			h.Healthy = true
		}

		h.Statuses = append(h.Statuses, SatelliteHealthStatus{
			SatelliteID:    stat.SatelliteID,
			OnlineScore:    stat.OnlineScore,
			DisqualifiedAt: stat.DisqualifiedAt,
			SuspendedAt:    stat.SuspendedAt,
		})
	}
	h.Help = "To access Storagenode services, please use DRPC protocol!"

	return h, nil
}
