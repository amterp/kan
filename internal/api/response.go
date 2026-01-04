package api

import (
	"encoding/json"
	"errors"
	"net/http"

	kanerr "github.com/amterp/kan/internal/errors"
)

// JSON writes a JSON response with the given status code.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// Error writes an error response, mapping domain errors to HTTP status codes.
func Error(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := err.Error()

	var notFound *kanerr.NotFoundError
	var notInit *kanerr.NotInitializedError
	var alreadyExists *kanerr.AlreadyExistsError
	var validation *kanerr.ValidationError

	switch {
	case errors.As(err, &notFound):
		status = http.StatusNotFound
	case errors.As(err, &notInit):
		status = http.StatusNotFound
		message = "Kan is not initialized in this repository"
	case errors.As(err, &alreadyExists):
		status = http.StatusConflict
	case errors.As(err, &validation):
		status = http.StatusBadRequest
	}

	JSON(w, status, map[string]string{"error": message})
}

// BadRequest writes a 400 error with the given message.
func BadRequest(w http.ResponseWriter, message string) {
	JSON(w, http.StatusBadRequest, map[string]string{"error": message})
}
