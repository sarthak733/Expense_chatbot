package response

import (
	"encoding/json"
	"log"
	"net/http"
)

// envelope is the consistent shape every endpoint responds with.
// Agree on this shape with your teammates before writing CRUD handlers —
// it's the API contract the frontend will code against.
type envelope struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// JSON writes a successful response with the given status code and data.
func JSON(w http.ResponseWriter, status int, message string, data interface{}) {
	writeEnvelope(w, status, envelope{Success: true, Message: message, Data: data})
}

// Error writes a failure response with the given status code and message.
// Never leak internal error details (db errors, stack traces) to the client —
// log them server-side and send a safe message here instead.
func Error(w http.ResponseWriter, status int, message string) {
	writeEnvelope(w, status, envelope{Success: false, Message: message})
}

func writeEnvelope(w http.ResponseWriter, status int, e envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(e); err != nil {
		// The status code and headers are already sent at this point,
		// so we can only log — there's nothing left to send to the client.
		log.Printf("response: failed to encode json: %v", err)
	}
}
