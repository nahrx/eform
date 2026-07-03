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

	"github.com/bpskaltim/eform-backend/internal/auth"
	"github.com/bpskaltim/eform-backend/internal/config"
	"github.com/bpskaltim/eform-backend/internal/db"
	"github.com/bpskaltim/eform-backend/internal/httpapi"
	"github.com/bpskaltim/eform-backend/internal/store"
	migrations "github.com/bpskaltim/eform-backend/migrations"
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
	am := auth.NewManager(cfg.JWTSecret, cfg.JWTTTL)

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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("mematikan server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
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
