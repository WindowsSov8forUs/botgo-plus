// This example receives native QQ events. It deliberately does not send messages,
// modify groups, or pretend that an in-memory log is a durable event queue.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/event"
	"github.com/WindowsSov8forUs/botgo-plus/interaction/webhook"
	"github.com/WindowsSov8forUs/botgo-plus/token"
)

func main() {
	if err := run(); err != nil {
		slog.Error("QQ webhook stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	credentials := &token.QQBotCredentials{AppID: os.Getenv("QQBOT_APP_ID"), AppSecret: os.Getenv("QQBOT_APP_SECRET")}
	dispatcher := event.NewDispatcher(func(ctx context.Context, payload *dto.WSPayload) error {
		slog.Info("received native QQ event", "type", payload.Type, "sequence", payload.Seq)
		return nil
	})
	event.RegisterTyped(dispatcher, dto.EventGroupMessageCreate, func(ctx context.Context, payload *dto.WSPayload, message *dto.WSGroupMessageData) error {
		// Replace this with a durable enqueue before acknowledging production events.
		// message.Raw and payload.RawMessage remain available for a separate protocol adapter.
		slog.Info("received native QQ group message", "message_id", message.ID, "sequence", payload.Seq)
		return nil
	})
	handler, err := webhook.NewHandler(credentials, webhook.WithEventHandler(dispatcher.Handle))
	if err != nil {
		return err
	}
	address := os.Getenv("QQBOT_LISTEN_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	mux := http.NewServeMux()
	mux.Handle("/qqbot", handler)
	server := &http.Server{
		Addr: address, Handler: mux,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 16 * 1024,
		BaseContext:    func(net.Listener) context.Context { return ctx },
	}
	go func() {
		<-ctx.Done()
		shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		_ = server.Shutdown(shutdown)
	}()
	slog.Info("listening behind an HTTPS reverse proxy", "address", address, "path", "/qqbot")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
