package v1

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/revrost/go-openrouter"
)

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest represents the incoming chat request.
type ChatRequest struct {
	Messages []ChatMessage `json:"messages"`
}

// ChatResponse represents the response from the AI.
type ChatResponse struct {
	Message ChatMessage `json:"message"`
}

// RegisterChatRoutes registers the chat-related HTTP routes.
func (s *APIV1Service) RegisterChatRoutes(g *echo.Group) {
	g.POST("/chat", s.HandleChat)
}

// HandleChat processes incoming chat requests and returns AI responses.
func (s *APIV1Service) HandleChat(c echo.Context) error {
	ctx := c.Request().Context()

	// Verify authenticated user
	userID, ok := c.Get(getUserIDContextKey()).(int32)
	if !ok {
		slog.Warn("HandleChat: missing user ID in context")
		return echo.NewHTTPError(http.StatusUnauthorized, "Authentication required")
	}
	slog.Info("HandleChat: processing request", "userID", userID)

	// Check if OpenRouter API key is configured
	if s.Profile.OpenRouterAPIKey == "" {
		slog.Error("HandleChat: OPENROUTER_API_KEY not configured")
		return echo.NewHTTPError(http.StatusServiceUnavailable, "AI chat service not configured")
	}

	// Check if LLM model is configured
	model := s.Profile.LLMModel
	if model == "" {
		model = "openai/gpt-4o-mini" // Default fallback
	}

	// Bind request
	request := &ChatRequest{}
	if err := c.Bind(request); err != nil {
		slog.Error("HandleChat: failed to bind request", "error", err)
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	if len(request.Messages) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Messages required")
	}

	// Call OpenRouter API
	response, err := s.callOpenRouter(ctx, request.Messages, model)
	if err != nil {
		slog.Error("HandleChat: OpenRouter API error", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get AI response")
	}

	slog.Info("HandleChat: successfully generated response", "userID", userID)
	return c.JSON(http.StatusOK, ChatResponse{
		Message: ChatMessage{
			Role:    "assistant",
			Content: response,
		},
	})
}

// callOpenRouter makes a request to the OpenRouter API.
func (s *APIV1Service) callOpenRouter(ctx context.Context, messages []ChatMessage, model string) (string, error) {
	client := openrouter.NewClient(s.Profile.OpenRouterAPIKey)

	// Build messages with system prompt
	openRouterMessages := []openrouter.ChatCompletionMessage{
		openrouter.SystemMessage("You are a helpful assistant for the Memos application. Help users with their questions about notes, tasks, and general inquiries. Be concise and helpful."),
	}

	// Add conversation history
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			openRouterMessages = append(openRouterMessages, openrouter.UserMessage(msg.Content))
		case "assistant":
			openRouterMessages = append(openRouterMessages, openrouter.AssistantMessage(msg.Content))
		}
	}

	// Make API request
	resp, err := client.CreateChatCompletion(ctx, openrouter.ChatCompletionRequest{
		Model:    model,
		Messages: openRouterMessages,
	})
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "No response from AI")
	}

	return resp.Choices[0].Message.Content.Text, nil
}
