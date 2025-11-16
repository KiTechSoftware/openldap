package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/kitechsoftware/ldappy/internal/common/config"
)

// 🔹 App struct holds context and config
type App struct {
	Ctx    context.Context
	Cancel context.CancelFunc
	Cfg    *config.Config
	Mux    *http.ServeMux
	Server *http.Server
}

// 🔹 AppHandlerFunc allows access to *App in handlers
type AppHandlerFunc func(*App, http.ResponseWriter, *http.Request)
type MiddlewareFunc func(AppHandlerFunc) AppHandlerFunc

// 🔹 NewApp creates a new App instance
func NewApp(ctx context.Context, cfg *config.Config) *App {
	ctx, cancel := context.WithCancel(ctx)

	app := &App{
		Ctx:    ctx,
		Cancel: cancel,
		Cfg:    cfg,
		Mux:    http.NewServeMux(),
	}

	app.Server = &http.Server{
		Addr:    cfg.API.Address(), // e.g. "0.0.0.0:8080"
		Handler: app.Mux,
	}

	return app
}

// 🔹 Connect to LDAP using app config
func (a *App) LdapConnect() (*ldap.Conn, string, error) {
	ldapURL := a.Cfg.LDAPURL()

	bindDN := fmt.Sprintf("cn=%s,%s", a.Cfg.LDAP.AdminUser, a.Cfg.LDAP.BaseDN)
	bindPassword := a.Cfg.LDAP.AdminPassword
	baseDN := a.Cfg.LDAP.BaseDN

	conn, err := ldap.DialURL(ldapURL)
	if err != nil {
		return nil, baseDN, fmt.Errorf("connection failed: %w", err)
	}

	if err := conn.Bind(bindDN, bindPassword); err != nil {
		conn.Close()
		return nil, baseDN, fmt.Errorf("bind failed: %w", err)
	}

	log.Printf("✅ Connected to %s as %s", ldapURL, bindDN)
	return conn, baseDN, nil // caller must close conn
}

// 🔹 AddRoute binds App-aware handlers
func (a *App) AddRoute(path string, handler AppHandlerFunc) {
	a.Mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		handler(a, w, r)
	})
}

// 🔹 Start the HTTP server with graceful shutdown
func (a *App) Start() error {
	// Handle OS interrupts (Ctrl+C, SIGTERM, etc.)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		log.Println("🛑 Received shutdown signal...")
		a.Cancel()
	}()

	go func() {
		log.Printf("🚀 LDAP API running on %s", a.Server.Addr)
		if err := a.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server error: %v", err)
		}
	}()

	<-a.Ctx.Done() // Block until Cancel() is called

	return a.Shutdown()
}

// 🔹 Graceful shutdown logic
func (a *App) Shutdown() error {
	log.Println("🔻 Shutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.Server.Shutdown(ctx); err != nil {
		log.Printf("⚠️ Error during shutdown: %v", err)
		return err
	}

	log.Println("✅ Server stopped cleanly")
	return nil
}

// 🔹 JSON response helper with error safety
func JsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("❌ JSON encode error: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
	}
}
