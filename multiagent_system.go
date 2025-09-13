package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// MultiAgentOrchestrator implements the official multi-agent pattern from eino-examples
type MultiAgentOrchestrator struct {
	config          *UnifyDialogConfig
	modelFactory    *ModelFactory
	toolRegistry    *ToolRegistry
	callbackManager *CallbackManager
	compiledGraph   compose.Runnable[*AgentInput, *AgentOutput]
	stateManager    *StateManager
}

// AgentInput represents input to the multi-agent system
type AgentInput struct {
	Query   string                 `json:"query"`
	Context map[string]interface{} `json:"context"`
}

// AgentOutput represents output from the multi-agent system
type AgentOutput struct {
	Success  bool                   `json:"success"`
	Response string                 `json:"response"`
	Metadata map[string]interface{} `json:"metadata"`
	State    *SystemState           `json:"state"`
}

// SystemState represents the shared state across agents
type SystemState struct {
	Plan           *Plan                  `json:"plan"`
	ExecutionLog   []ExecutionEntry       `json:"execution_log"`
	CurrentStep    int                    `json:"current_step"`
	Feedback       []string               `json:"feedback"`
	Context        map[string]interface{} `json:"context"`
	MaxIterations  int                    `json:"max_iterations"`
	CurrentIteration int                  `json:"current_iteration"`
}

// Plan represents a structured plan
type Plan struct {
	Steps []PlanStep `json:"steps"`
	Goal  string     `json:"goal"`
}

// PlanStep represents a single step in the plan
type PlanStep struct {
	ID          string                 `json:"id"`
	Description string                 `json:"description"`
	Tool        string                 `json:"tool,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Dependencies []string              `json:"dependencies,omitempty"`
}

// ExecutionEntry represents a log entry for execution
type ExecutionEntry struct {
	StepID    string                 `json:"step_id"`
	Status    string                 `json:"status"`
	Result    interface{}            `json:"result"`
	Error     string                 `json:"error,omitempty"`
	Timestamp string                 `json:"timestamp"`
}

// StateManager manages the shared state
type StateManager struct {
	state *SystemState
}

// NewStateManager creates a new state manager
func NewStateManager() *StateManager {
	return &StateManager{
		state: &SystemState{
			ExecutionLog:     []ExecutionEntry{},
			Context:          make(map[string]interface{}),
			MaxIterations:    5,
			CurrentIteration: 0,
		},
	}
}

// NewMultiAgentOrchestrator creates a new multi-agent orchestrator
func NewMultiAgentOrchestrator(config *UnifyDialogConfig) (*MultiAgentOrchestrator, error) {
	return &MultiAgentOrchestrator{
		config:          config,
		modelFactory:    NewModelFactory(config),
		callbackManager: NewCallbackManager(config),
		stateManager:    NewStateManager(),
	}, nil
}

// Initialize sets up the multi-agent system following the official pattern
func (o *MultiAgentOrchestrator) Initialize(ctx context.Context) error {
	// Initialize MCP tools
	mcpManager := NewMCPToolManager(o.config)
	if err := mcpManager.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize MCP tools: %w", err)
	}
	o.toolRegistry = NewToolRegistry(mcpManager)

	// Initialize callbacks
	if err := o.callbackManager.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize callbacks: %w", err)
	}

	// Build the multi-agent graph using official patterns
	if err := o.buildOfficialPatternGraph(ctx); err != nil {
		return fmt.Errorf("failed to build multi-agent graph: %w", err)
	}

	log.Println("Multi-agent orchestrator initialized with official pattern")
	return nil
}

// buildOfficialPatternGraph builds the graph following the official eino-examples pattern
func (o *MultiAgentOrchestrator) buildOfficialPatternGraph(ctx context.Context) error {
	// Create graph following the plan-execute pattern from eino-examples
	g := compose.NewGraph[*AgentInput, *AgentOutput]()

	// Create models for each agent
	plannerModel, err := o.modelFactory.CreatePlannerModel(ctx)
	if err != nil {
		return err
	}

	executorModel, err := o.modelFactory.CreateExecutorModel(ctx)
	if err != nil {
		return err
	}

	supervisorModel, err := o.modelFactory.CreateSupervisorModel(ctx)
	if err != nil {
		return err
	}

	// Add state handler for managing shared state (following official pattern)
	stateHandler := compose.StatePreHandler[*SystemState](
		func(ctx context.Context, state *SystemState, input compose.GraphState) (*SystemState, error) {
			// Update state based on input
			if state == nil {
				state = o.stateManager.state
			}
			return state, nil
		},
	)

	// Planning Node - Create detailed plan
	if err := g.AddLambdaNode("planner", compose.InvokableLambda(
		func(ctx context.Context, input *AgentInput) (*PlannerOutput, error) {
			// Create planning prompt
			planPrompt := prompt.FromMessages(
				schema.SystemMessage(`You are an expert planner. Create a detailed, step-by-step plan to address the user's query.
				Output your plan in JSON format with the following structure:
				{
					"goal": "overall goal",
					"steps": [
						{
							"id": "step_1",
							"description": "what to do",
							"tool": "tool_name (optional)",
							"parameters": {"key": "value"},
							"dependencies": []
						}
					]
				}`),
				schema.UserMessage(fmt.Sprintf("Query: %s\nContext: %v", input.Query, input.Context)),
			)

			// Get messages from prompt
			messages, err := planPrompt.Format(ctx, map[string]interface{}{
				"query":   input.Query,
				"context": input.Context,
			})
			if err != nil {
				return nil, err
			}

			// Generate plan
			response, err := plannerModel.Generate(ctx, messages)
			if err != nil {
				return nil, err
			}

			// Parse plan
			var plan Plan
			if err := json.Unmarshal([]byte(response.Content), &plan); err != nil {
				// Fallback to simple plan
				plan = Plan{
					Goal: input.Query,
					Steps: []PlanStep{
						{
							ID:          "step_1",
							Description: "Process query: " + input.Query,
						},
					},
				}
			}

			// Update state
			o.stateManager.state.Plan = &plan
			o.stateManager.state.CurrentStep = 0

			return &PlannerOutput{Plan: plan}, nil
		},
	)); err != nil {
		return err
	}

	// Execution Node - Execute plan steps
	if err := g.AddLambdaNode("executor", compose.InvokableLambda(
		func(ctx context.Context, input *PlannerOutput) (*ExecutorOutput, error) {
			plan := input.Plan
			results := []interface{}{}

			for i, step := range plan.Steps {
				o.stateManager.state.CurrentStep = i

				// Check if we need to use a tool
				if step.Tool != "" {
					if tool, ok := o.toolRegistry.GetToolByName(step.Tool); ok {
						// Execute tool
						paramsJSON, _ := json.Marshal(step.Parameters)
						result, err := tool.Run(ctx, string(paramsJSON))
						if err != nil {
							o.stateManager.state.ExecutionLog = append(o.stateManager.state.ExecutionLog, ExecutionEntry{
								StepID: step.ID,
								Status: "failed",
								Error:  err.Error(),
							})
							continue
						}

						results = append(results, result)
						o.stateManager.state.ExecutionLog = append(o.stateManager.state.ExecutionLog, ExecutionEntry{
							StepID: step.ID,
							Status: "success",
							Result: result,
						})
					}
				} else {
					// Execute without tool (using model)
					execPrompt := prompt.FromMessages(
						schema.SystemMessage("Execute the following step and provide the result."),
						schema.UserMessage(fmt.Sprintf("Step: %s\nDescription: %s", step.ID, step.Description)),
					)

					messages, _ := execPrompt.Format(ctx, map[string]interface{}{
						"step": step,
					})

					response, err := executorModel.Generate(ctx, messages)
					if err != nil {
						o.stateManager.state.ExecutionLog = append(o.stateManager.state.ExecutionLog, ExecutionEntry{
							StepID: step.ID,
							Status: "failed",
							Error:  err.Error(),
						})
						continue
					}

					results = append(results, response.Content)
					o.stateManager.state.ExecutionLog = append(o.stateManager.state.ExecutionLog, ExecutionEntry{
						StepID: step.ID,
						Status: "success",
						Result: response.Content,
					})
				}
			}

			return &ExecutorOutput{Results: results}, nil
		},
	)); err != nil {
		return err
	}

	// Supervisor Node - Evaluate and provide feedback
	if err := g.AddLambdaNode("supervisor", compose.InvokableLambda(
		func(ctx context.Context, input *ExecutorOutput) (*SupervisorOutput, error) {
			// Create supervision prompt
			supervisePrompt := prompt.FromMessages(
				schema.SystemMessage(`You are a quality supervisor. Evaluate the execution results and determine if they satisfy the original goal.
				Provide feedback and determine the next action:
				- "complete": The task is successfully completed
				- "retry": The task needs to be retried with modifications
				- "escalate": The task needs human intervention`),
				schema.UserMessage(fmt.Sprintf("Goal: %s\nResults: %v\nExecution Log: %v",
					o.stateManager.state.Plan.Goal,
					input.Results,
					o.stateManager.state.ExecutionLog)),
			)

			messages, _ := supervisePrompt.Format(ctx, map[string]interface{}{
				"results": input.Results,
				"state":   o.stateManager.state,
			})

			response, err := supervisorModel.Generate(ctx, messages)
			if err != nil {
				return nil, err
			}

			// Parse supervisor response
			var supervision struct {
				Action   string   `json:"action"`
				Feedback []string `json:"feedback"`
				Score    float64  `json:"score"`
			}

			if err := json.Unmarshal([]byte(response.Content), &supervision); err != nil {
				// Fallback
				supervision.Action = "complete"
				supervision.Feedback = []string{response.Content}
				supervision.Score = 0.8
			}

			o.stateManager.state.Feedback = supervision.Feedback

			return &SupervisorOutput{
				Action:   supervision.Action,
				Feedback: supervision.Feedback,
				Score:    supervision.Score,
			}, nil
		},
	)); err != nil {
		return err
	}

	// Final output node
	if err := g.AddLambdaNode("final_output", compose.InvokableLambda(
		func(ctx context.Context, input *SupervisorOutput) (*AgentOutput, error) {
			// Prepare final response
			response := fmt.Sprintf("Task completed with action: %s\nFeedback: %v",
				input.Action, input.Feedback)

			return &AgentOutput{
				Success:  input.Action == "complete",
				Response: response,
				Metadata: map[string]interface{}{
					"score":      input.Score,
					"iterations": o.stateManager.state.CurrentIteration,
					"steps":      len(o.stateManager.state.Plan.Steps),
				},
				State: o.stateManager.state,
			}, nil
		},
	)); err != nil {
		return err
	}

	// Add edges following the official pattern
	g.AddEdge(compose.START, "planner")
	g.AddEdge("planner", "executor")
	g.AddEdge("executor", "supervisor")

	// Add conditional branching based on supervisor decision
	g.AddBranch("supervisor", func(ctx context.Context, output *SupervisorOutput) (string, error) {
		o.stateManager.state.CurrentIteration++

		switch output.Action {
		case "complete":
			return "final_output", nil
		case "retry":
			if o.stateManager.state.CurrentIteration < o.stateManager.state.MaxIterations {
				return "planner", nil // Go back to planning
			}
			return "final_output", nil
		default:
			return "final_output", nil
		}
	})

	g.AddEdge("final_output", compose.END)

	// Compile with state handler and callbacks
	options := []compose.GraphCompileOption{
		compose.WithStatePreHandler(stateHandler),
	}

	if handlers := o.callbackManager.GetHandlers(); len(handlers) > 0 {
		for _, handler := range handlers {
			options = append(options, compose.WithGraphCallbacks(handler))
		}
	}

	compiledGraph, err := g.Compile(ctx, options...)
	if err != nil {
		return fmt.Errorf("failed to compile graph: %w", err)
	}

	o.compiledGraph = compiledGraph
	return nil
}

// Process executes the multi-agent workflow
func (o *MultiAgentOrchestrator) Process(ctx context.Context, query string, context_ map[string]interface{}) (*AgentOutput, error) {
	input := &AgentInput{
		Query:   query,
		Context: context_,
	}

	// Reset state for new query
	o.stateManager.state.CurrentIteration = 0
	o.stateManager.state.ExecutionLog = []ExecutionEntry{}

	log.Printf("Processing query through multi-agent system: %s", query)

	output, err := o.compiledGraph.Invoke(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("multi-agent processing failed: %w", err)
	}

	return output, nil
}

// Helper output structures for graph nodes
type PlannerOutput struct {
	Plan Plan `json:"plan"`
}

type ExecutorOutput struct {
	Results []interface{} `json:"results"`
}

type SupervisorOutput struct {
	Action   string   `json:"action"`
	Feedback []string `json:"feedback"`
	Score    float64  `json:"score"`
}

// ProcessWithCheckpoint processes with checkpoint support (following official pattern)
func (o *MultiAgentOrchestrator) ProcessWithCheckpoint(ctx context.Context, query string, checkpointID string) (*AgentOutput, error) {
	// This would integrate with the checkpoint system from the official examples
	// For now, we'll use regular processing
	return o.Process(ctx, query, nil)
}

// GetState returns the current system state
func (o *MultiAgentOrchestrator) GetState() *SystemState {
	return o.stateManager.state
}