package v1

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/internal/util"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	"github.com/usememos/memos/server/router/api/v1/agent"
	"github.com/usememos/memos/store"
)

type APIV1Service struct {
	grpc_health_v1.UnimplementedHealthServer

	v1pb.UnimplementedWorkspaceServiceServer
	v1pb.UnimplementedWorkspaceSettingServiceServer
	v1pb.UnimplementedAuthServiceServer
	v1pb.UnimplementedUserServiceServer
	v1pb.UnimplementedMemoServiceServer
	v1pb.UnimplementedResourceServiceServer
	v1pb.UnimplementedShortcutServiceServer
	v1pb.UnimplementedInboxServiceServer
	v1pb.UnimplementedActivityServiceServer
	v1pb.UnimplementedWebhookServiceServer
	v1pb.UnimplementedMarkdownServiceServer
	v1pb.UnimplementedIdentityProviderServiceServer

	Secret  string
	Profile *profile.Profile
	Store   *store.Store

	grpcServer   *grpc.Server
	agentHandler *agent.Handler
}

func NewAPIV1Service(secret string, profile *profile.Profile, store *store.Store, grpcServer *grpc.Server) *APIV1Service {
	grpc.EnableTracing = true

	// Initialize agent service and handler
	agentService := agent.NewService(store, profile)
	agentHandler := agent.NewHandler(agentService, store)

	apiv1Service := &APIV1Service{
		Secret:       secret,
		Profile:      profile,
		Store:        store,
		grpcServer:   grpcServer,
		agentHandler: agentHandler,
	}
	grpc_health_v1.RegisterHealthServer(grpcServer, apiv1Service)
	v1pb.RegisterWorkspaceServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterWorkspaceSettingServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterAuthServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterUserServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterMemoServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterResourceServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterShortcutServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterInboxServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterActivityServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterWebhookServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterMarkdownServiceServer(grpcServer, apiv1Service)
	v1pb.RegisterIdentityProviderServiceServer(grpcServer, apiv1Service)
	reflection.Register(grpcServer)
	return apiv1Service
}

// RegisterGateway registers the gRPC-Gateway with the given Echo instance.
func (s *APIV1Service) RegisterGateway(ctx context.Context, echoServer *echo.Echo) error {
	var target string
	if len(s.Profile.UNIXSock) == 0 {
		target = fmt.Sprintf("%s:%d", s.Profile.Addr, s.Profile.Port)
	} else {
		target = fmt.Sprintf("unix:%s", s.Profile.UNIXSock)
	}
	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
	)
	if err != nil {
		return err
	}

	gwMux := runtime.NewServeMux()
	if err := v1pb.RegisterWorkspaceServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	if err := v1pb.RegisterWorkspaceSettingServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	if err := v1pb.RegisterAuthServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	if err := v1pb.RegisterUserServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	if err := v1pb.RegisterMemoServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	if err := v1pb.RegisterResourceServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	if err := v1pb.RegisterShortcutServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	if err := v1pb.RegisterInboxServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	if err := v1pb.RegisterActivityServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	if err := v1pb.RegisterWebhookServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	if err := v1pb.RegisterMarkdownServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	if err := v1pb.RegisterIdentityProviderServiceHandler(ctx, gwMux, conn); err != nil {
		return err
	}
	gwGroup := echoServer.Group("")
	gwGroup.Use(middleware.CORS())

	// Global CORS middleware for all routes - handles OPTIONS preflight before auth
	echoServer.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{echo.GET, echo.POST, echo.PUT, echo.DELETE, echo.OPTIONS},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	// Register ticket routes directly to Echo group with Auth middleware
	// Register these BEFORE the gRPC-gateway Any wildcard to ensure they take precedence
	ticketGroup := echoServer.Group("/api/v1")
	ticketGroup.Use(s.AuthMiddleware)
	s.RegisterTicketRoutes(ticketGroup)
	s.RegisterNotificationRoutes(ticketGroup)

	// Register agent routes
	s.RegisterAgentRoutes(echoServer)

	handler := echo.WrapHandler(gwMux)
	gwGroup.Any("/api/v1/*", handler)
	gwGroup.Any("/file/*", handler)

	// GRPC web proxy.
	options := []grpcweb.Option{
		grpcweb.WithCorsForRegisteredEndpointsOnly(false),
		grpcweb.WithOriginFunc(func(_ string) bool {
			return true
		}),
	}
	wrappedGrpc := grpcweb.WrapServer(s.grpcServer, options...)
	echoServer.Any("/memos.api.v1.*", echo.WrapHandler(wrappedGrpc))

	// Register SSE notification stream endpoint
	// This uses raw http.Handler to support SSE streaming properly
	echoServer.GET("/api/v1/notifications/stream", func(c echo.Context) error {
		userID, ok := c.Get(getUserIDContextKey()).(int32)
		if !ok {
			return echo.NewHTTPError(http.StatusUnauthorized, "Missing user ID")
		}
		s.NotificationStreamHandler(c.Response().Writer, c.Request(), userID)
		return nil
	}, s.AuthMiddleware)

	return nil
}

// RegisterAgentRoutes registers the agent chat API routes.
func (s *APIV1Service) RegisterAgentRoutes(echoServer *echo.Echo) {
	// Public routes (no auth required) - CORS handled globally
	publicGroup := echoServer.Group("/api/v1/agent")
	publicGroup.POST("/:slug/chat/ext", s.agentHandler.HandleChatExternal)
	publicGroup.GET("/:slug/widget.js", s.agentHandler.HandleWidget) // Legacy - inline JS

	// Widget routes (public, no auth) - CORS handled globally
	// URL format: /widget/:slugGuid/embed.js where slugGuid = "tenant-slug-uuid"
	widgetGroup := echoServer.Group("/widget")
	widgetGroup.GET("/:slugGuid/embed.js", s.agentHandler.HandleWidgetEmbed) // Built bundle
	widgetGroup.GET("/:slugGuid/iframe", s.agentHandler.HandleWidgetIframe)  // iframe HTML

	// Authenticated routes (Memos user auth required)
	authGroup := echoServer.Group("/api/v1/agent")
	authGroup.Use(s.AuthMiddleware)
	authGroup.GET("/:slug/validate", s.agentHandler.HandleValidateTenant)
	authGroup.POST("/:slug/chat/int", s.agentHandler.HandleChatInternal)

	// LLM Config routes (permission-based, requires auth)
	authGroup.GET("/:slug/llm-config", s.agentHandler.HandleGetLLMConfig)
	authGroup.PUT("/:slug/llm-config", s.agentHandler.HandleSetLLMConfig)

	// Permission management routes (requires auth)
	authGroup.GET("/:slug/permissions", s.agentHandler.HandleListPermissions)
	authGroup.POST("/:slug/permissions", s.agentHandler.HandleGrantPermission)
	authGroup.DELETE("/:slug/permissions/:userId", s.agentHandler.HandleRevokePermission)

	// Chat logs routes (requires auth + chat:logs permission)
	authGroup.GET("/:slug/sessions", s.agentHandler.HandleListSessions)
	authGroup.GET("/:slug/sessions/:sessionId", s.agentHandler.HandleGetSession)

	// Simulation routes (requires auth + chat:test permission)
	authGroup.POST("/:slug/simulate", s.agentHandler.HandleStartSimulation)
	authGroup.GET("/:slug/simulate/:sessionId/stream", s.agentHandler.HandleSimulationStream)
	authGroup.POST("/:slug/simulate/:sessionId/control", s.agentHandler.HandleSimulationControl)
	authGroup.GET("/:slug/simulations", s.agentHandler.HandleListSimulations)
	authGroup.GET("/:slug/simulations/:simulationId", s.agentHandler.HandleGetSimulation)

	// Unified conversation history (combines simulations and chat sessions)
	authGroup.GET("/:slug/conversations", s.agentHandler.HandleGetConversations)
	authGroup.GET("/:slug/conversations/:conversationId", s.agentHandler.HandleGetConversation)

	// Script routes (SCRIPT.MD - tenant-level conversation flow)
	authGroup.GET("/:slug/script", s.agentHandler.HandleGetScript)
	authGroup.POST("/:slug/script", s.agentHandler.HandleImportScript)
	authGroup.DELETE("/:slug/script", s.agentHandler.HandleDeleteScript)

	// Analysis routes (transcript benchmark analysis)
	authGroup.POST("/:slug/analyze", s.agentHandler.HandleAnalyzeTranscript)
	authGroup.GET("/:slug/analysis", s.agentHandler.HandleGetAnalysisHistory)

	// Learning memory routes (agent self-improvement)
	authGroup.GET("/:slug/learning", s.agentHandler.HandleGetLearning)
	authGroup.POST("/:slug/learning/apply", s.agentHandler.HandleApplyLearnings) // v2 simplified
	authGroup.POST("/:slug/learning/regenerate", s.agentHandler.HandleRegenerateLearning)
	authGroup.POST("/:slug/learning/approve", s.agentHandler.HandleApproveSuggestion)
	authGroup.POST("/:slug/learning/dismiss", s.agentHandler.HandleDismissSuggestion)
	authGroup.DELETE("/:slug/learning/behaviors/:behaviorId", s.agentHandler.HandleRemoveLearnedBehavior)
	authGroup.POST("/:slug/learning/behaviors/:behaviorId/toggle", s.agentHandler.HandleToggleLearnedBehavior)
	authGroup.DELETE("/:slug/learning", s.agentHandler.HandleClearLearning)

	// User tenants route
	userGroup := echoServer.Group("/api/v1/user")
	userGroup.Use(s.AuthMiddleware)
	userGroup.GET("/tenants", s.agentHandler.HandleGetUserTenants)

	// Admin routes (Memos admin role required)
	adminGroup := echoServer.Group("/api/v1/agent")
	adminGroup.Use(s.AuthMiddleware)
	adminGroup.GET("/tenants", s.agentHandler.HandleListTenants)
	adminGroup.POST("/onboard", s.agentHandler.HandleOnboard)
	adminGroup.GET("/:slug/config", s.agentHandler.HandleGetTenantFullConfig)
	adminGroup.PATCH("/:slug", s.agentHandler.HandleUpdateTenant)
	adminGroup.DELETE("/:slug", s.agentHandler.HandleDeleteTenant)
	adminGroup.POST("/:slug/import", s.agentHandler.HandleImportSingleFile)
	adminGroup.POST("/:slug/reindex", s.agentHandler.HandleReindexTenant)
	adminGroup.GET("/:slug/reindex/status", s.agentHandler.HandleReindexStatus)
	adminGroup.GET("/:slug/export", s.agentHandler.HandleExport)
	adminGroup.POST("/:slug/generate-kb", s.agentHandler.HandleGenerateKB)
	adminGroup.POST("/:slug/generate-policy", s.agentHandler.HandleGeneratePolicy)
	adminGroup.POST("/:slug/format-for-rag", s.agentHandler.HandleFormatForRAG)
	adminGroup.POST("/:slug/processing-options", s.agentHandler.HandleSaveProcessingOptions)
	adminGroup.GET("/:slug/processing-options", s.agentHandler.HandleGetProcessingOptions)
	adminGroup.GET("/:slug/files/:audienceType/:fileType/versions", s.agentHandler.HandleGetFileVersions)
	adminGroup.POST("/:slug/files/:audienceType/:fileType/restore", s.agentHandler.HandleRestoreFileVersion)
	adminGroup.GET("/:slug/source-file", s.agentHandler.HandleGetSourceFileContent)

	// Q&A Pairs routes (admin only)
	adminGroup.POST("/:slug/qa-pairs/generate", s.agentHandler.HandleGenerateQAPairs)
	adminGroup.GET("/:slug/qa-pairs", s.agentHandler.HandleListQAPairs)
	adminGroup.POST("/:slug/qa-pairs", s.agentHandler.HandleCreateQAPair)
	adminGroup.PUT("/:slug/qa-pairs/:id", s.agentHandler.HandleUpdateQAPair)
	adminGroup.DELETE("/:slug/qa-pairs/:id", s.agentHandler.HandleDeleteQAPair)
	adminGroup.POST("/:slug/qa-pairs/:id/test", s.agentHandler.HandleTestQAPair)
	adminGroup.POST("/:slug/qa-pairs/test-all", s.agentHandler.HandleTestAllQAPairs)

	// RAG Search Explorer (per-tenant, admin only)
	adminGroup.POST("/:slug/rag/search", s.agentHandler.HandleRAGSearch)

	// Transcript routes (admin only)
	adminGroup.GET("/:slug/transcripts", s.agentHandler.HandleListTranscripts)
	adminGroup.GET("/:slug/transcripts/:id", s.agentHandler.HandleGetTranscript)
	adminGroup.DELETE("/:slug/transcripts/:id", s.agentHandler.HandleDeleteTranscript)

	// Tenant settings routes (admin only)
	adminGroup.GET("/:slug/settings", s.agentHandler.HandleGetTenantSettings)
	adminGroup.PUT("/:slug/settings", s.agentHandler.HandleUpdateTenantSettings)

	// RAG Stats routes (admin only)
	ragGroup := echoServer.Group("/api/v1/admin/rag")
	ragGroup.Use(s.AuthMiddleware)
	ragGroup.GET("/stats", s.agentHandler.HandleGetRAGStats)
	ragGroup.GET("/tenants/:tenantId", s.agentHandler.HandleGetTenantRAGDetails)
	ragGroup.POST("/search", s.agentHandler.HandleTestRAGSearch)
}

func (s *APIV1Service) AuthMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		accessToken := ""

		// Check header
		authHeader := c.Request().Header.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				accessToken = parts[1]
			}
		} else {
			// Check cookie
			cookie, err := c.Cookie(AccessTokenCookieName)
			if err == nil {
				accessToken = cookie.Value
			}
		}

		if accessToken == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "Missing access token")
		}

		// Validate token
		claims := &ClaimsMessage{}
		_, err := jwt.ParseWithClaims(accessToken, claims, func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Name {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			if kid, ok := t.Header["kid"].(string); ok {
				if kid == KeyID {
					return []byte(s.Secret), nil
				}
			}
			return nil, fmt.Errorf("unexpected kid: %v", t.Header["kid"])
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "Invalid or expired token")
		}

		userID, err := util.ConvertStringToInt32(claims.Subject)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "Invalid token subject")
		}

		// Get user to ensure exists and active
		user, err := s.Store.GetUser(ctx, &store.FindUser{ID: &userID})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get user").SetInternal(err)
		}
		if user == nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
		}
		if user.RowStatus == store.Archived {
			return echo.NewHTTPError(http.StatusUnauthorized, "User is archived")
		}

		// Validate token against DB tokens
		accessTokens, err := s.Store.GetUserAccessTokens(ctx, user.ID)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get user access tokens").SetInternal(err)
		}
		isValid := false
		for _, t := range accessTokens {
			if t.AccessToken == accessToken {
				isValid = true
				break
			}
		}
		if !isValid {
			return echo.NewHTTPError(http.StatusUnauthorized, "Token revoked or invalid")
		}

		c.Set(getUserIDContextKey(), userID)
		return next(c)
	}
}
