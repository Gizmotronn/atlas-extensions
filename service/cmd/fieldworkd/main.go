// Command fieldworkd is Fieldwork's standalone HTTP service. It talks to
// the shared Star Sailors PocketBase instance purely as an API client
// (never a Go import), keeping Fieldwork decoupled from ~/Navigation/backend
// per the project's architecture decision.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/signal-k/fieldwork-service/internal/api"
	"github.com/signal-k/fieldwork-service/internal/schema"
	"github.com/signal-k/fieldwork/pbclient"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	pbURL := getenv("PB_URL", "https://signal-k-starsailors.fly.dev")
	addr := getenv("FIELDWORKD_ADDR", ":8095")

	pb := pbclient.New(pbURL)

	if email, password := os.Getenv("PB_ADMIN_EMAIL"), os.Getenv("PB_ADMIN_PASSWORD"); email != "" && password != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		ensurer, err := schema.NewEnsurer(ctx, pbURL, email, password)
		if err != nil {
			logger.Error("could not authenticate as PocketBase superuser to ensure schema", "error", err)
			os.Exit(1)
		}
		if err := ensurer.EnsureAll(ctx); err != nil {
			logger.Error("could not ensure fieldwork collections exist", "error", err)
			os.Exit(1)
		}
		logger.Info("fieldwork collections ensured")
	} else {
		logger.Warn("PB_ADMIN_EMAIL/PB_ADMIN_PASSWORD not set; skipping schema bootstrap (assuming collections already exist)")
	}

	server := api.NewServer(pb, logger)

	logger.Info("fieldworkd listening", "addr", addr, "pbURL", pbURL)
	if err := http.ListenAndServe(addr, server.Routes()); err != nil {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
