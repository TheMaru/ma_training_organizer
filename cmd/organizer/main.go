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
	"github.com/TheMaru/ma_training_organizer/internal/session"
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
	// Help is answered before the configuration is read, because an environment
	// the binary refuses to start under is exactly when somebody asks for it.
	if len(args) > 0 && isHelpRequest(args[0]) {
		fmt.Println(helpListing())
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if len(args) > 0 {
		cmd, ok := lookupCommand(args[0])
		if !ok {
			return fmt.Errorf("unknown command: %s\nrun: organizer help", args[0])
		}
		return cmd.Run(cfg.DBPath, args[1:])
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

	sessions := session.ForServer(db, session.Policy{
		Lifetime:     cfg.SessionLifetime,
		IdleTimeout:  cfg.SessionIdleTimeout,
		SecureCookie: cfg.Secure,
	})

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
