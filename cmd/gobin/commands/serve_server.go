package commands

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mengbin92/gobin/internal/config"
)

type serveServer struct {
	addr         string
	newServer    func(string, *config.Config, serveRuntime) devServer
	signals      chan os.Signal
	runLifecycle func(io.Writer, devServer, string, <-chan os.Signal) error
}

func newServeServer(port int) serveServer {
	return serveServer{
		addr: fmt.Sprintf(":%d", port),
		newServer: func(addr string, cfg *config.Config, runtime serveRuntime) devServer {
			return newDevServer(addr, cfg.PublishDir, runtime.liveReload)
		},
		signals: make(chan os.Signal, 1),
		runLifecycle: func(stdout io.Writer, server devServer, addr string, signals <-chan os.Signal) error {
			return runServerLifecycle(stdout, server, addr, signals)
		},
	}
}

func (s serveServer) run(stdout io.Writer, cfg *config.Config) error {
	return s.runWithRuntime(stdout, cfg, serveRuntime{})
}

func (s serveServer) runWithRuntime(stdout io.Writer, cfg *config.Config, runtime serveRuntime) error {
	server := s.newServer(s.addr, cfg, runtime)
	signal.Notify(s.signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(s.signals)
	return s.runLifecycle(stdout, server, s.addr, s.signals)
}

func startServer(stdout io.Writer, cfg *config.Config) error {
	return newServeServer(servePort).run(stdout, cfg)
}

func startServerWithRuntime(stdout io.Writer, cfg *config.Config, runtime serveRuntime) error {
	return newServeServer(servePort).runWithRuntime(stdout, cfg, runtime)
}

func runServerLifecycle(stdout io.Writer, server devServer, addr string, signals <-chan os.Signal) error {
	go func() {
		<-signals

		fmt.Fprintln(stdout, "\nShutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	}()

	fmt.Fprintf(stdout, "Development server started at http://localhost%s\n", addr)
	fmt.Fprintln(stdout, "Press Ctrl+C to stop")

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

func newDevServer(addr, outputDir string, liveReload *liveReloadBroker) devServer {
	return &http.Server{
		Addr:         addr,
		Handler:      newDevServerHandler(outputDir, liveReload),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func newDevServerHandler(outputDir string, liveReload ...*liveReloadBroker) http.Handler {
	var broker *liveReloadBroker
	if len(liveReload) > 0 {
		broker = liveReload[0]
	}

	mux := http.NewServeMux()
	if broker != nil {
		mux.HandleFunc(liveReloadEndpoint, broker.serveEvents)
	}
	mux.Handle("/", newLiveReloadFileHandler(outputDir, broker))
	return mux
}
