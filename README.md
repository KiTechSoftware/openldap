# 🧩 **LDAPPY an OpenLDAP Stack**

A self-contained **OpenLDAP management stack** that combines:

* 🧠 A **Go-based REST API** for programmatic access
* ⚙️ A **powerful CLI tool (`ldappy`)** for local and remote configuration
* 🗄️ An embedded **OpenLDAP server (`slapd`)**
* 🔒 Optional LDAP port exposure or secure API-only access

This design lets you run a single image that can act as:

* a **local LDAP directory backend** managed via REST,
* or a **fully exposed LDAP service** with CLI and API interfaces.

---

## 📦 **Features**

✅ Fully containerized OpenLDAP + API + CLI stack
✅ Manage configuration through API or CLI (`ldappy` or `ldappy-api`)
✅ Backup / rollback / restore with LDIF snapshots
✅ Initialize directories and reset admin credentials
✅ Systemd-free `slapd` runtime for containers
✅ Optional external LDAP exposure (`389/tcp`)
✅ Health checks for both API and LDAP layers
✅ Minimal `debian:trixie-slim` runtime image

---

## 🏗️ **Architecture Overview**

```
+----------------------------------------+
|             ldapctl stack              |
|----------------------------------------|
|  CLI (ldapctl)      API Server         |
|    │                    │              |
|    │ via shell cmds     │ via REST     |
|    └──────────┬─────────┘              |
|               │                        |
|           slapd (OpenLDAP)             |
|----------------------------------------|
|   Debian (Trixie) Runtime Base         |
+----------------------------------------+
```

### Components

| Component                | Description                                            |
| ------------------------ | ------------------------------------------------------ |
| **src/cmd/api/**         | Go HTTP service exposing REST endpoints to manage LDAP |
| **src/cmd/cli/**         | Go-based command-line utility (`ldapctl`)              |
| **slapd**                | Embedded OpenLDAP server                               |
| **entrypoint.sh**        | Starts slapd + API, or executes CLI commands           |
| **Dockerfile**           | Multi-stage build for API + CLI + LDAP runtime         |
| **supervisord.template** | Configuration for multi-process container startup      |
