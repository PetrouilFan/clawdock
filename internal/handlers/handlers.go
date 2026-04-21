package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"clawdock/internal/config"
	"clawdock/internal/docker"
	"clawdock/internal/providers"
	"clawdock/internal/terminal"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Handler struct {
	cfg              *config.Config
	db               *sql.DB
	docker           *docker.Client
	term             *terminal.Terminal
	providerRegistry *providers.Registry // to be added later
}

func New(cfg *config.Config, db *sql.DB, docker *docker.Client) *Handler {
	// Initialize provider registry with encryption secret
	secret := []byte(cfg.Security.SecretKey)
	reg := providers.NewRegistry(db, secret)

	return &Handler{
		cfg:              cfg,
		db:               db,
		docker:           docker,
		term:             terminal.New(docker),
		providerRegistry: reg,
	}
}

func (h *Handler) SetupRoutes() *mux.Router {
	r := mux.NewRouter()

	// Security middleware
	r.Use(SecurityHeaders)
	r.Use(RequestIDMiddleware)

	// Rate limiting
	rl := NewRateLimiter(100, time.Minute)

	// API routes with rate limiting
	api := r.PathPrefix("/api").Subrouter()
	api.Use(rl.Middleware)

	api.HandleFunc("/agents", h.ListAgents).Methods("GET")
	api.HandleFunc("/agents", h.CreateAgent).Methods("POST")
	api.HandleFunc("/agents/{id}", h.GetAgent).Methods("GET")
	api.HandleFunc("/agents/{id}", h.UpdateAgent).Methods("PATCH")
	api.HandleFunc("/agents/{id}", h.DeleteAgent).Methods("DELETE")
	api.HandleFunc("/agents/{id}/start", h.StartAgent).Methods("POST")
	api.HandleFunc("/agents/{id}/stop", h.StopAgent).Methods("POST")
	api.HandleFunc("/agents/{id}/restart", h.RestartAgent).Methods("POST")
	api.HandleFunc("/agents/{id}/recreate", h.RecreateAgent).Methods("POST")
	api.HandleFunc("/agents/{id}/clone", h.CloneAgent).Methods("POST")
	api.HandleFunc("/agents/{id}/logs", h.GetLogs).Methods("GET")
	api.HandleFunc("/agents/{id}/repair", h.RepairAgent).Methods("POST")
	api.HandleFunc("/agents/{id}/backup", h.BackupAgent).Methods("POST")
	api.HandleFunc("/agents/{id}/restore", h.RestoreAgent).Methods("POST")
	api.HandleFunc("/agents/{id}/workspace/download", h.DownloadWorkspace).Methods("GET")
	api.HandleFunc("/agents/{id}/terminal", h.TerminalWebSocket)

	// Providers
	api.HandleFunc("/providers", h.ListProviders).Methods("GET")
	api.HandleFunc("/providers", h.CreateProvider).Methods("POST")
	api.HandleFunc("/providers/{id}", h.GetProvider).Methods("GET")
	api.HandleFunc("/providers/{id}", h.UpdateProvider).Methods("PATCH")
	api.HandleFunc("/providers/{id}", h.DeleteProvider).Methods("DELETE")
	api.HandleFunc("/providers/{id}/refresh-models", h.RefreshProviderModels).Methods("POST")

	// Provider Models (optional listing)
	api.HandleFunc("/provider-models", h.ListProviderModels).Methods("GET")
	api.HandleFunc("/provider-models/{id}", h.UpdateProviderModel).Methods("PATCH")

	// Model status
	api.HandleFunc("/models/status", h.GetModelStatus).Methods("GET")

	// Custom Models
	api.HandleFunc("/custom-models", h.ListCustomModels).Methods("GET")
	api.HandleFunc("/custom-models", h.CreateCustomModel).Methods("POST")
	api.HandleFunc("/custom-models/{alias}", h.GetCustomModel).Methods("GET")
	api.HandleFunc("/custom-models/{alias}", h.UpdateCustomModel).Methods("PATCH")
	api.HandleFunc("/custom-models/{alias}", h.DeleteCustomModel).Methods("DELETE")

	// Settings
	api.HandleFunc("/settings/default_model", h.GetDefaultModel).Methods("GET")
	api.HandleFunc("/settings/default_model", h.SetDefaultModel).Methods("PUT")
	api.HandleFunc("/settings/chat_proxy_enabled", h.GetChatProxyEnabled).Methods("GET")
	api.HandleFunc("/settings/chat_proxy_enabled", h.SetChatProxyEnabled).Methods("PUT")

	// Existing routes...
	api.HandleFunc("/validate/path", h.ValidatePath).Methods("POST")
	api.HandleFunc("/validate/token", h.ValidateToken).Methods("POST")
	api.HandleFunc("/reconcile", h.TriggerReconcile).Methods("POST")
	api.HandleFunc("/audit", h.GetAuditLog).Methods("GET")
	api.HandleFunc("/version", h.Version).Methods("GET")

	// Health endpoints
	r.HandleFunc("/healthz", h.Healthz)
	r.HandleFunc("/readyz", h.Readyz)
	r.HandleFunc("/version", h.Version).Methods("GET")

	// OpenAI-compatible chat proxy (with separate rate limiting)
	chatRL := NewRateLimiter(60, time.Minute) // 60 requests/min for proxy
	v1 := r.PathPrefix("/v1").Subrouter()
	v1.Use(chatRL.Middleware)
	v1.HandleFunc("/models", h.ListOpenAICompatibleModels).Methods("GET")
	v1.HandleFunc("/chat/completions", h.ChatCompletions).Methods("POST")

	// Static files - serve from directory relative to executable
	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir(getStaticDir()))))

	return r
}

func getStaticDir() string {
	// Always use the directory relative to the executable
	execPath, err := os.Executable()
	if err == nil {
		staticDir := filepath.Join(filepath.Dir(execPath), "web", "static")
		if _, err := os.Stat(staticDir); err == nil {
			return staticDir
		}
	}
	// Fallback: /var/lib for packaged installs
	if _, err := os.Stat("/var/lib/openclaw-manager/web/static"); err == nil {
		return "/var/lib/openclaw-manager/web/static"
	}
	// Fallback to embedded web/static for dev
	return "web/static"
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	if err := h.db.Ping(); err != nil {
		http.Error(w, "database unreachable", http.StatusServiceUnavailable)
		return
	}
	if err := h.docker.Ping(); err != nil {
		http.Error(w, "docker unreachable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("openclaw-manager dev"))
}

func (h *Handler) TerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	agentID := vars["id"]

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade failed", http.StatusInternalServerError)
		return
	}

	if err := h.term.Handle(agentID, conn); err != nil {
		conn.Close()
	}
}
