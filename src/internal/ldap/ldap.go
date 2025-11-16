package ldap

import (
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

// UserExists checks if a given username exists in LDAP
func UserExists(conn *ldap.Conn, username string) (bool, error) {
	searchRequest := ldap.NewSearchRequest(
		"dc=example,dc=com", // Base DN (could be from config)
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(uid=%s)", ldap.EscapeFilter(username)),
		[]string{"dn"},
		nil,
	)

	res, err := conn.Search(searchRequest)
	if err != nil {
		return false, fmt.Errorf("LDAP search failed: %w", err)
	}

	return len(res.Entries) > 0, nil
}

// func ResetUserPassword(ctx context.Context, conn *ldap.Conn, username, newPass string, jsonOutput bool) *ConfigReport {
// 	start := time.Now()
// 	report := &ConfigReport{Action: "reset_user_password", Timestamp: start}

// 	if newPass == "" {
// 		report.ErrorMsg = "password cannot be empty"
// 		return report
// 	}

// 	hashed, err := security.HashPasswordContext(ctx, newPass)
// 	if err != nil {
// 		report.ErrorMsg = fmt.Sprintf("failed to hash password: %v", err)
// 		return report
// 	}

// 	modifyReq := ldap.NewModifyRequest(fmt.Sprintf("uid=%s,ou=people,dc=example,dc=com", username), nil)
// 	modifyReq.Replace("userPassword", []string{hashed})

// 	if err := conn.Modify(modifyReq); err != nil {
// 		report.ErrorMsg = fmt.Sprintf("LDAP modify failed: %v", err)
// 	} else {
// 		report.Success = true
// 		report.Message = "User password reset successfully"
// 	}

// 	report.Duration = time.Since(start).String()
// 	return report
// }
