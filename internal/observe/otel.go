package observe

import (
	"context"
	"fmt"
	"time"
)

// Simple tracer implementation for MVP
type Tracer struct {
	serviceName string
}

func NewTracer(serviceName string) (*Tracer, error) {
	return &Tracer{serviceName: serviceName}, nil
}

func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, func()) {
	start := time.Now()
	fmt.Printf("[TRACE] %s.%s started\n", t.serviceName, name)
	
	return ctx, func() {
		duration := time.Since(start)
		fmt.Printf("[TRACE] %s.%s completed (%v)\n", t.serviceName, name, duration)
	}
}

func (t *Tracer) AddEvent(ctx context.Context, name string, attrs map[string]string) {
	fmt.Printf("[TRACE] %s event: %s %v\n", t.serviceName, name, attrs)
}

func (t *Tracer) SetAttributes(ctx context.Context, attrs map[string]string) {
	fmt.Printf("[TRACE] %s attributes: %v\n", t.serviceName, attrs)
}

func (t *Tracer) RecordError(ctx context.Context, err error, attrs map[string]string) {
	fmt.Printf("[TRACE] %s error: %v %v\n", t.serviceName, err, attrs)
}