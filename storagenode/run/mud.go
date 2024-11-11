package root

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular"
	"storj.io/storj/storagenode"
)

func CreateModule() *mud.Ball {
	ball := &mud.Ball{}
	mud.Provide[*zap.Logger](ball, func() (*zap.Logger, error) {
		logger, err := zap.NewDevelopment()
		if err != nil {
			return nil, errs.Wrap(err)
		}
		return logger.With(zap.String("Process", "storagenode")), nil
	})
	modular.IdentityModule(ball)
	storagenode.Module(ball)
	return ball
}
