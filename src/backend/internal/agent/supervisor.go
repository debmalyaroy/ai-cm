package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/memory"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SupervisorAgent orchestrates requests by classifying intent and delegating to worker agents.
// It integrates all 6 agents (Analyst, Strategist, Planner, Liaison, Watchdog) and the Critic layer.
type SupervisorAgent struct {
	llmClient  llm.Client
	intentLLM  llm.Client
	analyst    *AnalystAgent
	strategist *StrategistAgent
	planner    *PlannerAgent
	liaison    *LiaisonAgent
	watchdog   *WatchdogAgent
	critic     *CriticLayer
	memory     memory.Manager
}

// NewSupervisorAgent creates a new supervisor that manages all worker agents.
func NewSupervisorAgent(llmClient llm.Client, db *pgxpool.Pool, agentModels map[string]string) *SupervisorAgent {
	tools := NewToolSet(db, llmClient)

	var mem memory.Manager
	if db != nil && llmClient != nil {
		mem = memory.NewPgStore(db, llmClient)
	}

	if agentModels == nil {
		agentModels = make(map[string]string)
	}

	var intentLLM llm.Client
	if m, ok := agentModels["supervisor"]; ok && m != "" {
		intentLLM = llmClient.WithModel(m)
	}

	analystLLM := llmClient
	if m, ok := agentModels["analyst"]; ok && m != "" {
		analystLLM = llmClient.WithModel(m)
	}

	strategistLLM := llmClient
	if m, ok := agentModels["strategist"]; ok && m != "" {
		strategistLLM = llmClient.WithModel(m)
	}

	plannerLLM := llmClient
	if m, ok := agentModels["planner"]; ok && m != "" {
		plannerLLM = llmClient.WithModel(m)
	}

	liaisonLLM := llmClient
	if m, ok := agentModels["liaison"]; ok && m != "" {
		liaisonLLM = llmClient.WithModel(m)
	}

	return &SupervisorAgent{
		llmClient:  llmClient,
		intentLLM:  intentLLM,
		analyst:    NewAnalystAgent(analystLLM, db, tools),
		strategist: NewStrategistAgent(strategistLLM, db, tools),
		planner:    NewPlannerAgent(db, plannerLLM),
		liaison:    NewLiaisonAgent(liaisonLLM),
		watchdog:   NewWatchdogAgent(db),
		critic:     NewCriticLayer(),
		memory:     mem,
	}
}

func (s *SupervisorAgent) Name() string { return "supervisor" }

func (s *SupervisorAgent) Process(ctx context.Context, input *Input) (*Output, error) {
	slog.DebugContext(ctx, "Executing supervisor agent")

	// Step 0: Enrich input with memory context
	if s.memory != nil && input.SessionID != "" {
		mc, err := s.memory.BuildContext(ctx, input.SessionID, input.Query)
		if err != nil {
			slog.WarnContext(ctx, "Supervisor: memory context failed (non-fatal)", "error", err)
		} else {
			input.MemoryContext = mc
			slog.DebugContext(ctx, "Supervisor: memory context built",
				"session_history_len", len(mc.SessionHistory),
				"episodes_len", len(mc.SimilarEpisodes),
				"facts_len", len(mc.RelevantFacts))
		}
	}

	// Step 1: Classify the intent
	intent := s.classifyIntent(ctx, input.Query)
	slog.InfoContext(ctx, "Supervisor: classified intent", "intent", intent, "query", input.Query, "session_id", input.SessionID)

	output := &Output{
		AgentName: s.Name(),
		Reasoning: []ReasoningStep{
			{Type: "thought", Content: fmt.Sprintf("Classified user intent as: %s", intent)},
		},
	}

	switch intent {
	case IntentQuery:
		// Delegate to Analyst for data retrieval
		output.Reasoning = append(output.Reasoning, ReasoningStep{
			Type:    "action",
			Content: "Delegating to Analyst Agent for data retrieval",
		})

		slog.DebugContext(ctx, "Supervisor: delegating intent processing", "intent", intent, "agent", "analyst")
		analystOutput, err := s.analyst.Process(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("analyst error: %w", err)
		}
		output.Response = analystOutput.Response
		output.Data = analystOutput.Data
		output.AgentName = analystOutput.AgentName
		output.Reasoning = append(output.Reasoning, analystOutput.Reasoning...)

	case IntentInsight:
		// First get data from Analyst, then reason with Strategist
		output.Reasoning = append(output.Reasoning, ReasoningStep{
			Type:    "action",
			Content: "Delegating to Analyst for data, then Strategist for insight",
		})

		// Get data
		slog.DebugContext(ctx, "Supervisor: delegating intent processing part 1", "intent", intent, "agent", "analyst")
		analystOutput, err := s.analyst.Process(ctx, input)
		if err != nil {
			slog.WarnContext(ctx, "Analyst failed, falling back to Strategist only", "error", err, "session_id", input.SessionID)
			// Fall through to Strategist without analyst data
		}

		// Enrich context for Strategist
		strategistInput := &Input{
			Query:         input.Query,
			SessionID:     input.SessionID,
			Context:       make(map[string]any),
			History:       input.History,
			MemoryContext: input.MemoryContext,
		}
		if err == nil && analystOutput != nil {
			strategistInput.Context["analyst_data"] = analystOutput.Data
		}

		slog.DebugContext(ctx, "Supervisor: delegating intent processing part 2", "intent", intent, "agent", "strategist")
		strategistOutput, err := s.strategist.Process(ctx, strategistInput)
		if err != nil {
			return nil, fmt.Errorf("strategist error: %w", err)
		}

		output.Response = strategistOutput.Response
		output.AgentName = strategistOutput.AgentName
		if analystOutput != nil {
			output.Data = analystOutput.Data
		}
		output.Reasoning = append(output.Reasoning, strategistOutput.Reasoning...)

	case IntentPlan:
		// Delegate to Planner for action proposals
		output.Reasoning = append(output.Reasoning, ReasoningStep{
			Type:    "action",
			Content: "Delegating to Planner Agent for action proposals",
		})

		slog.DebugContext(ctx, "Supervisor: delegating intent processing", "intent", intent, "agent", "planner")
		plannerOutput, err := s.planner.Process(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("planner error: %w", err)
		}
		output.Response = plannerOutput.Response
		output.Actions = plannerOutput.Actions
		output.AgentName = plannerOutput.AgentName
		output.Reasoning = append(output.Reasoning, plannerOutput.Reasoning...)

	case IntentCommunicate:
		// Delegate to Liaison for communication drafts
		output.Reasoning = append(output.Reasoning, ReasoningStep{
			Type:    "action",
			Content: "Delegating to Liaison Agent for communication generation",
		})

		slog.DebugContext(ctx, "Supervisor: delegating intent processing", "intent", intent, "agent", "liaison")
		liaisonOutput, err := s.liaison.Process(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("liaison error: %w", err)
		}
		output.Response = liaisonOutput.Response
		output.AgentName = liaisonOutput.AgentName
		output.Reasoning = append(output.Reasoning, liaisonOutput.Reasoning...)

	case IntentMonitor:
		// Delegate to Watchdog for anomaly detection
		output.Reasoning = append(output.Reasoning, ReasoningStep{
			Type:    "action",
			Content: "Delegating to Watchdog Agent for anomaly detection",
		})

		slog.DebugContext(ctx, "Supervisor: delegating intent processing", "intent", intent, "agent", "watchdog")
		watchdogOutput, err := s.watchdog.Process(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("watchdog error: %w", err)
		}
		output.Response = watchdogOutput.Response
		output.Data = watchdogOutput.Data
		output.AgentName = watchdogOutput.AgentName
		output.Reasoning = append(output.Reasoning, watchdogOutput.Reasoning...)

	case IntentGeneral:
		output.Response = `👋 Hello! I'm your **AI Category Manager Copilot**.

I can help you with:
- 📊 **Data Queries**: "Show me sales by region" or "What's the top selling product?"
- 🔍 **Insights**: "Why did margin drop in East India?" or "Explain the sales trend"
- ⚡ **Actions**: "Propose a promotional plan for diapers"
- ✉️ **Communications**: "Draft a compliance alert for seller"
- 🔔 **Monitoring**: "Check for anomalies" or "System health"

What would you like to know?`

	default:
		output.Response = "I'm not sure how to help with that. Could you rephrase your question? I can help with data queries, insights, and action recommendations."
	}

	// Step 3: Critic post-processing (Reflection pattern)
	if s.critic != nil {
		validation := s.critic.Validate(ctx, output)
		if len(validation.Warnings) > 0 {
			slog.WarnContext(ctx, "Critic warnings", "warnings", validation.Warnings, "session_id", input.SessionID)
			output.Response = validation.Cleaned
			output.Reasoning = append(output.Reasoning, ReasoningStep{
				Type:    "observation",
				Content: fmt.Sprintf("Critic flagged %d warning(s)", len(validation.Warnings)),
			})
		}
	}

	// Note: episodic memory storage is handled by handleChat's goroutine via StoreEpisodicMemory.
	// Do NOT store here to avoid duplicate records in agent_memory.

	return output, nil
}

// StoreEpisodicMemory saves a successful Q/A interaction into the agent's long-term memory tier
// by generating an embedding and stringing the interaction.
func (s *SupervisorAgent) StoreEpisodicMemory(ctx context.Context, sessionID, query, response, agentName string) error {
	if s.memory == nil {
		return nil
	}
	return s.memory.StoreEpisode(ctx, sessionID, query, response, agentName)
}

func (s *SupervisorAgent) classifyIntent(ctx context.Context, query string) IntentType {
	// Attempt LLM-based classification first if intentLLM is configured
	if s.intentLLM != nil {
		slog.DebugContext(ctx, "Supervisor: attempting LLM intent classification", "query", query)
		intent, err := s.classifyWithLLM(ctx, query)
		if err != nil {
			slog.WarnContext(ctx, "Supervisor: LLM intent failed, using heuristics", "error", err)
		} else {
			slog.DebugContext(ctx, "Supervisor: LLM intent result", "intent", intent)
			return intent
		}
	}

	return s.classifyWithHeuristics(ctx, query)
}

func (s *SupervisorAgent) classifyWithLLM(ctx context.Context, query string) (IntentType, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	systemPrompt := `You are an intent classifier. Reply with ONLY one word from this list: query, insight, plan, communicate, monitor, general

Examples:
- "Show me sales by region" → query
- "What are the top selling products?" → query
- "Why did margins drop in East India?" → insight
- "Analyze the sales trend" → insight
- "Propose a promotional plan" → plan
- "What actions should I take?" → plan
- "Draft an email to the supplier" → communicate
- "Check for anomalies" → monitor
- "Is everything healthy?" → monitor
- "Hello, what can you do?" → general`
	userPrompt := "Classify this request (reply with ONE word only): " + query

	// Intent only needs one word — cap tokens to avoid the model rambling
	resp, err := s.intentLLM.WithMaxTokens(20).Generate(timeoutCtx, systemPrompt, userPrompt)
	if err != nil {
		return IntentGeneral, err
	}

	// Strip any punctuation/extra chars the model may add
	cleaned := strings.TrimSpace(strings.ToLower(resp))
	cleaned = strings.Trim(cleaned, ".,!?\"'`→:-")
	// Extract the first word in case the model added explanation
	if idx := strings.IndexAny(cleaned, " \t\n"); idx > 0 {
		cleaned = cleaned[:idx]
	}

	switch cleaned {
	case "query":
		return IntentQuery, nil
	case "insight":
		return IntentInsight, nil
	case "plan":
		return IntentPlan, nil
	case "communicate":
		return IntentCommunicate, nil
	case "monitor":
		return IntentMonitor, nil
	case "general":
		return IntentGeneral, nil
	default:
		return IntentGeneral, fmt.Errorf("unrecognized LLM intent response: %q", resp)
	}
}

func (s *SupervisorAgent) classifyWithHeuristics(_ context.Context, query string) IntentType {
	lower := strings.ToLower(query)

	// Greeting patterns
	greetings := []string{"hello", "hi ", "hi!", "hey", "good morning", "good afternoon", "help", "what can you do"}
	for _, g := range greetings {
		if strings.Contains(lower, g) || strings.HasPrefix(lower, g) {
			return IntentGeneral
		}
	}

	// Monitoring patterns — keep specific to avoid false positives on "alert", "status"
	monitorPatterns := []string{"anomal", "watchdog", "monitor", "system health", "health check", "check system", "system status"}
	for _, p := range monitorPatterns {
		if strings.Contains(lower, p) {
			return IntentMonitor
		}
	}

	// Communication patterns
	commPatterns := []string{"draft", "email", "communicate", "send message", "report to", "notify", "compliance"}
	for _, p := range commPatterns {
		if strings.Contains(lower, p) {
			return IntentCommunicate
		}
	}

	// Planning/Action patterns
	planPatterns := []string{"plan", "propose", "what should", "recommend", "action", "fix", "resolve", "do about", "suggestion", "next step"}
	for _, p := range planPatterns {
		if strings.Contains(lower, p) {
			return IntentPlan
		}
	}

	// Insight patterns (Why/Explain/Analyze)
	insightPatterns := []string{"why", "explain", "analyze", "reason", "root cause", "insight", "how come", "what happened", "trend"}
	for _, p := range insightPatterns {
		if strings.Contains(lower, p) {
			return IntentInsight
		}
	}

	// Default to data query
	return IntentQuery
}
