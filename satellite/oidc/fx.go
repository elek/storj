package oidc

import (
	"github.com/gorilla/mux"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"storj.io/common/storj"
	"storj.io/storj/satellite/console/consoleweb"
	"storj.io/storj/satellite/modules"
)

var Module = fx.Module("oidc",
	fx.Provide(
		NewService,
		fx.Annotate(extendConsole, fx.ResultTags(`group:"console"`)),
	))

func init() {
	modules.AutoRegistered = append(modules.AutoRegistered, Module)

}
func extendConsole(logger *zap.Logger, oidcService *Service, nodeURL storj.NodeURL, config consoleweb.Config) consoleweb.ConsoleExtension {
	return func(router *mux.Router, ch mux.MiddlewareFunc, ah mux.MiddlewareFunc) {
		oidc := NewEndpoint(
			nodeURL,
			config.ExternalAddress,
			logger,
			oidcService,
			// TODO
			nil,
			config.OauthCodeExpiry,
			config.OauthAccessTokenExpiry,
			config.OauthRefreshTokenExpiry,
		)

		router.HandleFunc("/api/v0/.well-known/openid-configuration", oidc.WellKnownConfiguration)
		//router.Handle("/api/v0/oauth/v2/authorize", server.withAuth(http.HandlerFunc(oidc.AuthorizeUser))).Methods(http.MethodPost)
		//router.Handle("/api/v0/oauth/v2/tokens", server.ipRateLimiter.Limit(http.HandlerFunc(oidc.Tokens))).Methods(http.MethodPost)
		//router.Handle("/api/v0/oauth/v2/userinfo", server.ipRateLimiter.Limit(http.HandlerFunc(oidc.UserInfo))).Methods(http.MethodGet)
		//router.Handle("/api/v0/oauth/v2/clients/{id}", server.withAuth(http.HandlerFunc(oidc.GetClient))).Methods(http.MethodGet)
	}
}
