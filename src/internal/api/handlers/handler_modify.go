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

type ModifyRequest struct {
	DN         string              `json:"dn"`
	Attributes map[string][]string `json:"attributes"`
}

func ModifyHandler(a *app.App, w http.ResponseWriter, r *http.Request) {
	var req ModifyRequest
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

	modReq := ldap.NewModifyRequest(req.DN, nil)
	for attr, vals := range req.Attributes {
		modReq.Replace(attr, vals)
	}

	if err := conn.Modify(modReq); err != nil {
		log.Printf("❌ LDAP Modify failed for %s: %v", req.DN, err)
		http.Error(w, err.Error(), 500)
		return
	}

	app.JsonResponse(w, map[string]string{"message": fmt.Sprintf("Modified %s successfully", req.DN)})
}
