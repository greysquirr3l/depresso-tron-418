package main

import (
	"bufio"
	"context"
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed templates static assets
var embedFS embed.FS

var (
	tmpl        *template.Template
	geminiKey   string
	serverState *AppState
)

// loadDotEnv reads key=value pairs from a .env file into the process environment.
// It is intentionally minimal: no interpolation, no export keyword, no comments
// mid-line. Lines starting with # are ignored. Already-set env vars are not
// overwritten so that shell exports always win.
func loadDotEnv(path string) {
	f, err := os.Open(path) //nolint:gosec // path is always ".env", the caller-controlled literal
	if err != nil {
		return // .env is optional
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key   = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		// Strip optional surrounding quotes.
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		// Never overwrite a value already set in the environment.
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

func main() {
	// Load .env before reading env vars; shell exports always take precedence.
	loadDotEnv(".env")

	// Env var takes priority; DB-persisted key loaded after initDB().
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		geminiKey = key
	}

	if err := initDB(); err != nil {
		log.Fatalf("DB init failed: %v", err)
	}
	defer closeDB()

	// If not set by env, try DB-persisted value.
	if geminiKey == "" {
		if stored, err := getSetting(context.Background(), "gemini_api_key"); err == nil && stored != "" {
			geminiKey = stored
			log.Println("☕  GEMINI_API_KEY loaded from DB")
		} else {
			log.Println("⚠  GEMINI_API_KEY not set — Barista running in offline snark mode")
			log.Println("   Set GEMINI_API_KEY env var OR enter it in the browser at http://localhost:4180")
		}
	}

	serverState = newAppState()
	go serverState.runMoodCycle()

	var err error
	tmpl, err = template.New("").Funcs(template.FuncMap{
		"not": func(b bool) bool { return !b },
	}).ParseFS(embedFS, "templates/*.html")
	if err != nil {
		log.Fatalf("template parse failed: %v", err)
	}

	staticFS, err := fs.Sub(embedFS, "static")
	if err != nil {
		log.Fatalf("static fs sub failed: %v", err)
	}
	assetsFS, err := fs.Sub(embedFS, "assets")
	if err != nil {
		log.Fatalf("assets fs sub failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsFS))))

	mux.HandleFunc("GET /", handleIndex)
	mux.HandleFunc("POST /set-gemini-key", handleSetGeminiKey)
	mux.HandleFunc("GET /gemini-status", handleGeminiStatus)
	mux.HandleFunc("POST /apply-permit", handleApplyPermit)
	mux.HandleFunc("GET /permit-status", handlePermitStatus)
	mux.HandleFunc("POST /check-beans", handleCheckBeans)
	mux.HandleFunc("GET /pow-challenge", handlePowChallenge)
	mux.HandleFunc("POST /pow-verify", handlePowVerify)
	mux.HandleFunc("GET /queue", handleQueue)
	mux.HandleFunc("POST /brew", handleBrew)
	mux.HandleFunc("GET /brew-status", handleBrewStatus)
	mux.HandleFunc("POST /when", handleWhen)
	mux.HandleFunc("GET /teapot-state", handleTeapotState)
	mux.HandleFunc("POST /manager", handleManager)

	port := "4180" // 418 ∪ 80
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	log.Printf("☕  DEPRESSO-TRON 418 :: RFC 2324 COMPLIANT :: http://localhost:%s", port) //nolint:gosec,misspell // port from env; DEPRESSO is the project name
	srv := &http.Server{ //nolint:gosec // timeouts omitted intentionally for SSE long-poll handlers
		Addr:    ":" + port,
		Handler: mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
