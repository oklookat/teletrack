package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func writeJSON(
	w http.ResponseWriter,
	status int,
	value any,
) {
	data, err := json.Marshal(value)
	if err != nil {
		http.Error(
			w,
			`{"error":"failed to encode response"}`,
			http.StatusInternalServerError,
		)
		return
	}

	w.Header().Set(
		"Content-Type",
		"application/json; charset=utf-8",
	)

	w.Header().Set(
		"Content-Length",
		fmt.Sprintf("%d", len(data)),
	)

	w.WriteHeader(status)

	_, _ = w.Write(data)
}

func writeMethodNotAllowed(
	w http.ResponseWriter,
	method string,
) {
	w.Header().Set("Allow", method)

	writeJSON(
		w,
		http.StatusMethodNotAllowed,
		ErrorResponse{
			Error: "method not allowed",
		},
	)
}

func writeSSE(
	w http.ResponseWriter,
	flusher http.Flusher,
	event []byte,
) bool {
	if _, err := w.Write(event); err != nil {
		return false
	}

	flusher.Flush()

	return true
}

func encodeSSEEvent(
	eventName string,
	data []byte,
) []byte {
	result := make(
		[]byte,
		0,
		len(eventName)+len(data)+32,
	)

	result = append(result, "event: "...)
	result = append(result, eventName...)
	result = append(result, '\n')

	result = append(result, "data: "...)
	result = append(result, data...)
	result = append(result, '\n')
	result = append(result, '\n')

	return result
}
