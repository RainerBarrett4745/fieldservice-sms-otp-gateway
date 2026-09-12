package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
)

type server struct{ dispatch *DispatchLogin }

type startRequest struct {
	RequestID   string   `json:"request_id"`
	WorkOrderID string   `json:"work_order_id"`
	Phone       string   `json:"phone"`
	PhotoURLs   []string `json:"photo_urls"`
	FollowUp    string   `json:"technician_follow_up"`
}

type verifyRequest struct {
	Phone string `json:"phone"`
	Code  string `json:"code"`
}

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	s := &server{dispatch: NewDispatchLogin(NewSMSClient(key))}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /login/code", s.start)
	mux.HandleFunc("POST /login/verify", s.verify)
	log.Println("field-service login listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func (s *server) start(w http.ResponseWriter, r *http.Request) {
	var in startRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	login := WorkOrderLogin{WorkOrderID: in.WorkOrderID, Phone: in.Phone, PhotoURLs: in.PhotoURLs, FollowUp: in.FollowUp}
	if err := s.dispatch.Start(r.Context(), login, in.RequestID); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"dispatch_status": "code_sent"})
}

func (s *server) verify(w http.ResponseWriter, r *http.Request) {
	var in verifyRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	login, err := s.dispatch.Verify(r.Context(), in.Phone, in.Code)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, login)
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	var apiErr *InfraiError
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		status = apiErr.HTTPStatus
	} else if !errors.As(err, &apiErr) {
		status = http.StatusBadRequest
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
