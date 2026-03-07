package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/memory"
	"github.com/debmalyaroy/ai-cm/internal/prompts"
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
		// mem is passed to agents so they can independently query/store memory.
		// Analyst uses it for the vector SQL cache (L2) on top of its in-process L1 cache.
		analyst:    NewAnalystAgent(analystLLM, db, tools, mem),
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
		output.Reasoning = append(output.Reasoning, ReasoningStep{
			Type:    "action",
			Content: "Delegating to Analyst for data, then Strategist for insight",
		})
		slog.DebugContext(ctx, "Supervisor: delegating intent processing", "intent", intent, "agent", "analyst+strategist")
		if err := s.runInsightPipeline(ctx, input, output); err != nil {
			return nil, err
		}

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
		if isGreeting(input.Query) {
			output.Response = "Hello! I'm your **AI Category Manager Copilot**.\n\nI can help you with:\n- **Data Queries**: \"Show me sales by region\" or \"What's the top selling product?\"\n- **Insights & Analysis**: \"Why did margin drop in East India?\" or \"Compare MamyPoko vs competitors\"\n- **Actions & Plans**: \"Propose a promotional plan for diapers\"\n- **Communications**: \"Draft a compliance alert for seller\"\n- **Monitoring**: \"Check for anomalies\" or \"System health\"\n\nWhat would you like to know?"
		} else {
			// Ambiguous query with business content — route through Analyst + Strategist
			slog.InfoContext(ctx, "Supervisor: ambiguous query routed to insight pipeline", "query", input.Query)
			if err := s.runInsightPipeline(ctx, input, output); err != nil {
				return nil, err
			}
		}

	default:
		// Unknown intent — route to Insight pipeline as safe default
		slog.WarnContext(ctx, "Supervisor: unknown intent, defaulting to insight pipeline", "intent", intent)
		if err := s.runInsightPipeline(ctx, input, output); err != nil {
			return nil, err
		}
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

// runInsightPipeline runs Analyst then Strategist and merges results into output.
// Used for IntentInsight, IntentGeneral (non-greeting), and unknown intents.
func (s *SupervisorAgent) runInsightPipeline(ctx context.Context, input *Input, output *Output) error {
	strategistInput := &Input{
		Query:         input.Query,
		SessionID:     input.SessionID,
		Context:       make(map[string]any),
		History:       input.History,
		MemoryContext: input.MemoryContext,
	}
	analystOutput, err := s.analyst.Process(ctx, input)
	if err != nil {
		slog.WarnContext(ctx, "Analyst failed in insight pipeline, falling back to Strategist only", "error", err)
	} else if analystOutput != nil {
		strategistInput.Context["analyst_data"] = analystOutput.Data
	}
	strategistOutput, err := s.strategist.Process(ctx, strategistInput)
	if err != nil {
		return fmt.Errorf("strategist error: %w", err)
	}
	output.Response = strategistOutput.Response
	output.AgentName = strategistOutput.AgentName
	if analystOutput != nil {
		output.Data = analystOutput.Data
	}
	output.Reasoning = append(output.Reasoning, strategistOutput.Reasoning...)
	return nil
}

func (s *SupervisorAgent) classifyIntent(ctx context.Context, query string) IntentType {
	// Classify only the user's own question — strip any appended context blocks
	// (e.g. "[Context from previous response: ...]" added by chat.go for follow-ups).
	// Classifying the full enriched string causes the LLM to pick up words from the
	// previous assistant response and misclassify the intent.
	classifyQuery := query
	if idx := strings.Index(query, "\n\n[Context"); idx > 0 {
		classifyQuery = strings.TrimSpace(query[:idx])
	}

	// Attempt LLM-based classification first if intentLLM is configured
	if s.intentLLM != nil {
		slog.DebugContext(ctx, "Supervisor: attempting LLM intent classification", "query", classifyQuery)
		intent, err := s.classifyWithLLM(ctx, classifyQuery)
		if err != nil {
			slog.WarnContext(ctx, "Supervisor: LLM intent failed, using heuristics", "error", err)
		} else {
			slog.DebugContext(ctx, "Supervisor: LLM intent result", "intent", intent)
			return intent
		}
	}

	return s.classifyWithHeuristics(ctx, classifyQuery)
}

func (s *SupervisorAgent) classifyWithLLM(ctx context.Context, query string) (IntentType, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	systemPrompt := prompts.Get("supervisor_intent.md")
	if systemPrompt == "" {
		return IntentGeneral, fmt.Errorf("supervisor_intent.md prompt not found")
	}
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
		// Only accept "monitor" from LLM if the query actually looks like a monitoring request.
		// LLMs sometimes mislabel analytical follow-up questions as "monitor" due to words
		// like "underperformance", "concerns", "gaps" in the surrounding text.
		if isMonitoringQuery(query) {
			return IntentMonitor, nil
		}
		// Fall back to heuristics — they will likely classify as query or insight
		return IntentGeneral, fmt.Errorf("LLM returned monitor for non-monitoring query, falling back to heuristics")
	case "general":
		// Only accept "general" from LLM if the query really looks like a greeting.
		if isGreeting(query) {
			return IntentGeneral, nil
		}
		return IntentGeneral, fmt.Errorf("LLM returned general for non-greeting, falling back to heuristics")
	default:
		return IntentGeneral, fmt.Errorf("unrecognized LLM intent response: %q", resp)
	}
}

func (s *SupervisorAgent) classifyWithHeuristics(_ context.Context, query string) IntentType {
	lower := strings.ToLower(query)

	// Pure greeting check — must come first but only for very short/simple greetings
	if isGreeting(query) {
		return IntentGeneral
	}

	// Communication patterns
	commPatterns := []string{"draft", "email", "communicate", "send message", "report to", "notify", "compliance"}
	for _, p := range commPatterns {
		if strings.Contains(lower, p) {
			return IntentCommunicate
		}
	}

	// Planning/Action patterns — checked before monitor so "create alert" routes to plan, not monitor
	planPatterns := []string{
		// Recommendation / planning
		"plan", "propose", "what should", "recommend", "do about", "next step", "prioriti",
		// Directive action-creation verbs (retail operations)
		"create a ", "create an ", "create the ",  // "create a replenishment order", "create a promotion"
		"create action", "create an action", "create alert",
		"replenish", "restock", "reorder",          // inventory replenishment
		"place an order", "raise a", "raise an",    // PO/ticket creation
		"schedule", "launch", "initiate",           // "launch a promotion", "schedule a restock"
		"submit for approval", "approve",
		// Adjustment directives
		"implement", "execute the", "carry out",
		"increase stock", "reduce price", "boost", "fix", "resolve",
		"adjust pric", "update pric", "set price",  // "adjust pricing", "update price for X"
		"change the price", "modify price",
	}
	for _, p := range planPatterns {
		if strings.Contains(lower, p) {
			return IntentPlan
		}
	}

	// Monitoring patterns — after plan so "create alert" is caught above, not here
	monitorPatterns := []string{"anomal", "watchdog", "monitor", "system health", "health check", "check system", "system status", "alert"}
	for _, p := range monitorPatterns {
		if strings.Contains(lower, p) {
			return IntentMonitor
		}
	}

	// Insight patterns — analytical reasoning, comparisons, forecasts, explanations
	insightPatterns := []string{
		"why", "explain", "analyze", "analyse", "analysis", "reason", "root cause",
		"insight", "how come", "what happened", "trend", "forecast", "predict",
		"compare", "comparison", "versus", "vs ", " vs.", "against",
		"performance", "evaluate", "assessment", "diagnose",
		"adjust the forecast", "based on",
		"profit margin", "margin analysis", "revenue analysis",
	}
	for _, p := range insightPatterns {
		if strings.Contains(lower, p) {
			return IntentInsight
		}
	}

	// Default to data query — any remaining business question is treated as a data retrieval
	return IntentQuery
}

// isMonitoringQuery returns true only if the query is genuinely about system monitoring,
// anomaly detection, or health checks — not a business analysis question that
// happens to use words like "underperformance" or "concerns".
func isMonitoringQuery(query string) bool {
	lower := strings.TrimSpace(strings.ToLower(query))
	monitorKeywords := []string{
		"anomal", "watchdog", "monitor", "system health", "health check",
		"check system", "system status", "alert", "anomaly detection",
	}
	for _, kw := range monitorKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// isGreeting returns true only if the query is a short, greeting-style message
// with no meaningful business content.
func isGreeting(query string) bool {
	lower := strings.TrimSpace(strings.ToLower(query))

	// Must be short — long queries are almost never pure greetings
	if len(lower) > 80 {
		return false
	}

	greetingPhrases := []string{
		"hello", "hi", "hey", "good morning", "good afternoon", "good evening",
		"what can you do", "how can you help", "what do you do",
		"who are you", "introduce yourself",
	}
	for _, g := range greetingPhrases {
		if strings.Contains(lower, g) {
			return true
		}
	}
	return false
}
