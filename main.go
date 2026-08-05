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
		log.Printf("eForm backend jalan di http://localhost:%s  (landing: /, admin: /admin, builder: /builder)", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	// Pemangkasan log berkala. Tanpa ini activity_logs dan api_access_logs tumbuh
	// selamanya — satu integrasi yang menarik data tiap menit saja sudah menambah
	// ratusan ribu baris per tahun.
	pruneCtx, stopPrune := context.WithCancel(context.Background())
	defer stopPrune()
	go runLogPruner(pruneCtx, st, cfg.LogRetentionDays)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("mematikan server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

// runLogPruner memangkas log lama sekali saat start lalu setiap 24 jam.
func runLogPruner(ctx context.Context, st *store.Store, days int) {
	if days <= 0 {
		return
	}
	prune := func() {
		c, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		act, api, err := st.PruneLogs(c, days)
		if err != nil {
			log.Printf("[prune] gagal memangkas log: %v", err)
			return
		}
		if act > 0 || api > 0 {
			log.Printf("[prune] log >%d hari dihapus: %d aktivitas, %d akses API", days, act, api)
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

// seedSuperadmin membuat user superadmin pertama jika tabel users masih kosong.
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
	log.Println(" SUPER ADMIN dibuat (login pertama):")
	log.Printf("   username : %s", sc.Username)
	log.Printf("   password : %s", sc.Password)
	log.Println(" >> GANTI password ini segera, dan set SUPERADMIN_PASSWORD di .env")
	log.Println("============================================================")
	return nil
}
