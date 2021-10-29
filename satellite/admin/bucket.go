// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

func validateGeofencePathParameters(w http.ResponseWriter, r *http.Request) (project uuid.UUID, bucket []byte, ok bool) {
	vars := mux.Vars(r)

	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing", "", http.StatusBadRequest)
		return
	}

	var err error

	project, err = uuid.FromString(projectUUIDString)
	ok = err == nil
	if !ok {
		sendJSONError(w, "project-uuid not a valid uuid", err.Error(), http.StatusBadRequest)
		return
	}

	bucketName, ok := vars["bucket"]
	if !ok {
		sendJSONError(w, "bucket name missing", "", http.StatusBadRequest)
		return
	}

	bucket = []byte(bucketName)
	return
}

func (server *Server) validateProjectAndBucket(w http.ResponseWriter, r *http.Request, project uuid.UUID, bucket []byte) (b storj.Bucket, ok bool) {
	ctx := r.Context()

	b, err := server.db.Buckets().GetBucket(ctx, bucket, project)
	ok = err == nil
	if !ok {
		if storj.ErrBucketNotFound.Has(err) {
			sendJSONError(w, "bucket does not exist", "", http.StatusBadRequest)
		} else {
			sendJSONError(w, "unable to check bucket", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	ok, err = server.metabaseDB.BucketEmpty(ctx, metabase.BucketEmpty{
		ProjectID:  project,
		BucketName: string(bucket),
	})

	if err != nil {
		sendJSONError(w, "unable to check bucket status", err.Error(), http.StatusInternalServerError)
	} else if !ok {
		sendJSONError(w, "bucket must be empty", "", http.StatusBadRequest)
	}

	return b, ok
}

func (server *Server) createGeofenceForBucket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	project, bucket, ok := validateGeofencePathParameters(w, r)
	if !ok {
		return
	}

	regionCode := r.URL.Query().Get("region")
	if regionCode == "" {
		sendJSONError(w, "missing region query parameter", "", http.StatusBadRequest)
		return
	}

	placement := storj.EveryCountry
	switch regionCode {
	case "EU":
		placement = storj.EU
	case "EEA":
		placement = storj.EEA
	case "US":
		placement = storj.US
	case "DE":
		placement = storj.DE
	default:
		sendJSONError(w, "unrecognized region code", "available: EU, EEA, US, DE", http.StatusBadRequest)
	}

	b, ok := server.validateProjectAndBucket(w, r, project, bucket)
	if !ok {
		return
	}

	b.Placement = placement

	_, err := server.db.Buckets().UpdateBucket(ctx, b)
	if err != nil {
		sendJSONError(w, "failed to update bucket", err.Error(), http.StatusBadRequest)
		return
	}

	data, err := json.Marshal(b)
	if err != nil {
		sendJSONError(w, "json encoding failed", err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) deleteGeofenceForBucket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	project, bucket, ok := validateGeofencePathParameters(w, r)
	if !ok {
		return
	}

	b, ok := server.validateProjectAndBucket(w, r, project, bucket)
	if !ok {
		return
	}

	b.Placement = storj.EveryCountry

	b, err := server.db.Buckets().UpdateBucket(ctx, b)
	if err != nil {
		sendJSONError(w, "failed to update bucket", err.Error(), http.StatusBadRequest)
		return
	}

	data, err := json.Marshal(b)
	if err != nil {
		sendJSONError(w, "json encoding failed", err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) checkGeofenceForBucket(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	project, bucket, ok := validateGeofencePathParameters(w, r)
	if !ok {
		return
	}

	b, err := server.db.Buckets().GetBucket(ctx, bucket, project)
	if err != nil {
		if storj.ErrBucketNotFound.Has(err) {
			sendJSONError(w, "bucket does not exist", "", http.StatusBadRequest)
		} else {
			sendJSONError(w, "unable to check bucket", err.Error(), http.StatusInternalServerError)
		}
		return
	}

	data, err := json.Marshal(b)
	if err != nil {
		sendJSONError(w, "json encoding failed", err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}
