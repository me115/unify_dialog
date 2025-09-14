package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cloudwego/eino/callbacks"
)

// CallbackManager 管理系统的所有回调处理器
type CallbackManager struct {
	config   *UnifyDialogConfig
	handlers []callbacks.Handler
}

// NewCallbackManager 创建新的回调管理器
func NewCallbackManager(config *UnifyDialogConfig) *CallbackManager {
	return &CallbackManager{
		config:   config,
		handlers: []callbacks.Handler{},
	}
}

// Initialize 设置所有已配置的回调处理器
func (cm *CallbackManager) Initialize(ctx context.Context) error {
	if !cm.config.System.EnableCallbacks {
		log.Println("配置中禁用了回调")
		return nil
	}

	// 如果启用调试模式，添加自定义调试处理器
	if cm.config.System.Debug {
		cm.handlers = append(cm.handlers, NewDebugCallbackHandler())
	}

	// 添加自定义日志处理器
	cm.handlers = append(cm.handlers, NewLoggingCallbackHandler(cm.config.System.LogLevel))

	// 添加性能监控处理器
	cm.handlers = append(cm.handlers, NewPerformanceCallbackHandler())

	// 未来：添加Cozeloop处理器
	// if cm.config.Callbacks.Cozeloop.Enabled {
	//     handler, err := cm.createCozeloopHandler()
	//     if err != nil {
	//         log.Printf("Failed to create Cozeloop handler: %v", err)
	//     } else {
	//         cm.handlers = append(cm.handlers, handler)
	//     }
	// }

	// 未来：添加LangSmith处理器
	// if cm.config.Callbacks.LangSmith.Enabled {
	//     handler, err := cm.createLangSmithHandler()
	//     if err != nil {
	//         log.Printf("Failed to create LangSmith handler: %v", err)
	//     } else {
	//         cm.handlers = append(cm.handlers, handler)
	//     }
	// }

	log.Printf("初始化了 %d 个回调处理器", len(cm.handlers))
	return nil
}

// GetHandlers 返回所有已配置的处理器
func (cm *CallbackManager) GetHandlers() []callbacks.Handler {
	return cm.handlers
}

// DebugCallbackHandler 提供详细的调试信息
type DebugCallbackHandler struct {
	callbacks.SimpleCallbackHandler
}

// NewDebugCallbackHandler 创建新的调试回调处理器
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

// LoggingCallbackHandler 提供结构化日志记录
type LoggingCallbackHandler struct {
	callbacks.SimpleCallbackHandler
	logLevel string
}

// NewLoggingCallbackHandler 创建新的日志回调处理器
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
	// 简单的日志级别检查
	levels := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}

	configLevel, ok := levels[h.logLevel]
	if !ok {
		configLevel = 1 // 默认为info
	}

	requestLevel, ok := levels[level]
	if !ok {
		requestLevel = 1
	}

	return requestLevel >= configLevel
}

// PerformanceCallbackHandler 跟踪性能指标
type PerformanceCallbackHandler struct {
	callbacks.SimpleCallbackHandler
	startTimes map[string]time.Time
}

// NewPerformanceCallbackHandler 创建新的性能回调处理器
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

// StreamingCallbackHandler 处理流式输出
type StreamingCallbackHandler struct {
	callbacks.SimpleCallbackHandler
	onChunk func(chunk string)
}

// NewStreamingCallbackHandler 创建新的流式回调处理器
func NewStreamingCallbackHandler(onChunk func(chunk string)) *StreamingCallbackHandler {
	h := &StreamingCallbackHandler{
		onChunk: onChunk,
	}

	// 在真实实现中这将处理流式数据块
	// 目前只是一个占位符，等待添加流式支持

	return h
}

// CombineHandlers 组合多个处理器的辅助函数
func CombineHandlers(handlers ...callbacks.Handler) callbacks.Handler {
	if len(handlers) == 0 {
		return &callbacks.SimpleCallbackHandler{}
	}
	if len(handlers) == 1 {
		return handlers[0]
	}

	// 在真实实现中，我们会正确组合处理器
	// 现在返回第一个非空处理器
	for _, h := range handlers {
		if h != nil {
			return h
		}
	}
	return &callbacks.SimpleCallbackHandler{}
}

// 外部回调服务的未来实现

// createCozeloopHandler 创建Cozeloop回调处理器
// 当eino-ext/callbacks/cozeloop可用时将实现
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

// createLangSmithHandler 创建LangSmith回调处理器
// 当eino-ext/callbacks/langsmith可用时将实现
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