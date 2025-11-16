package handlers

import (
	"net/http"

	"github.com/kitechsoftware/ldappy/internal/api/app"
)

func PingHandler(a *app.App, w http.ResponseWriter, r *http.Request) {
	conn, _, err := a.LdapConnect()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	app.JsonResponse(w, map[string]string{"status": "ok"})
}
