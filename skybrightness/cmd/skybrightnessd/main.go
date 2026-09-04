// Command skybrightnessd is the skybrightness service: a poll loop, not an
// HTTP server like fieldworkd, since it has no client-facing request to
// answer -- Atlas's own Journal capture flow already wrote the submission
// row (see signal-k/atlas's CaptureSheet.tsx); this only needs to notice it
// and fill in the sky_brightness_* fields the client left empty. Talks to
// the shared PocketBase instance purely as an API client, same architecture
// decision as Fieldwork (see ../../README.md).
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/signal-k/skybrightness/internal/processor"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	pbURL := getenv("PB_URL", "https://signal-k-starsailors.fly.dev")
	pollInterval := 60 * time.Second

	email, password := os.Getenv("PB_ADMIN_EMAIL"), os.Getenv("PB_ADMIN_PASSWORD")
	if email == "" || password == "" {
		logger.Error("PB_ADMIN_EMAIL/PB_ADMIN_PASSWORD are required -- this service needs superuser access to read and write every user's atlas_observations rows")
		os.Exit(1)
	}

	ctx := context.Background()
	authCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	proc, err := processor.New(authCtx, pbURL, email, password)
	cancel()
	if err != nil {
		logger.Error("could not start processor", "error", err)
		os.Exit(1)
	}

	logger.Info("skybrightnessd started", "pbURL", pbURL, "pollInterval", pollInterval)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	runOnce(ctx, proc, logger)
	for range ticker.C {
		runOnce(ctx, proc, logger)
	}
}

func runOnce(ctx context.Context, proc *processor.Processor, logger *slog.Logger) {
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	count, err := proc.RunOnce(runCtx)
	if err != nil {
		logger.Error("poll failed", "error", err)
		return
	}
	if count > 0 {
		logger.Info("processed submissions", "count", count)
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
