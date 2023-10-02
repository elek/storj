package overlay

import (
	"context"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"storj.io/storj/satellite/nodeevents"
)

func createOverlay(lc fx.Lifecycle,
	shutdowner fx.Shutdowner,
	log *zap.Logger,
	overlayDB DB,
	nodevents nodeevents.DB,
	overlayConfig Config,
	placementRules PlacementRules,
) (*Service, error) {

	service, err := NewService(log.Named("overlay"),
		overlayDB,
		nodevents,
		placementRules,
		overlayConfig)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				err = service.Run(ctx)
				if err != nil {
					log.Error("Error on executing service", zap.Error(err))
				}
				err = shutdowner.Shutdown(fx.ExitCode(1))
				if err != nil {
					log.Error("Error on executing service", zap.Error(err))
				}
			}()
			return err
		},
		OnStop: func(ctx context.Context) error {
			return service.Close()
		},
	})
	return service, nil
}

var Module = fx.Module("overlay",
	fx.Provide(createOverlay))
