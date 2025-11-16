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

type AddRequest struct {
	DN         string              `json:"dn"`
	Attributes map[string][]string `json:"attributes"`
}

func AddHandler(a *app.App, w http.ResponseWriter, r *http.Request) {
	var req AddRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	conn, baseDN, err := a.LdapConnect()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	// Ensure DN includes baseDN
	if !strings.HasSuffix(strings.ToLower(req.DN), strings.ToLower(baseDN)) {
		req.DN = fmt.Sprintf("%s,%s", req.DN, baseDN)
	}

	parentDN := getParentDN(req.DN)

	// Ensure parent exists
	if err := ensureDNExists(conn, parentDN); err != nil {
		log.Printf("⚠️ Failed to ensure parent DN %s exists: %v", parentDN, err)
		http.Error(w, err.Error(), 500)
		return
	}

	addReq := ldap.NewAddRequest(req.DN, nil)
	for attr, vals := range req.Attributes {
		addReq.Attribute(attr, vals)
	}

	if err := conn.Add(addReq); err != nil {
		log.Printf("❌ LDAP Add failed for %s: %v", req.DN, err)
		http.Error(w, err.Error(), 500)
		return
	}

	app.JsonResponse(w, map[string]string{"message": fmt.Sprintf("Added %s successfully", req.DN)})
}

// getParentDN returns the parent DN of a DN string
func getParentDN(dn string) string {
	parts := strings.SplitN(dn, ",", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}

// ensureDNExists checks if a DN exists, and if not, creates it as an OU
func ensureDNExists(conn *ldap.Conn, dn string) error {
	if dn == "" {
		return fmt.Errorf("invalid DN: empty parent")
	}

	searchReq := ldap.NewSearchRequest(
		dn,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases,
		1, 0, false,
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	)

	_, err := conn.Search(searchReq)
	if err == nil {
		// parent exists
		return nil
	}

	ldapErr, ok := err.(*ldap.Error)
	if !ok || ldapErr.ResultCode != ldap.LDAPResultNoSuchObject {
		// unexpected error
		return err
	}

	// Parent doesn't exist, recursively ensure its parent exists first
	parent := getParentDN(dn)
	if parent != "" {
		if err := ensureDNExists(conn, parent); err != nil {
			return err
		}
	}

	// Create the missing OU
	addReq := ldap.NewAddRequest(dn, nil)
	addReq.Attribute("objectClass", []string{"top", "organizationalUnit"})

	if strings.HasPrefix(strings.ToLower(dn), "ou=") {
		ouName := strings.TrimPrefix(strings.SplitN(dn, ",", 2)[0], "ou=")
		addReq.Attribute("ou", []string{ouName})
	}

	log.Printf("🆕 Creating missing parent OU: %s", dn)
	return conn.Add(addReq)
}
