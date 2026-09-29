package httpx

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const contentTypeJSON = "application/json; charset=utf-8"

type dataBody struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

func WriteData(w http.ResponseWriter, status int, data, meta any) error {
	body, err := json.Marshal(dataBody{Data: data, Meta: meta})
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}

	writeJSON(w, status, body)
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
