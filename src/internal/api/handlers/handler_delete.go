package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-ldap/ldap/v3"
	"github.com/kitechsoftware/ldappy/internal/api/app"
)

type DeleteRequest struct {
	DN string `json:"dn"`
}

func DeleteHandler(a *app.App, w http.ResponseWriter, r *http.Request) {
	var req DeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	conn, baseDN, err := a.LdapConnect()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer conn.Close()

	if !strings.HasSuffix(strings.ToLower(req.DN), strings.ToLower(baseDN)) {
		req.DN = fmt.Sprintf("%s,%s", req.DN, baseDN)
	}

	delReq := ldap.NewDelRequest(req.DN, nil)
	if err := conn.Del(delReq); err != nil {
		log.Printf("❌ LDAP Delete failed for %s: %v", req.DN, err)
		http.Error(w, err.Error(), 500)
		return
	}

	app.JsonResponse(w, map[string]string{"message": fmt.Sprintf("Deleted %s successfully", req.DN)})
}
