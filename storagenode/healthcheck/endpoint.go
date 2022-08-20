package healthcheck

import (
	"encoding/json"
	"net/http"
)

type Endpoint struct {
	service *Service
}

func NewEndpoint(service *Service) *Endpoint {
	return &Endpoint{
		service: service,
	}
}

func (e *Endpoint) HandleHTTP(writer http.ResponseWriter, request *http.Request) {
	health, err := e.service.GetHealth(request.Context())
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(err.Error()))
		return
	}

	out, err := json.MarshalIndent(health, "", "  ")
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(err.Error()))
		return
	}

	if health.Healthy {
		writer.WriteHeader(http.StatusOK)
	} else {
		writer.WriteHeader(http.StatusGone)
	}

	_, _ = writer.Write(out)
}
