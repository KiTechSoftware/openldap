package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-ldap/ldap/v3"
	"github.com/kitechsoftware/ldappy/internal/api/app"
)

type SearchRequest struct {
	BaseDN   string   `json:"base_dn"`
	Filter   string   `json:"filter"`
	Attrs    []string `json:"attrs"`
	ScopeSub bool     `json:"scope_sub"`
}

func SearchHandler(a *app.App, w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	conn, _, err := a.LdapConnect()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer conn.Close()

	scope := ldap.ScopeSingleLevel
	if req.ScopeSub {
		scope = ldap.ScopeWholeSubtree
	}

	searchReq := ldap.NewSearchRequest(
		req.BaseDN,
		scope, ldap.NeverDerefAliases,
		0, 0, false,
		req.Filter,
		req.Attrs,
		nil,
	)

	sr, err := conn.Search(searchReq)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	results := []map[string]interface{}{}
	for _, entry := range sr.Entries {
		obj := map[string]interface{}{"dn": entry.DN}
		for _, attr := range entry.Attributes {
			obj[attr.Name] = attr.Values
		}
		results = append(results, obj)
	}

	app.JsonResponse(w, results)
}
