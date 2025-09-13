package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/callbacks"
)

// CallbackManager manages all callback handlers for the system
type CallbackManager struct {
	config   *UnifyDialogConfig
	handlers []callbacks.Handler
}

// NewCallbackManager creates a new callback manager
func NewCallbackManager(config *UnifyDialogConfig) *CallbackManager {
	return &CallbackManager{
		config:   config,
		handlers: []callbacks.Handler{},
	}
}

// Initialize sets up all configured callback handlers
func (cm *CallbackManager) Initialize(ctx context.Context) error {
	if !cm.config.System.EnableCallbacks {
		log.Println("Callbacks are disabled in configuration")
		return nil
	}

	// Add custom debug handler if debug mode is enabled
	if cm.config.System.Debug {
		cm.handlers = append(cm.handlers, NewDebugCallbackHandler())
	}

	// Add custom logging handler
	cm.handlers = append(cm.handlers, NewLoggingCallbackHandler(cm.config.System.LogLevel))

	// Add performance monitoring handler
	cm.handlers = append(cm.handlers, NewPerformanceCallbackHandler())

	// Future: Add Cozeloop handler
	// if cm.config.Callbacks.Cozeloop.Enabled {
	//     handler, err := cm.createCozeloopHandler()
	//     if err != nil {
	//         log.Printf("Failed to create Cozeloop handler: %v", err)
	//     } else {
	//         cm.handlers = append(cm.handlers, handler)
	//     }
	// }

	// Future: Add LangSmith handler
	// if cm.config.Callbacks.LangSmith.Enabled {
	//     handler, err := cm.createLangSmithHandler()
	//     if err != nil {
	//         log.Printf("Failed to create LangSmith handler: %v", err)
	//     } else {
	//         cm.handlers = append(cm.handlers, handler)
	//     }
	// }

	log.Printf("Initialized %d callback handlers", len(cm.handlers))
	return nil
}

// GetHandlers returns all configured handlers
func (cm *CallbackManager) GetHandlers() []callbacks.Handler {
	return cm.handlers
}

// DebugCallbackHandler provides detailed debug information
type DebugCallbackHandler struct {
	callbacks.SimpleCallbackHandler
}

// NewDebugCallbackHandler creates a new debug callback handler
func NewDebugCallbackHandler() *DebugCallbackHandler {
	h := &DebugCallbackHandler{}

	h.OnStartFunc = func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		log.Printf("[DEBUG] Starting %s (Node: %s, RunID: %s)", info.Type, info.Name, info.RunID)
		log.Printf("[DEBUG] Input type: %T", input)
		return ctx
	}

	h.OnEndFunc = func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		log.Printf("[DEBUG] Completed %s (Node: %s, RunID: %s)", info.Type, info.Name, info.RunID)
		log.Printf("[DEBUG] Output type: %T", output)
		return ctx
	}

	h.OnErrorFunc = func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
		log.Printf("[DEBUG] Error in %s (Node: %s, RunID: %s): %v", info.Type, info.Name, info.RunID, err)
		return ctx
	}

	return h
}

// LoggingCallbackHandler provides structured logging
type LoggingCallbackHandler struct {
	callbacks.SimpleCallbackHandler
	logLevel string
}

// NewLoggingCallbackHandler creates a new logging callback handler
func NewLoggingCallbackHandler(logLevel string) *LoggingCallbackHandler {
	h := &LoggingCallbackHandler{logLevel: logLevel}

	h.OnStartFunc = func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		if h.shouldLog("info") {
			log.Printf("[INFO] Starting %s: %s", info.Type, info.Name)
		}
		return ctx
	}

	h.OnEndFunc = func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		if h.shouldLog("info") {
			log.Printf("[INFO] Completed %s: %s", info.Type, info.Name)
		}
		return ctx
	}

	h.OnErrorFunc = func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
		if h.shouldLog("error") {
			log.Printf("[ERROR] Failed %s: %s - %v", info.Type, info.Name, err)
		}
		return ctx
	}

	return h
}

func (h *LoggingCallbackHandler) shouldLog(level string) bool {
	// Simple log level checking
	levels := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}

	configLevel, ok := levels[h.logLevel]
	if !ok {
		configLevel = 1 // default to info
	}

	requestLevel, ok := levels[level]
	if !ok {
		requestLevel = 1
	}

	return requestLevel >= configLevel
}

// PerformanceCallbackHandler tracks performance metrics
type PerformanceCallbackHandler struct {
	callbacks.SimpleCallbackHandler
	startTimes map[string]time.Time
}

// NewPerformanceCallbackHandler creates a new performance callback handler
func NewPerformanceCallbackHandler() *PerformanceCallbackHandler {
	h := &PerformanceCallbackHandler{
		startTimes: make(map[string]time.Time),
	}

	h.OnStartFunc = func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		h.startTimes[info.RunID] = time.Now()
		return ctx
	}

	h.OnEndFunc = func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
		if startTime, ok := h.startTimes[info.RunID]; ok {
			duration := time.Since(startTime)
			log.Printf("[PERF] %s (%s) completed in %v", info.Type, info.Name, duration)
			delete(h.startTimes, info.RunID)
		}
		return ctx
	}

	h.OnErrorFunc = func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
		if startTime, ok := h.startTimes[info.RunID]; ok {
			duration := time.Since(startTime)
			log.Printf("[PERF] %s (%s) failed after %v", info.Type, info.Name, duration)
			delete(h.startTimes, info.RunID)
		}
		return ctx
	}

	return h
}

// StreamingCallbackHandler handles streaming outputs
type StreamingCallbackHandler struct {
	callbacks.SimpleCallbackHandler
	onChunk func(chunk string)
}

// NewStreamingCallbackHandler creates a new streaming callback handler
func NewStreamingCallbackHandler(onChunk func(chunk string)) *StreamingCallbackHandler {
	h := &StreamingCallbackHandler{
		onChunk: onChunk,
	}

	// This would handle streaming chunks in a real implementation
	// For now it's a placeholder for when we add streaming support

	return h
}

// Helper function to combine multiple handlers
func CombineHandlers(handlers ...callbacks.Handler) callbacks.Handler {
	if len(handlers) == 0 {
		return &callbacks.SimpleCallbackHandler{}
	}
	if len(handlers) == 1 {
		return handlers[0]
	}

	// In a real implementation, we would properly combine handlers
	// For now, return the first non-nil handler
	for _, h := range handlers {
		if h != nil {
			return h
		}
	}
	return &callbacks.SimpleCallbackHandler{}
}

// Future implementations for external callback services

// createCozeloopHandler creates a Cozeloop callback handler
// This would be implemented when eino-ext/callbacks/cozeloop is available
/*
func (cm *CallbackManager) createCozeloopHandler() (callbacks.Handler, error) {
	import "github.com/cloudwego/eino-ext/callbacks/cozeloop"

	client, err := cozeloop.NewClient(
		cozeloop.WithAPIToken(cm.config.Callbacks.Cozeloop.APIToken),
		cozeloop.WithWorkspaceID(cm.config.Callbacks.Cozeloop.WorkspaceID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cozeloop client: %w", err)
	}

	return cozeloop.NewLoopHandler(client), nil
}
*/

// createLangSmithHandler creates a LangSmith callback handler
// This would be implemented when eino-ext/callbacks/langsmith is available
/*
func (cm *CallbackManager) createLangSmithHandler() (callbacks.Handler, error) {
	import "github.com/cloudwego/eino-ext/callbacks/langsmith"

	config := &langsmith.Config{
		APIKey:   cm.config.Callbacks.LangSmith.APIKey,
		Endpoint: cm.config.Callbacks.LangSmith.Endpoint,
	}

	return langsmith.NewHandler(config)
}
*/