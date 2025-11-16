package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/kitechsoftware/ldappy/internal/api/app"
	"github.com/kitechsoftware/ldappy/internal/common/security"
)

type passwordRequest struct {
	Password string `json:"password"`
}

func PasswordHashHandler(a *app.App, w http.ResponseWriter, r *http.Request) {
	conn, _, err := a.LdapConnect()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	var req passwordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "password is required", http.StatusBadRequest)
		return
	}

	hashed, err := security.HashPasswordContext(r.Context(), req.Password)
	if err != nil {
		http.Error(w, "failed to hash password: "+err.Error(), http.StatusInternalServerError)
		return
	}

	app.JsonResponse(w, map[string]string{
		"hashed": hashed,
	})
}

func PasswordResetHandler(a *app.App, w http.ResponseWriter, r *http.Request) {
	conn, _, err := a.LdapConnect()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	app.JsonResponse(w, map[string]string{"status": "ok"})
}
