package main

import (
	"encoding/json"
	"time"

	"github.com/cloudwego/eino/schema"
)

// ExecutionPlan represents a complex execution plan with dependencies and parallel steps
type ExecutionPlan struct {
	ID        string              `json:"id"`
	CreatedAt time.Time           `json:"created_at"`
	Goal      string              `json:"goal"`
	Steps     []ExecutionStep     `json:"steps"`
	DataFlow  map[string][]string `json:"data_flow,omitempty"` // step_id -> dependent_step_ids
}

// ExecutionStep represents a single step in the execution plan
type ExecutionStep struct {
	ID           string                 `json:"id"`
	Type         StepType               `json:"type"`
	Tool         string                 `json:"tool,omitempty"`
	Operation    string                 `json:"operation,omitempty"`
	Name         string                 `json:"name"`
	Parameters   map[string]interface{} `json:"parameters"`
	Dependencies []string               `json:"dependencies"`
	RetryPolicy  *RetryPolicy           `json:"retry_policy,omitempty"`
	ErrorPolicy  ErrorHandlingPolicy    `json:"error_policy,omitempty"`
	Parallel     bool                   `json:"parallel,omitempty"` // Can be executed in parallel with other steps
}

// StepType defines the type of execution step
type StepType string

const (
	StepTypeToolCall       StepType = "tool_call"
	StepTypeDataProcessing StepType = "data_processing"
	StepTypeCondition      StepType = "condition"
	StepTypeSupervisor     StepType = "supervisor_call"
	StepTypeDataStore      StepType = "data_store"
	StepTypeParallel       StepType = "parallel"
)

// ErrorHandlingPolicy defines how to handle errors
type ErrorHandlingPolicy string

const (
	ErrorPolicyFailFast ErrorHandlingPolicy = "fail_fast"
	ErrorPolicyContinue ErrorHandlingPolicy = "continue"
	ErrorPolicyRetry    ErrorHandlingPolicy = "retry"
	ErrorPolicyAskUser  ErrorHandlingPolicy = "ask_user"
)

// RetryPolicy defines retry behavior for a step
type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts"`
	BackoffTime time.Duration `json:"backoff_time"`
	Exponential bool          `json:"exponential"`
}

// ExecutionState holds the state during execution
type ExecutionState struct {
	Plan          *ExecutionPlan            `json:"plan"`
	CurrentStepID string                    `json:"current_step_id"`
	DataStore     map[string]interface{}    `json:"data_store"`
	StepResults   map[string]*StepResult    `json:"step_results"`
	ExecutionLog  []ExecutionLogEntry       `json:"execution_log"`
	ErrorContext  *ErrorContext             `json:"error_context,omitempty"`
	UserInput     []*schema.Message         `json:"user_input"`
	Iteration     int                       `json:"iteration"`
	MaxIterations int                       `json:"max_iterations"`
}

// StepResult represents the result of executing a step
type StepResult struct {
	StepID      string      `json:"step_id"`
	Status      StepStatus  `json:"status"`
	Result      interface{} `json:"result,omitempty"`
	Error       string      `json:"error,omitempty"`
	StartTime   time.Time   `json:"start_time"`
	EndTime     time.Time   `json:"end_time"`
	Attempts    int         `json:"attempts"`
	Duration    time.Duration `json:"duration"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// StepStatus represents the status of a step execution
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusSuccess   StepStatus = "success"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
	StepStatusWaiting   StepStatus = "waiting"
	StepStatusRetrying  StepStatus = "retrying"
)

// ExecutionLogEntry represents a log entry during execution
type ExecutionLogEntry struct {
	Timestamp time.Time   `json:"timestamp"`
	StepID    string      `json:"step_id"`
	Event     string      `json:"event"`
	Details   interface{} `json:"details,omitempty"`
	Level     LogLevel    `json:"level"`
}

// LogLevel defines the severity of log entries
type LogLevel string

const (
	LogLevelInfo    LogLevel = "info"
	LogLevelWarn    LogLevel = "warn"
	LogLevelError   LogLevel = "error"
	LogLevelDebug   LogLevel = "debug"
)

// ErrorContext provides context about current errors
type ErrorContext struct {
	FailedStepID    string      `json:"failed_step_id"`
	ErrorMessage    string      `json:"error_message"`
	ErrorType       ErrorType   `json:"error_type"`
	RetryCount      int         `json:"retry_count"`
	PossibleActions []string    `json:"possible_actions"`
	Context         interface{} `json:"context,omitempty"`
}

// ErrorType categorizes different types of errors
type ErrorType string

const (
	ErrorTypeTransient  ErrorType = "transient"  // Network, timeout, rate limit
	ErrorTypeSemantic   ErrorType = "semantic"   // Invalid parameters, validation
	ErrorTypePermanent  ErrorType = "permanent"  // Authentication, not found
	ErrorTypeUserAction ErrorType = "user_action" // Requires user intervention
)

// SupervisorRequest represents a request to the supervisor
type SupervisorRequest struct {
	PlanID          string                `json:"plan_id"`
	CurrentStep     *ExecutionStep        `json:"current_step"`
	ExecutionState  *ExecutionState       `json:"execution_state"`
	ErrorContext    *ErrorContext         `json:"error_context,omitempty"`
	AvailableActions []SupervisorAction   `json:"available_actions"`
	Context         map[string]interface{} `json:"context,omitempty"`
}

// SupervisorAction represents an action the supervisor can take
type SupervisorAction struct {
	Action      ActionType             `json:"action"`
	Description string                 `json:"description"`
	StepID      string                 `json:"step_id,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	NewStep     *ExecutionStep         `json:"new_step,omitempty"`
}

// ActionType defines types of actions the supervisor can take
type ActionType string

const (
	ActionContinue       ActionType = "continue"
	ActionRetry          ActionType = "retry"
	ActionSkip           ActionType = "skip"
	ActionAbort          ActionType = "abort"
	ActionModifyStep     ActionType = "modify_step"
	ActionAddStep        ActionType = "add_step"
	ActionReplan         ActionType = "replan"
	ActionAskUser        ActionType = "ask_user"
	ActionComplete       ActionType = "complete"
)

// SupervisorResponse represents the supervisor's decision
type SupervisorResponse struct {
	Action      ActionType             `json:"action"`
	Reason      string                 `json:"reason"`
	StepID      string                 `json:"step_id,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	NewStep     *ExecutionStep         `json:"new_step,omitempty"`
	NewPlan     *ExecutionPlan         `json:"new_plan,omitempty"`
	UserMessage string                 `json:"user_message,omitempty"`
}

// MCPToolConfig represents configuration for an MCP tool
type MCPToolConfig struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Endpoint    string                 `json:"endpoint,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Timeout     time.Duration          `json:"timeout"`
	Retries     int                    `json:"retries"`
}

// ParameterReference represents a reference to data from previous steps
type ParameterReference struct {
	Type      ReferenceType `json:"type"`
	Reference string        `json:"reference"`
	Path      string        `json:"path,omitempty"`      // JSONPath for extracting specific data
	Default   interface{}   `json:"default,omitempty"`   // Default value if reference fails
	Transform string        `json:"transform,omitempty"` // Transformation function name
}

// ReferenceType defines how to resolve parameter references
type ReferenceType string

const (
	ReferenceStepResult  ReferenceType = "step_result"  // Reference result from another step
	ReferenceUserInput   ReferenceType = "user_input"   // Reference original user input
	ReferenceDataStore   ReferenceType = "data_store"   // Reference data store value
	ReferenceEnvironment ReferenceType = "environment"  // Reference environment variable
	ReferenceStatic      ReferenceType = "static"       // Static value
)

// AgentConfig holds configuration for the unified dialog agent
type AgentConfig struct {
	MaxIterations     int                      `json:"max_iterations"`
	Tools             []MCPToolConfig          `json:"tools"`
	PlannerConfig     *PlannerConfig           `json:"planner_config"`
	ExecutorConfig    *ExecutorConfig          `json:"executor_config"`
	SupervisorConfig  *SupervisorConfig        `json:"supervisor_config"`
	DefaultRetryPolicy *RetryPolicy            `json:"default_retry_policy"`
	GlobalTimeout     time.Duration            `json:"global_timeout"`
	EnableDebug       bool                     `json:"enable_debug"`
}

// PlannerConfig configuration for the planner
type PlannerConfig struct {
	Model           string                 `json:"model"`
	Temperature     float64                `json:"temperature"`
	MaxTokens       int                    `json:"max_tokens"`
	SystemPrompt    string                 `json:"system_prompt,omitempty"`
	ExamplePlans    []ExecutionPlan        `json:"example_plans,omitempty"`
	PlanningTimeout time.Duration          `json:"planning_timeout"`
	ExtraParams     map[string]interface{} `json:"extra_params,omitempty"`
}

// ExecutorConfig configuration for the executor
type ExecutorConfig struct {
	MaxParallelSteps int                    `json:"max_parallel_steps"`
	StepTimeout      time.Duration          `json:"step_timeout"`
	EnableCaching    bool                   `json:"enable_caching"`
	CacheTTL         time.Duration          `json:"cache_ttl"`
	ExtraParams      map[string]interface{} `json:"extra_params,omitempty"`
}

// SupervisorConfig configuration for the supervisor
type SupervisorConfig struct {
	Model            string                 `json:"model"`
	Temperature      float64                `json:"temperature"`
	MaxTokens        int                    `json:"max_tokens"`
	SystemPrompt     string                 `json:"system_prompt,omitempty"`
	TriggerThreshold int                    `json:"trigger_threshold"` // How many failures before calling supervisor
	DecisionTimeout  time.Duration          `json:"decision_timeout"`
	ExtraParams      map[string]interface{} `json:"extra_params,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for ExecutionPlan
func (p *ExecutionPlan) MarshalJSON() ([]byte, error) {
	type Alias ExecutionPlan
	return json.Marshal((*Alias)(p))
}

// UnmarshalJSON implements custom JSON unmarshaling for ExecutionPlan
func (p *ExecutionPlan) UnmarshalJSON(data []byte) error {
	type Alias ExecutionPlan
	return json.Unmarshal(data, (*Alias)(p))
}

// FirstStep returns the first step to be executed in the plan
func (p *ExecutionPlan) FirstStep() string {
	if len(p.Steps) == 0 {
		return ""
	}
	return p.Steps[0].ID
}

// GetReadySteps returns steps that are ready to be executed (dependencies satisfied)
func (p *ExecutionPlan) GetReadySteps(completedSteps map[string]bool) []ExecutionStep {
	var readySteps []ExecutionStep

	for _, step := range p.Steps {
		if completedSteps[step.ID] {
			continue // Already completed
		}

		// Check if all dependencies are satisfied
		allDepsReady := true
		for _, depID := range step.Dependencies {
			if !completedSteps[depID] {
				allDepsReady = false
				break
			}
		}

		if allDepsReady {
			readySteps = append(readySteps, step)
		}
	}

	return readySteps
}

// AddLogEntry adds a log entry to the execution state
func (s *ExecutionState) AddLogEntry(stepID, event string, details interface{}, level LogLevel) {
	entry := ExecutionLogEntry{
		Timestamp: time.Now(),
		StepID:    stepID,
		Event:     event,
		Details:   details,
		Level:     level,
	}
	s.ExecutionLog = append(s.ExecutionLog, entry)
}

// SetStepResult sets the result for a step
func (s *ExecutionState) SetStepResult(stepID string, result *StepResult) {
	if s.StepResults == nil {
		s.StepResults = make(map[string]*StepResult)
	}
	s.StepResults[stepID] = result

	// Also store in data store for reference resolution
	if s.DataStore == nil {
		s.DataStore = make(map[string]interface{})
	}
	s.DataStore[stepID] = result.Result
}

// GetCompletedSteps returns a map of completed step IDs
func (s *ExecutionState) GetCompletedSteps() map[string]bool {
	completed := make(map[string]bool)
	for stepID, result := range s.StepResults {
		if result.Status == StepStatusSuccess {
			completed[stepID] = true
		}
	}
	return completed
}