package observability

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

func StartServer(ctx context.Context, addr string, handler http.Handler) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen metrics server on %s: %w", addr, err)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Startup errors are caught by net.Listen before this goroutine starts.
			return
		}
	}()

	return nil
}
