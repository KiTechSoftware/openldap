Perfect — this makes things much clearer ✅

You’re using a **single LDAP root admin** account (like `cn=admin,dc=example,dc=com`) to manage **multiple domain trees** (`dc=example,dc=com`, `dc=example1,dc=com`, etc.) — all inside the same LDAP server.

That’s a **great setup**: simple, secure, and flexible.

Let’s go step-by-step through how to make your Go code handle this pattern cleanly.

---

## 🧩 1. Your Directory Layout

You want to manage users like:

```
dc=example,dc=com
└── ou=users
    ├── cn=john (mail=john@example.com)

dc=example1,dc=com
└── ou=users
    ├── cn=susie (mail=susie@example1.com)
```

You’ll use the same root admin:

```
cn=admin,dc=example,dc=com
```

to manage both subtrees.

That’s fine — the LDAP admin DN at `dc=example,dc=com` can create and manage sibling `dc=example1,dc=com` entries, **as long as your slapd configuration allows it** (which OpenLDAP does by default if you’re the rootDN).

---

## 🧠 2. Update Your `ldapConnect()` Function

We’ll make `ldapConnect()` smart enough to:

* Always bind with your root admin (`cn=admin,<baseDN>`).
* Allow you to specify a **target domain** at runtime.
* Fall back to the default `baseDN` from config when not provided.

Here’s the clean version you want 👇

```go
func (a *App) ldapConnect(domain string) (*ldap.Conn, string, error) {
	ldapURL := a.cfg.LDAP.URL
	baseDN := a.cfg.LDAP.BaseDN
	bindPassword := a.cfg.LDAP.AdminPassword

	// If a domain was provided, convert it into dc= format
	if domain != "" {
		baseDN = domainToBaseDN(domain)
	}

	bindDN := fmt.Sprintf("cn=admin,%s", baseDN)

	conn, err := ldap.DialURL(ldapURL)
	if err != nil {
		return nil, baseDN, fmt.Errorf("connection failed: %w", err)
	}

	if err := conn.Bind(bindDN, bindPassword); err != nil {
		conn.Close()
		return nil, baseDN, fmt.Errorf("bind failed for %s: %w", bindDN, err)
	}

	log.Printf("✅ Bound to %s as %s", ldapURL, bindDN)
	return conn, baseDN, nil
}
```

---

## 🧩 3. Add a Small Helper

Add this utility function anywhere in your file:

```go
func domainToBaseDN(domain string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(domain)), ".")
	var dcParts []string
	for _, p := range parts {
		dcParts = append(dcParts, fmt.Sprintf("dc=%s", p))
	}
	return strings.Join(dcParts, ",")
}
```

Examples:

```go
domainToBaseDN("example.com")  // => "dc=example,dc=com"
domainToBaseDN("example1.com") // => "dc=example1,dc=com"
```

---

## 🧩 4. Update Your Handlers (Add Domain Support)

You already have `AddRequest`, `ModifyRequest`, etc.
We just add an optional `"domain"` field to them:

```go
type AddRequest struct {
	DN         string              `json:"dn"`
	Attributes map[string][]string `json:"attributes"`
	Domain     string              `json:"domain,omitempty"`
}

type ModifyRequest struct {
	DN         string              `json:"dn"`
	Attributes map[string][]string `json:"attributes"`
	Domain     string              `json:"domain,omitempty"`
}

type DeleteRequest struct {
	DN     string `json:"dn"`
	Domain string `json:"domain,omitempty"`
}

type SearchRequest struct {
	BaseDN   string   `json:"base_dn"`
	Filter   string   `json:"filter"`
	Attrs    []string `json:"attrs"`
	ScopeSub bool     `json:"scope_sub"`
	Domain   string   `json:"domain,omitempty"`
}
```

Now you can pass `"domain": "example1.com"` in any request.

---

## 🧩 5. Use the Domain When Connecting

Here’s how to change your handlers slightly.

Example — **Add Handler**:

```go
conn, baseDN, err := a.ldapConnect(req.Domain)
```

Same for modify/delete/search.

Everything else in your handler remains identical — `ensureDNExists()` and your auto-OU logic still work perfectly.

---

## ✅ Example Add Request

Now you can create a user for any domain with one API:

**Request:**

```json
{
  "dn": "cn=susie,ou=users",
  "domain": "example1.com",
  "attributes": {
    "objectClass": ["inetOrgPerson"],
    "cn": ["susie"],
    "sn": ["Q"],
    "mail": ["susie@example1.com"]
  }
}
```

**Server Logs:**

```
✅ Bound to ldap://localhost:389 as cn=admin,dc=example1,dc=com
🆕 Creating missing parent OU: ou=users,dc=example1,dc=com
✅ Added cn=susie,ou=users,dc=example1,dc=com
```

Now you can also add `john@example.com` under `dc=example,dc=com` with a single call by just changing the domain field.

---

## 🧠 Summary

✅ **You can absolutely manage multiple business domains** in one LDAP instance using your root admin.

| What You Do                             | Why                                                       |
| --------------------------------------- | --------------------------------------------------------- |
| Keep one admin (`cn=admin,<baseDN>`)    | Simple authentication                                     |
| Derive `baseDN` dynamically from domain | Supports multiple domains (`example.com`, `example1.com`) |
| Add `"domain"` field in requests        | Lets clients choose target tenant/domain                  |
| Use same `ensureDNExists()` logic       | Auto-creates the right tree                               |

---

Would you like me to show you **the final unified handler signatures** (`addHandler`, `modifyHandler`, `deleteHandler`, `searchHandler`) all updated to accept `req.Domain` and call `a.ldapConnect(req.Domain)`?
That way, you’ll have a copy-paste complete version that supports all domains out of the box.
