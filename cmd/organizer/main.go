// Command organizer is the single binary for the martial-arts organizer:
// it runs the HTTP server and (from issue 04) provides account-management
// subcommands.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/TheMaru/ma_training_organizer/internal/config"
	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/web"
)

// errAlreadyReported marks a failure whose explanation the subcommand has
// already printed itself (the import's German abort report), so main exits
// non-zero without tacking a second, redundant message onto it.
var errAlreadyReported = errors.New("already reported")

func main() {
	if err := run(os.Args[1:]); err != nil {
		if !errors.Is(err, errAlreadyReported) {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(args) > 0 {
		switch args[0] {
		case "create-trainer":
			return cmdCreateTrainer(cfg.DBPath, args[1:])
		case "reset-password":
			return cmdResetPassword(cfg.DBPath, args[1:])
		case "revoke-sessions":
			return cmdRevokeSessions(cfg.DBPath, args[1:])
		case "seed-demo":
			return cmdSeedDemo(cfg.DBPath, args[1:])
		case "clear-demo":
			return cmdClearDemo(cfg.DBPath, args[1:])
		case "import-athletes":
			return cmdImportAthletes(cfg.DBPath, args[1:])
		default:
			return fmt.Errorf("unknown command: %s", args[0])
		}
	}

	return serve(cfg)
}

func serve(cfg config.Config) error {
	log.Printf("config: %s", cfg)

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	sessions := web.NewSessionManager(db, cfg.SessionLifetime, cfg.SessionIdleTimeout, cfg.Secure)

	srv, err := web.NewServer(db, sessions)
	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("listening on %s", cfg.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}
