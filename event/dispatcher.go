package event

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
)

type Handler func(context.Context, *dto.WSPayload) error

// Dispatcher is application-local and does not share the legacy global handler registry.
// It deliberately leaves durable queues and event deduplication to the receiving application.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[dto.EventType]Handler
	fallback Handler
}

func NewDispatcher(fallback Handler) *Dispatcher {
	return &Dispatcher{handlers: make(map[dto.EventType]Handler), fallback: fallback}
}

func (d *Dispatcher) On(kind dto.EventType, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if handler == nil {
		delete(d.handlers, kind)
	} else {
		d.handlers[kind] = handler
	}
}

func (d *Dispatcher) Handle(ctx context.Context, payload *dto.WSPayload) error {
	if payload == nil {
		return errors.New("nil QQ event")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	d.mu.RLock()
	handler := d.handlers[payload.Type]
	if handler == nil {
		handler = d.fallback
	}
	d.mu.RUnlock()
	if handler != nil {
		return handler(ctx, payload)
	}
	return nil
}

// RegisterTyped decodes QQ's native 'd' field while retaining the full envelope for callers.
func RegisterTyped[T any](d *Dispatcher, kind dto.EventType, handler func(context.Context, *dto.WSPayload, *T) error) {
	if handler == nil {
		d.On(kind, nil)
		return
	}
	d.On(kind, func(ctx context.Context, payload *dto.WSPayload) error {
		var envelope struct {
			Data json.RawMessage `json:"d"`
		}
		if err := json.Unmarshal(payload.RawMessage, &envelope); err != nil {
			return err
		}
		if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
			return errors.New("QQ event has no data")
		}
		var value T
		if err := json.Unmarshal(envelope.Data, &value); err != nil {
			return err
		}
		return handler(ctx, payload, &value)
	})
}
