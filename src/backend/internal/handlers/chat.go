package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/agent"
	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/debmalyaroy/ai-cm/internal/prompts"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterChatRoutes registers chat API routes.
func RegisterChatRoutes(rg *gin.RouterGroup, db *pgxpool.Pool, llmClient llm.Client, agentModels map[string]string) {
	supervisor := agent.NewSupervisorAgent(llmClient, db, agentModels)

	rg.POST("/chat", handleChat(db, supervisor, llmClient))
	rg.POST("/chat/sessions", createChatSession(db))
	rg.GET("/chat/sessions", getChatSessions(db))
	rg.GET("/chat/sessions/:id/messages", getChatMessages(db))
	rg.DELETE("/chat/sessions/:id", deleteChatSession(db, supervisor))

	// Admin tool: Force reloading LLM system prompts from disk
	rg.POST("/prompts/reload", reloadPrompts())
}

// reloadPrompts handles forcing the internal prompts cache to reload from disk.
func reloadPrompts() gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := prompts.Reload(); err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to reload prompts via admin tool", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload prompts: " + err.Error()})
			return
		}
		slog.InfoContext(c.Request.Context(), "Prompts reloaded successfully via admin tool")
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "System prompts successfully reloaded from disk"})
	}
}

type ChatRequest struct {
	Message    string `json:"message" binding:"required"`
	SessionID  string `json:"session_id"`
	ContextMsg string `json:"context_msg,omitempty"`
}

// @Summary Handle chat messages
// @Description Process user message through Supervisor Agent and stream response via SSE
// @Tags chat
// @Accept json
// @Produce text/event-stream
// @Param request body ChatRequest true "Chat Message"
// @Success 200 {string} string "SSE Stream"
// @Failure 400 {object} map[string]string
// @Router /chat [post]
func handleChat(db *pgxpool.Pool, supervisor *agent.SupervisorAgent, llmClient llm.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
			return
		}

		// Create or use existing session — use the request context for fast DB ops
		reqCtx := c.Request.Context()
		sessionID := req.SessionID
		if sessionID == "" {
			sessionID = uuid.New().String()
			_, err := db.Exec(reqCtx, "INSERT INTO chat_sessions (id) VALUES ($1)", sessionID)
			if err != nil {
				slog.WarnContext(reqCtx, "failed to create session (non-fatal)", "error", err)
			}
		}

		slog.DebugContext(reqCtx, "Chat: processing message", "session_id", sessionID, "message_len", len(req.Message))

		// Store user message and update session timestamp
		if _, err := db.Exec(reqCtx, "INSERT INTO chat_messages (session_id, role, content) VALUES ($1, $2, $3)",
			sessionID, "user", req.Message); err != nil {
			slog.WarnContext(reqCtx, "failed to store user message", "error", err)
		}
		if _, err := db.Exec(reqCtx, "UPDATE chat_sessions SET updated_at = NOW() WHERE id = $1", sessionID); err != nil {
			slog.WarnContext(reqCtx, "failed to update session timestamp", "error", err)
		}

		// Get chat history for context (ASC order so oldest first = correct conversation flow)
		var history []agent.Message
		rows, err := db.Query(reqCtx, `
			SELECT role, content FROM chat_messages
			WHERE session_id = $1
			ORDER BY created_at ASC LIMIT 10
		`, sessionID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var msg agent.Message
				if err := rows.Scan(&msg.Role, &msg.Content); err == nil {
					history = append(history, msg)
				}
			}
		}
		slog.DebugContext(reqCtx, "Chat: retrieved history", "message_count", len(history))

		// Set up SSE streaming
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Session-ID", sessionID)

		// Send session ID event
		fmt.Fprintf(c.Writer, "event: session\ndata: %s\n\n", sessionID)
		c.Writer.Flush()

		// Process through Supervisor Agent.
		// Use a detached context that copies the request's values (including request-ID
		// for structured logging) but is NOT cancelled when the HTTP server fires its
		// write-timeout. This prevents long LLM calls from being aborted mid-flight.
		processCtx, processCancel := context.WithTimeout(
			context.WithoutCancel(reqCtx),
			240*time.Second,
		)
		defer processCancel()

		input := &agent.Input{
			Query:     req.Message,
			SessionID: sessionID,
			History:   history,
		}

		if req.ContextMsg != "" {
			// Append the context to the query specifically so the Analyst or Strategist agent has historical tie
			input.Query = fmt.Sprintf("%s\n\n[Context from previous response: %s]", req.Message, req.ContextMsg)
			slog.DebugContext(reqCtx, "Chat: appended follow-up context to query")
		}

		sendSSE(c.Writer, "status", map[string]string{"status": "thinking", "message": "Analyzing your question..."})

		output, err := supervisor.Process(processCtx, input)
		if err != nil {
			slog.ErrorContext(reqCtx, "Supervisor processing failed", "error", err, "session_id", sessionID)
			sendSSE(c.Writer, "error", map[string]string{"error": "I encountered an error processing your request. Please try again."})
			sendSSE(c.Writer, "suggestions", map[string]interface{}{"questions": fallbackSuggestions("supervisor")})
			sendSSE(c.Writer, "done", map[string]string{"session_id": sessionID})
			return
		}

		slog.DebugContext(reqCtx, "Chat: supervisor processing complete", "agent", output.AgentName, "response_len", len(output.Response))

		// Stream reasoning steps
		for _, step := range output.Reasoning {
			sendSSE(c.Writer, "reasoning", step)
			time.Sleep(100 * time.Millisecond)
		}

		// Send data if available
		if output.Data != nil {
			sendSSE(c.Writer, "data", output.Data)
		}

		// Send final response (with confidence score and data source for transparency)
		sendSSE(c.Writer, "response", map[string]any{
			"content":          output.Response,
			"agent_name":       output.AgentName,
			"confidence_score": output.ConfidenceScore,
			"data_source":      output.DataSource,
		})

		// Store episodic memory in background (non-blocking)
		go func(sid, q, r, agentName string) {
			if err := supervisor.StoreEpisodicMemory(context.Background(), sid, q, r, agentName); err != nil {
				slog.Warn("Failed to store episodic memory", "error", err)
			}
		}(sessionID, req.Message, output.Response, output.AgentName)

		// Generate follow-up suggestions based on the full response (sequential — depends on response content)
		suggestions := generateSuggestions(output.AgentName, req.Message, output.Response, llmClient)
		sendSSE(c.Writer, "suggestions", map[string]interface{}{"questions": suggestions})

		suggestionsBytes, _ := json.Marshal(suggestions)
		metadataJSON := fmt.Sprintf(`{"agent": "%s", "suggestions": %s}`, output.AgentName, string(suggestionsBytes))

		// Store assistant message and update session timestamp
		if _, err := db.Exec(reqCtx, "INSERT INTO chat_messages (session_id, role, content, metadata) VALUES ($1, $2, $3, $4)",
			sessionID, "assistant", output.Response, metadataJSON); err != nil {
			slog.WarnContext(reqCtx, "failed to store assistant message", "error", err)
		}
		if _, err := db.Exec(reqCtx, "UPDATE chat_sessions SET updated_at = NOW() WHERE id = $1", sessionID); err != nil {
			slog.WarnContext(reqCtx, "failed to update session timestamp", "error", err)
		}

		// Send done event
		sendSSE(c.Writer, "done", map[string]string{"session_id": sessionID})
	}
}

func sendSSE(w io.Writer, event string, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		slog.Error("SSE marshal error", "error", err)
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, string(jsonData))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// @Summary Create a new chat session
// @Description Create a new chat session and return its ID
// @Tags chat
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /chat/sessions [post]
func createChatSession(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := uuid.New().String()
		_, err := db.Exec(c.Request.Context(), "INSERT INTO chat_sessions (id) VALUES ($1)", sessionID)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to create chat session", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		slog.DebugContext(c.Request.Context(), "Chat session created", "session_id", sessionID)
		c.JSON(http.StatusOK, gin.H{"session_id": sessionID})
	}
}

// @Summary Delete a chat session
// @Description Delete a chat session and all its messages
// @Tags chat
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /chat/sessions/{id} [delete]
func deleteChatSession(db *pgxpool.Pool, supervisor *agent.SupervisorAgent) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("id")
		reqCtx := c.Request.Context()

		// Collect session summary before deleting — used for episodic memory
		var firstMsg string
		var msgCount int
		if err := db.QueryRow(reqCtx, `
			SELECT
				COALESCE((SELECT content FROM chat_messages WHERE session_id=$1 AND role='user' ORDER BY created_at ASC LIMIT 1), ''),
				(SELECT COUNT(*) FROM chat_messages WHERE session_id=$1)
		`, sessionID, sessionID).Scan(&firstMsg, &msgCount); err != nil {
			slog.WarnContext(reqCtx, "failed to query session summary", "error", err)
		}

		// Store episodic memory of the ended session in background (non-blocking)
		if firstMsg != "" && msgCount > 0 {
			go func() {
				summary := fmt.Sprintf("Session closed after %d messages. Opening question: %s", msgCount, firstMsg)
				if err := supervisor.StoreEpisodicMemory(context.Background(), sessionID, firstMsg, summary, "supervisor"); err != nil {
					slog.Warn("Failed to store session end episodic memory", "error", err, "session_id", sessionID)
				}
			}()
		}

		// CASCADE delete removes chat_messages automatically
		result, err := db.Exec(reqCtx, "DELETE FROM chat_sessions WHERE id = $1", sessionID)
		if err != nil {
			slog.ErrorContext(reqCtx, "Failed to delete chat session", "error", err, "session_id", sessionID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if result.RowsAffected() == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
			return
		}
		slog.DebugContext(reqCtx, "Chat session deleted with episodic memory stored", "session_id", sessionID)
		c.JSON(http.StatusOK, gin.H{"message": "session deleted"})
	}
}

// @Summary Get chat sessions
// @Description Retrieve recent chat sessions with their first message
// @Tags chat
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /chat/sessions [get]
func getChatSessions(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		slog.DebugContext(c.Request.Context(), "Fetching recent chat sessions")
		// Prune sessions older than 30 days (best-effort, non-fatal)
		if _, err := db.Exec(c, `DELETE FROM chat_sessions WHERE created_at < NOW() - INTERVAL '30 days'`); err != nil {
			slog.WarnContext(c.Request.Context(), "failed to prune old sessions", "error", err)
		}

		rows, err := db.Query(c, `
			SELECT s.id, s.updated_at,
				(SELECT content FROM chat_messages WHERE session_id = s.id AND role = 'user' ORDER BY created_at ASC LIMIT 1) as first_message
			FROM chat_sessions s
			WHERE s.created_at >= NOW() - INTERVAL '30 days'
			  AND EXISTS (SELECT 1 FROM chat_messages WHERE session_id = s.id)
			ORDER BY s.updated_at DESC
			LIMIT 10
		`)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to fetch chat sessions", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type Session struct {
			ID           string `json:"id"`
			UpdatedAt    string `json:"updated_at"`
			FirstMessage string `json:"first_message"`
		}

		var sessions []Session
		for rows.Next() {
			var s Session
			var updatedAt time.Time
			var firstMsg *string
			if err := rows.Scan(&s.ID, &updatedAt, &firstMsg); err != nil {
				continue
			}
			s.UpdatedAt = updatedAt.Format(time.RFC3339)
			if firstMsg != nil {
				s.FirstMessage = *firstMsg
			} else {
				s.FirstMessage = "New Chat"
			}
			sessions = append(sessions, s)
		}

		if sessions == nil {
			sessions = []Session{}
		}

		c.JSON(http.StatusOK, sessions)
	}
}

// @Summary Get chat messages for session
// @Description Retrieve message history for a specific chat session
// @Tags chat
// @Produce json
// @Param id path string true "Session ID"
// @Success 200 {array} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /chat/sessions/{id}/messages [get]
func getChatMessages(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Param("id")
		slog.DebugContext(c.Request.Context(), "Fetching chat messages for session", "session_id", sessionID)

		rows, err := db.Query(c, `
			SELECT id, role, content, metadata, created_at
			FROM chat_messages
			WHERE session_id = $1
			ORDER BY created_at ASC
		`, sessionID)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "Failed to fetch chat messages", "error", err, "session_id", sessionID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		type ChatMsg struct {
			ID        string          `json:"id"`
			Role      string          `json:"role"`
			Content   string          `json:"content"`
			Metadata  json.RawMessage `json:"metadata"`
			CreatedAt string          `json:"created_at"`
		}

		var messages []ChatMsg
		for rows.Next() {
			var m ChatMsg
			var createdAt time.Time
			if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.Metadata, &createdAt); err != nil {
				continue
			}
			m.CreatedAt = createdAt.Format(time.RFC3339)
			messages = append(messages, m)
		}

		if messages == nil {
			messages = []ChatMsg{}
		}

		c.JSON(http.StatusOK, messages)
	}
}
