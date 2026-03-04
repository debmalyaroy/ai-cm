package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/debmalyaroy/ai-cm/internal/agent"
	"github.com/debmalyaroy/ai-cm/internal/llm"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/graphql-go/graphql"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RegisterGraphQLRoutes registers the /api/graphql endpoint.
func RegisterGraphQLRoutes(rg *gin.RouterGroup, db *pgxpool.Pool, llmClient llm.Client, agentModels map[string]string) {
	supervisor := agent.NewSupervisorAgent(llmClient, db, agentModels)
	schema := buildChatSchema(db, supervisor, llmClient)

	rg.POST("/graphql", func(c *gin.Context) {
		var req struct {
			Query     string                 `json:"query"`
			Variables map[string]interface{} `json:"variables"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		result := graphql.Do(graphql.Params{
			Schema:         schema,
			RequestString:  req.Query,
			VariableValues: req.Variables,
			Context:        c.Request.Context(),
		})

		c.JSON(http.StatusOK, result)
	})
}

func buildChatSchema(db *pgxpool.Pool, supervisor *agent.SupervisorAgent, llmClient llm.Client) graphql.Schema {
	messageType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ChatMessage",
		Fields: graphql.Fields{
			"id":         &graphql.Field{Type: graphql.String},
			"role":       &graphql.Field{Type: graphql.String},
			"content":    &graphql.Field{Type: graphql.String},
			"metadata":   &graphql.Field{Type: graphql.String},
			"created_at": &graphql.Field{Type: graphql.String},
		},
	})

	sessionType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ChatSession",
		Fields: graphql.Fields{
			"id":            &graphql.Field{Type: graphql.String},
			"created_at":    &graphql.Field{Type: graphql.String},
			"first_message": &graphql.Field{Type: graphql.String},
		},
	})

	suggestionType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Suggestion",
		Fields: graphql.Fields{
			"label": &graphql.Field{Type: graphql.String},
			"type":  &graphql.Field{Type: graphql.String},
			"value": &graphql.Field{Type: graphql.String},
		},
	})

	chatResponseType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ChatResponse",
		Fields: graphql.Fields{
			"content":     &graphql.Field{Type: graphql.String},
			"agent_name":  &graphql.Field{Type: graphql.String},
			"session_id":  &graphql.Field{Type: graphql.String},
			"suggestions": &graphql.Field{Type: graphql.NewList(suggestionType)},
		},
	})

	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"chatSessions": &graphql.Field{
				Type:        graphql.NewList(sessionType),
				Description: "Get recent chat sessions (max 10)",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					slog.DebugContext(p.Context, "GraphQL: Fetching chat sessions")
					rows, err := db.Query(p.Context, `
						SELECT s.id, s.created_at,
							COALESCE((SELECT content FROM chat_messages WHERE session_id = s.id AND role = 'user' ORDER BY created_at ASC LIMIT 1), 'New Chat') as first_message
						FROM chat_sessions s
						ORDER BY s.updated_at DESC
						LIMIT 10
					`)
					if err != nil {
						slog.ErrorContext(p.Context, "GraphQL: Failed to fetch chat sessions", "error", err)
						return nil, err
					}
					defer rows.Close()

					var sessions []map[string]interface{}
					for rows.Next() {
						var id, firstMsg string
						var createdAt time.Time
						if err := rows.Scan(&id, &createdAt, &firstMsg); err != nil {
							continue
						}
						sessions = append(sessions, map[string]interface{}{
							"id":            id,
							"created_at":    createdAt.Format(time.RFC3339),
							"first_message": firstMsg,
						})
					}
					if sessions == nil {
						sessions = []map[string]interface{}{}
					}
					return sessions, nil
				},
			},
			"chatMessages": &graphql.Field{
				Type:        graphql.NewList(messageType),
				Description: "Get messages for a session",
				Args: graphql.FieldConfigArgument{
					"sessionId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					sessionID := p.Args["sessionId"].(string)
					slog.DebugContext(p.Context, "GraphQL: Fetching chat messages", "session_id", sessionID)
					rows, err := db.Query(p.Context, `
						SELECT id, role, content, COALESCE(metadata, '{}'), created_at
						FROM chat_messages
						WHERE session_id = $1
						ORDER BY created_at ASC
					`, sessionID)
					if err != nil {
						slog.ErrorContext(p.Context, "GraphQL: Failed to fetch chat messages", "error", err, "session_id", sessionID)
						return nil, err
					}
					defer rows.Close()

					var messages []map[string]interface{}
					for rows.Next() {
						var id, role, content string
						var metadata json.RawMessage
						var createdAt time.Time
						if err := rows.Scan(&id, &role, &content, &metadata, &createdAt); err != nil {
							continue
						}
						messages = append(messages, map[string]interface{}{
							"id":         id,
							"role":       role,
							"content":    content,
							"metadata":   string(metadata),
							"created_at": createdAt.Format(time.RFC3339),
						})
					}
					if messages == nil {
						messages = []map[string]interface{}{}
					}
					return messages, nil
				},
			},
		},
	})

	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"createSession": &graphql.Field{
				Type:        sessionType,
				Description: "Create a new chat session",
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					id := uuid.New().String()
					now := time.Now()
					slog.DebugContext(p.Context, "GraphQL: Creating session")
					_, err := db.Exec(p.Context, "INSERT INTO chat_sessions (id) VALUES ($1)", id)
					if err != nil {
						slog.ErrorContext(p.Context, "GraphQL: Failed to create session", "error", err)
						return nil, fmt.Errorf("create session: %w", err)
					}
					slog.InfoContext(p.Context, "GraphQL: Session created", "session_id", id)
					return map[string]interface{}{
						"id":            id,
						"created_at":    now.Format(time.RFC3339),
						"first_message": "New Chat",
					}, nil
				},
			},
			"deleteSession": &graphql.Field{
				Type:        graphql.Boolean,
				Description: "Delete a chat session and its messages",
				Args: graphql.FieldConfigArgument{
					"sessionId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					sessionID := p.Args["sessionId"].(string)
					slog.DebugContext(p.Context, "GraphQL: Deleting session", "session_id", sessionID)
					db.Exec(p.Context, "DELETE FROM chat_messages WHERE session_id = $1", sessionID)
					db.Exec(p.Context, "DELETE FROM chat_sessions WHERE id = $1", sessionID)
					slog.InfoContext(p.Context, "GraphQL: Session deleted", "session_id", sessionID)
					return true, nil
				},
			},
			"sendMessage": &graphql.Field{
				Type:        chatResponseType,
				Description: "Send a message and get a response (non-streaming)",
				Args: graphql.FieldConfigArgument{
					"message":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"sessionId": &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					message := p.Args["message"].(string)
					sessionID, _ := p.Args["sessionId"].(string)

					slog.DebugContext(p.Context, "GraphQL: Processing message", "session_id", sessionID)

					if sessionID == "" {
						sessionID = uuid.New().String()
						db.Exec(p.Context, "INSERT INTO chat_sessions (id) VALUES ($1)", sessionID)
					}

					db.Exec(p.Context, "INSERT INTO chat_messages (session_id, role, content) VALUES ($1, $2, $3)",
						sessionID, "user", message)

					var history []agent.Message
					rows, err := db.Query(p.Context, `
						SELECT role, content FROM chat_messages
						WHERE session_id = $1
						ORDER BY created_at DESC LIMIT 10
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

					output, err := supervisor.Process(p.Context, &agent.Input{
						Query:     message,
						SessionID: sessionID,
						History:   history,
					})
					if err != nil {
						slog.ErrorContext(p.Context, "GraphQL: Message processing failed", "error", err, "session_id", sessionID)
						return nil, err
					}

					db.Exec(p.Context, "INSERT INTO chat_messages (session_id, role, content, metadata) VALUES ($1, $2, $3, $4)",
						sessionID, "assistant", output.Response, fmt.Sprintf(`{"agent": "%s"}`, output.AgentName))

					// Store explicitly in agent Episodic memory
					go func(storeCtx context.Context, sid, q, r, agentName string) {
						if err := supervisor.StoreEpisodicMemory(storeCtx, sid, q, r, agentName); err != nil {
							slog.WarnContext(storeCtx, "GraphQL: Failed to store episodic memory", "error", err)
						}
					}(context.Background(), sessionID, message, output.Response, output.AgentName)

					// Generate follow-up suggestions with guaranteed fallback.
					// generateSuggestions uses its own 30s background context.
					suggestions := generateSuggestions(output.AgentName, message, output.Response, llmClient)

					slog.InfoContext(p.Context, "GraphQL: Message processed successfully", "agent", output.AgentName)
					return map[string]interface{}{
						"content":     output.Response,
						"agent_name":  output.AgentName,
						"session_id":  sessionID,
						"suggestions": suggestions,
					}, nil
				},
			},
		},
	})

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	})
	if err != nil {
		slog.Error("failed to create GraphQL schema", "error", err)
		panic(err)
	}
	return schema
}
