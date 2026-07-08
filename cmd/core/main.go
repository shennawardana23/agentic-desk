package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/shennawardana23/agentic-desk/internal/api"
	"github.com/shennawardana23/agentic-desk/internal/app/database"
	chatpg "github.com/shennawardana23/agentic-desk/internal/chat/postgres"
	"github.com/shennawardana23/agentic-desk/internal/config"
	"github.com/shennawardana23/agentic-desk/internal/embedding"
	graphpg "github.com/shennawardana23/agentic-desk/internal/graph/postgres"
	"github.com/shennawardana23/agentic-desk/internal/library"
	"github.com/shennawardana23/agentic-desk/internal/orchestrator"
	"github.com/shennawardana23/agentic-desk/internal/secondbrain/postgres"
	taskpg "github.com/shennawardana23/agentic-desk/internal/task/postgres"
	"github.com/shennawardana23/agentic-desk/internal/voicelive"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Println("config loaded")

	ctx := context.Background()
	pool, applied, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()
	log.Printf("migrations applied: %d", applied)

	promptDir := resolveDir("prompts")
	g, _ := orchestrator.Init(ctx, cfg.GeminiAPIKey, promptDir)
	chatFlows := orchestrator.DefineChatFlows(g)
	deps := api.Deps{
		Store:       postgres.NewStore(pool),
		Embedder:    embedding.NewGenkitEmbedder(g),
		Hub:         api.NewHub(),
		Chat:        chatFlows.Chat,
		ChatStream:  chatFlows.Stream,
		Tasks:       taskpg.NewStore(pool),
		ChatHistory: chatpg.NewStore(pool),
		Graph:       graphpg.NewBuilder(pool),
		Library:     &library.Library{SkillsDir: resolveDir("skills"), PromptsDir: promptDir},
		VoiceLive:   voicelive.NewBridge(cfg.GeminiAPIKey),
	}

	log.Printf("api listening on %s", cfg.APIAddr)
	if err := http.ListenAndServe(cfg.APIAddr, api.NewRouter(deps)); err != nil {
		log.Fatalf("api server: %v", err)
	}
}

// resolveDir finds a repo-root data directory (prompts/, skills/) whether
// cmd/core is run the usual way (go run/./bin/core from the repo root,
// where the relative name resolves as-is) or launched by cmd/desktop as a
// child process from inside a packaged .app bundle (see
// cmd/desktop/corelauncher.go), where CWD is the bundle's MacOS/ dir and
// the directory instead ships as a sibling there (copied in by the
// Makefile's desktop-build target).
func resolveDir(name string) string {
	if _, err := os.Stat(name); err == nil {
		return name
	}
	if exe, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(sibling); err == nil {
			return sibling
		}
	}
	return name
}
