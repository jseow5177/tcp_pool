package util

import (
	"encoding/json"
	"net/http"
)

type Envelope map[string]interface{}

// WriteJSON() returns a JSON response from HTTP server back to the client
func WriteJSON(w http.ResponseWriter, status int, data Envelope, headers http.Header) error {
	js, err := json.Marshal(data)
	if err != nil {
		return err
	}

	for key, value := range headers {
		w.Header()[key] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(js)

	return nil
}

// ReadJSON() reads JSON data from a HTTP request
func ReadJSON(r *http.Request, dst interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	err := dec.Decode(dst)
	return err
}

func ServerErrorResponse(w http.ResponseWriter, err error) {
	message := "the server encountered a problem and could not process your request"
	errorResponse(w, http.StatusInternalServerError, message)
}

func errorResponse(w http.ResponseWriter, status int, message interface{}) {
	errData := Envelope{"error": message}

	err := WriteJSON(w, status, errData, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
