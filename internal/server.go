package internal

import (
	"encoding/json"
	"log"
	"net/http"
)

func Handler(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string   `json:"username"`
			Paths    []string `json:"paths"`
			Cleanup  bool     `json:"cleanup"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := svc.Run(r.Context(), req.Username, req.Paths, req.Cleanup); err != nil {
			log.Printf("preserve error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func Serve(svc *Service, addr string) error {
	http.HandleFunc("/preserve", Handler(svc))
	log.Printf("listening on %s", addr)
	return http.ListenAndServe(addr, nil)
}
