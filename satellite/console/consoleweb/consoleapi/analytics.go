// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/web"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
)

// ErrAnalyticsAPI - console analytics api error type.
var ErrAnalyticsAPI = errs.Class("consoleapi analytics")

// Analytics is an api controller that exposes analytics related functionality.
type Analytics struct {
	log       *zap.Logger
	service   *console.Service
	analytics *analytics.Service
}

// NewAnalytics is a constructor for api analytics controller.
func NewAnalytics(log *zap.Logger, service *console.Service, a *analytics.Service) *Analytics {
	return &Analytics{
		log:       log,
		service:   service,
		analytics: a,
	}
}

type eventTriggeredBody struct {
	EventName        string            `json:"eventName"`
	Link             string            `json:"link"`
	ErrorEventSource string            `json:"errorEventSource"`
	UIType           string            `json:"uiType"`
	Props            map[string]string `json:"props"`
}

type pageVisitBody struct {
	PageName string `json:"pageName"`
}

// EventTriggered tracks the occurrence of an arbitrary event on the client.
func (a *Analytics) EventTriggered(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
	var et eventTriggeredBody
	err = json.Unmarshal(body, &et)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}

	user, err := console.GetUser(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		return
	}

	if et.ErrorEventSource != "" {
		a.analytics.TrackErrorEvent(user.ID, user.Email, et.ErrorEventSource, et.UIType)
	} else if et.Link != "" {
		a.analytics.TrackLinkEvent(et.EventName, user.ID, user.Email, et.Link, et.UIType)
	} else {
		a.analytics.TrackEvent(et.EventName, user.ID, user.Email, et.UIType, et.Props)
	}
	w.WriteHeader(http.StatusOK)
}

// PageEventTriggered tracks the occurrence of an arbitrary page visit event on the client.
func (a *Analytics) PageEventTriggered(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
	var pv pageVisitBody
	err = json.Unmarshal(body, &pv)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}

	user, err := console.GetUser(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		return
	}

	a.analytics.PageVisitEvent(pv.PageName, user.ID, user.Email)

	w.WriteHeader(http.StatusOK)
}

// serveJSONError writes JSON error to response output stream.
func (a *Analytics) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, a.log, w, status, err)
}
