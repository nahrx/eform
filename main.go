package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nahrx/eform/internal/auth"
	"github.com/nahrx/eform/internal/config"
	"github.com/nahrx/eform/internal/db"
	"github.com/nahrx/eform/internal/httpapi"
	"github.com/nahrx/eform/internal/store"
	migrations "github.com/nahrx/eform/migrations"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("DB: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	st := store.New(pool)
	am := auth.NewManager(cfg.JWTSecret, cfg.JWTRespondentSecret, cfg.JWTTTL)

	if err := seedSuperadmin(ctx, st, cfg.Seed); err != nil {
		log.Fatalf("seed superadmin: %v", err)
	}

	srv := httpapi.New(cfg, st, am)
	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("eForm backend listening on http://localhost:%s  (landing: /, admin: /admin, builder: /builder)", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	// Periodic log pruning. Without it activity_logs and api_access_logs grow
	// forever — a single integration polling once a minute already adds
	// hundreds of thousands of rows per year.
	pruneCtx, stopPrune := context.WithCancel(context.Background())
	defer stopPrune()
	go runLogPruner(pruneCtx, st, cfg.LogRetentionDays)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

// runLogPruner prunes old logs once at startup and then every 24 hours.
func runLogPruner(ctx context.Context, st *store.Store, days int) {
	if days <= 0 {
		return
	}
	prune := func() {
		c, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		act, api, err := st.PruneLogs(c, days)
		if err != nil {
			log.Printf("[prune] failed to prune logs: %v", err)
			return
		}
		if act > 0 || api > 0 {
			log.Printf("[prune] logs older than %d days deleted: %d activity, %d API access", days, act, api)
		}

		// Queue reports are kept far longer than logs and on a fixed window rather than
		// LOG_RETENTION_DAYS. A stale report is not evidence that the backlog was
		// recovered — only that the device went quiet — so the row is the last trace
		// that work was stranded, and dropping it early destroys the evidence instead
		// of the problem.
		const queueReportRetention = 180 * 24 * time.Hour
		if n, err := st.PruneOfflineQueueReports(c, queueReportRetention); err != nil {
			log.Printf("[prune] failed to prune queue reports: %v", err)
		} else if n > 0 {
			log.Printf("[prune] queue reports untouched for 180 days deleted: %d", n)
		}
	}
	prune()

	t := time.NewTicker(24 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			prune()
		}
	}
}

// seedSuperadmin creates the first superadmin user if the users table is still empty.
func seedSuperadmin(ctx context.Context, st *store.Store, sc config.SeedConfig) error {
	n, err := st.CountUsers(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	hash, err := auth.HashPassword(sc.Password)
	if err != nil {
		return err
	}
	if _, err := st.CreateUser(ctx, sc.Username, sc.Email, hash, "superadmin", ""); err != nil {
		return err
	}
	log.Println("============================================================")
	log.Println(" SUPER ADMIN created (first login):")
	log.Printf("   username : %s", sc.Username)
	log.Printf("   password : %s", sc.Password)
	log.Println(" >> CHANGE this password immediately, and set SUPERADMIN_PASSWORD in .env")
	log.Println("============================================================")
	return nil
}
