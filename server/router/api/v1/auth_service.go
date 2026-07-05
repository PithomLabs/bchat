package v1

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/usememos/memos/internal/base"
	"github.com/usememos/memos/internal/util"
	"github.com/usememos/memos/plugin/idp"
	"github.com/usememos/memos/plugin/idp/oauth2"
	v1pb "github.com/usememos/memos/proto/gen/api/v1"
	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

const (
	unmatchedUsernameAndPasswordError = "unmatched username and password"
)

func (s *APIV1Service) GetAuthStatus(ctx context.Context, _ *v1pb.GetAuthStatusRequest) (*v1pb.User, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "failed to get current user: %v", err)
	}
	if user == nil {
		// Set the cookie header to expire access token.
		if err := s.clearAccessTokenCookie(ctx); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to set grpc header: %v", err)
		}
		return nil, status.Errorf(codes.Unauthenticated, "user not found")
	}
	return convertUserFromStore(user), nil
}

func (s *APIV1Service) SignIn(ctx context.Context, request *v1pb.SignInRequest) (*v1pb.User, error) {
	var existingUser *store.User
	if passwordCredentials := request.GetPasswordCredentials(); passwordCredentials != nil {
		user, err := s.Store.GetUser(ctx, &store.FindUser{
			Username: &passwordCredentials.Username,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get user, error: %v", err)
		}
		if user == nil {
			return nil, status.Errorf(codes.InvalidArgument, unmatchedUsernameAndPasswordError)
		}
		// Compare the stored hashed password, with the hashed version of the password that was received.
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(passwordCredentials.Password)); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, unmatchedUsernameAndPasswordError)
		}
		workspaceGeneralSetting, err := s.Store.GetWorkspaceGeneralSetting(ctx)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get workspace general setting, error: %v", err)
		}
		// Check if the password auth in is allowed.
		if workspaceGeneralSetting.DisallowPasswordAuth && user.Role == store.RoleUser {
			return nil, status.Errorf(codes.PermissionDenied, "password signin is not allowed")
		}
		existingUser = user
	} else if ssoCredentials := request.GetSsoCredentials(); ssoCredentials != nil {
		identityProvider, err := s.Store.GetIdentityProvider(ctx, &store.FindIdentityProvider{
			ID: &ssoCredentials.IdpId,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get identity provider, error: %v", err)
		}
		if identityProvider == nil {
			return nil, status.Errorf(codes.InvalidArgument, "identity provider not found")
		}

		var userInfo *idp.IdentityProviderUserInfo
		if identityProvider.Type == storepb.IdentityProvider_OAUTH2 {
			oauth2IdentityProvider, err := oauth2.NewIdentityProvider(identityProvider.Config.GetOauth2Config())
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create oauth2 identity provider, error: %v", err)
			}
			token, err := oauth2IdentityProvider.ExchangeToken(ctx, ssoCredentials.RedirectUri, ssoCredentials.Code)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to exchange token, error: %v", err)
			}
			userInfo, err = oauth2IdentityProvider.UserInfo(token)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get user info, error: %v", err)
			}
		}

		identifierFilter := identityProvider.IdentifierFilter
		if identifierFilter != "" {
			identifierFilterRegex, err := regexp.Compile(identifierFilter)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to compile identifier filter regex, error: %v", err)
			}
			if !identifierFilterRegex.MatchString(userInfo.Identifier) {
				return nil, status.Errorf(codes.PermissionDenied, "identifier %s is not allowed", userInfo.Identifier)
			}
		}

		user, err := s.Store.GetUser(ctx, &store.FindUser{
			Username: &userInfo.Identifier,
		})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to get user, error: %v", err)
		}
		if user == nil {
			// Check if the user is allowed to sign up.
			workspaceGeneralSetting, err := s.Store.GetWorkspaceGeneralSetting(ctx)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to get workspace general setting, error: %v", err)
			}
			if workspaceGeneralSetting.DisallowUserRegistration {
				return nil, status.Errorf(codes.PermissionDenied, "user registration is not allowed")
			}

			// Create a new user with the user info from the identity provider.
			userCreate := &store.User{
				Username: userInfo.Identifier,
				// The new signup user should be normal user by default.
				Role:      store.RoleUser,
				Nickname:  userInfo.DisplayName,
				Email:     userInfo.Email,
				AvatarURL: userInfo.AvatarURL,
			}
			password, err := util.RandomString(20)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to generate random password, error: %v", err)
			}
			passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to generate password hash, error: %v", err)
			}
			userCreate.PasswordHash = string(passwordHash)
			user, err = s.Store.CreateUser(ctx, userCreate)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create user, error: %v", err)
			}
		}
		existingUser = user
	}

	if existingUser == nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid credentials")
	}
	if existingUser.RowStatus == store.Archived {
		return nil, status.Errorf(codes.PermissionDenied, "user has been archived with username %s", existingUser.Username)
	}

	expireTime := time.Now().Add(AccessTokenDuration)
	if request.NeverExpire {
		// Cap "never expire" tokens at MaxNeverExpireDuration (30 days).
		// Previously this was 100 years, which posed a security risk.
		expireTime = time.Now().Add(MaxNeverExpireDuration)
	}
	if err := s.doSignIn(ctx, existingUser, nil, expireTime); err != nil {
		return nil, err
	}
	return convertUserFromStore(existingUser), nil
}

func (s *APIV1Service) doSignIn(ctx context.Context, user *store.User, tenantID *int32, expireTime time.Time) error {
	// External users MUST have a company association to log in.
	if user.Role == store.RoleUser {
		perms, err := s.Store.ListUserTenantPermissions(ctx, &store.FindUserTenantPermission{UserID: &user.ID})
		if err != nil {
			return status.Errorf(codes.Internal, "failed to verify user company association")
		}
		if len(perms) == 0 {
			return status.Errorf(codes.PermissionDenied, "user is not associated with any company")
		}
		// Auto-select single tenant if not already specified
		if tenantID == nil && len(perms) == 1 {
			tenantID = &perms[0].TenantID
		} else if tenantID == nil && len(perms) > 1 {
			return status.Errorf(codes.FailedPrecondition, "multiple tenants found, use /auth/tenants endpoint")
		}
	}

	accessToken, err := GenerateAccessToken(user.Email, user.ID, tenantID, expireTime, []byte(s.Secret))
	if err != nil {
		return status.Errorf(codes.Internal, "failed to generate access token, error: %v", err)
	}
	if err := s.UpsertAccessTokenToStore(ctx, user, accessToken, "user login"); err != nil {
		return status.Errorf(codes.Internal, "failed to upsert access token to store, error: %v", err)
	}

	cookie, err := s.buildAccessTokenCookie(ctx, accessToken, expireTime)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to build access token cookie, error: %v", err)
	}
	if err := grpc.SetHeader(ctx, metadata.New(map[string]string{
		"Set-Cookie": cookie,
	})); err != nil {
		return status.Errorf(codes.Internal, "failed to set grpc header, error: %v", err)
	}

	return nil
}

func (s *APIV1Service) SignUp(ctx context.Context, request *v1pb.SignUpRequest) (*v1pb.User, error) {
	workspaceGeneralSetting, err := s.Store.GetWorkspaceGeneralSetting(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get workspace general setting, error: %v", err)
	}
	if workspaceGeneralSetting.DisallowUserRegistration {
		return nil, status.Errorf(codes.PermissionDenied, "sign up is not allowed")
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate password hash, error: %v", err)
	}

	create := &store.User{
		Username:     request.Username,
		Nickname:     request.Username,
		PasswordHash: string(passwordHash),
	}
	if !base.UIDMatcher.MatchString(strings.ToLower(create.Username)) {
		return nil, status.Errorf(codes.InvalidArgument, "invalid username: %s", create.Username)
	}

	hostUserType := store.RoleHost
	existedHostUsers, err := s.Store.ListUsers(ctx, &store.FindUser{
		Role: &hostUserType,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list host users, error: %v", err)
	}
	if len(existedHostUsers) == 0 {
		// Change the default role to host if there is no host user.
		create.Role = store.RoleHost
	} else {
		create.Role = store.RoleUser
	}

	user, err := s.Store.CreateUser(ctx, create)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create user, error: %v", err)
	}

	if err := s.doSignIn(ctx, user, nil, time.Now().Add(AccessTokenDuration)); err != nil {
		return nil, err
	}
	return convertUserFromStore(user), nil
}

func (s *APIV1Service) SignOut(ctx context.Context, _ *v1pb.SignOutRequest) (*emptypb.Empty, error) {
	accessToken, ok := ctx.Value(accessTokenContextKey).(string)
	// Try to delete the access token from the store.
	if ok {
		user, _ := s.GetCurrentUser(ctx)
		if user != nil {
			if _, err := s.DeleteUserAccessToken(ctx, &v1pb.DeleteUserAccessTokenRequest{
				Name:        fmt.Sprintf("%s%d", UserNamePrefix, user.ID),
				AccessToken: accessToken,
			}); err != nil {
				slog.Error("failed to delete access token", "error", err)
			}
		}
	}

	if err := s.clearAccessTokenCookie(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to set grpc header, error: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *APIV1Service) clearAccessTokenCookie(ctx context.Context) error {
	cookie, err := s.buildAccessTokenCookie(ctx, "", time.Time{})
	if err != nil {
		return errors.Wrap(err, "failed to build access token cookie")
	}
	if err := grpc.SetHeader(ctx, metadata.New(map[string]string{
		"Set-Cookie": cookie,
	})); err != nil {
		return errors.Wrap(err, "failed to set grpc header")
	}
	return nil
}

func (*APIV1Service) buildAccessTokenCookie(ctx context.Context, accessToken string, expireTime time.Time) (string, error) {
	attrs := []string{
		fmt.Sprintf("%s=%s", AccessTokenCookieName, accessToken),
		"Path=/",
		"HttpOnly",
	}
	if expireTime.IsZero() {
		attrs = append(attrs, "Expires=Thu, 01 Jan 1970 00:00:00 GMT")
	} else {
		attrs = append(attrs, "Expires="+expireTime.Format(time.RFC1123))
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errors.New("failed to get metadata from context")
	}
	var origin string
	for _, v := range md.Get("origin") {
		origin = v
	}
	isHTTPS := strings.HasPrefix(origin, "https://")
	if isHTTPS {
		attrs = append(attrs, "SameSite=None")
		attrs = append(attrs, "Secure")
	} else {
		attrs = append(attrs, "SameSite=Strict")
	}
	return strings.Join(attrs, "; "), nil
}

func (s *APIV1Service) GetCurrentUser(ctx context.Context) (*store.User, error) {
	username, ok := ctx.Value(usernameContextKey).(string)
	if !ok {
		return nil, nil
	}
	user, err := s.Store.GetUser(ctx, &store.FindUser{
		Username: &username,
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

// ============================================================================
// REST Endpoints for Tenant Selection (multi-tenant sign-in flow)
// ============================================================================

// TenantInfo represents a tenant in the selection response.
type TenantInfo struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// AuthTenantsResponse is the response for POST /api/v1/auth/tenants.
type AuthTenantsResponse struct {
	Tenants       []TenantInfo `json:"tenants"`
	SelectionToken string      `json:"selection_token"`
}

// SelectTenantRequest is the request for POST /api/v1/auth/select-tenant.
type SelectTenantRequest struct {
	SelectionToken string `json:"selection_token"`
	TenantID       int32  `json:"tenant_id"`
}

// HandleAuthTenants handles POST /api/v1/auth/tenants.
// Validates credentials and returns available tenants + selection token.
func (s *APIV1Service) HandleAuthTenants(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Validate credentials
	user, err := s.Store.GetUser(c.Request().Context(), &store.FindUser{
		Username: &req.Username,
	})
	if err != nil || user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
	}
	if user.RowStatus == store.Archived {
		return echo.NewHTTPError(http.StatusForbidden, "user is archived")
	}

	// Check if password auth is allowed
	workspaceGeneralSetting, err := s.Store.GetWorkspaceGeneralSetting(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get workspace settings")
	}
	if workspaceGeneralSetting.DisallowPasswordAuth && user.Role == store.RoleUser {
		return echo.NewHTTPError(http.StatusForbidden, "password signin is not allowed")
	}

	// Get tenant permissions
	perms, err := s.Store.ListUserTenantPermissions(c.Request().Context(), &store.FindUserTenantPermission{UserID: &user.ID})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tenant permissions")
	}
	if len(perms) == 0 {
		return echo.NewHTTPError(http.StatusForbidden, "user is not associated with any company")
	}

	// Build tenant list
	tenants := make([]TenantInfo, 0, len(perms))
	for _, perm := range perms {
		tenant, err := s.Store.GetAgentTenant(c.Request().Context(), &store.FindAgentTenant{ID: &perm.TenantID})
		if err != nil || tenant == nil {
			continue
		}
		tenants = append(tenants, TenantInfo{
			ID:   tenant.ID,
			Name: tenant.CompanyName,
			Slug: tenant.Slug,
		})
	}

	// Generate selection token (random string)
	selectionToken, err := util.RandomString(32)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate selection token")
	}

	// Store selection token with timestamp in description for 5-min expiry enforcement
	tokenTimestamp := time.Now().Unix()
	selectionDescription := fmt.Sprintf("tenant-selection-token:%d", tokenTimestamp)
	if err := s.UpsertAccessTokenToStore(c.Request().Context(), user, "selection:"+selectionToken, selectionDescription); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store selection token")
	}

	return c.JSON(http.StatusOK, AuthTenantsResponse{
		Tenants:        tenants,
		SelectionToken: selectionToken,
	})
}

// HandleSelectTenant handles POST /api/v1/auth/select-tenant.
// Validates selection token and returns full JWT with tenant_id.
func (s *APIV1Service) HandleSelectTenant(c echo.Context) error {
	var req SelectTenantRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.SelectionToken == "" || req.TenantID == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "selection_token and tenant_id are required")
	}

	// Find user by selection token
	// The selection token is stored as "selection:<token>" in the access token
	// We need to find which user owns this token
	ctx := c.Request().Context()
	users, err := s.Store.ListUsers(ctx, &store.FindUser{})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list users")
	}

	var matchedUser *store.User
	var tokenCreatedAt time.Time
	for _, user := range users {
		tokens, err := s.Store.GetUserAccessTokens(ctx, user.ID)
		if err != nil {
			continue
		}
		for _, token := range tokens {
			if token.AccessToken == "selection:"+req.SelectionToken {
				matchedUser = user
				// Parse timestamp from description
				if _, err := fmt.Sscanf(token.Description, "tenant-selection-token:%d", &tokenCreatedAt); err == nil {
					tokenCreatedAt = time.Unix(tokenCreatedAt.Unix(), 0)
				}
				break
			}
		}
		if matchedUser != nil {
			break
		}
	}

	if matchedUser == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired selection token")
	}

	// Check if selection token was created within 5 minutes
	if time.Since(tokenCreatedAt) > 5*time.Minute {
		// Token expired, remove it
		_ = s.Store.RemoveUserAccessToken(ctx, matchedUser.ID, "selection:"+req.SelectionToken)
		return echo.NewHTTPError(http.StatusUnauthorized, "selection token expired, please sign in again")
	}

	// Verify user has access to the target tenant
	perm, err := s.Store.GetUserTenantPermission(ctx, &store.FindUserTenantPermission{
		UserID:   &matchedUser.ID,
		TenantID: &req.TenantID,
	})
	if err != nil || perm == nil {
		return echo.NewHTTPError(http.StatusForbidden, "user does not have access to this tenant")
	}

	// Delete the selection token (single-use)
	if err := s.Store.RemoveUserAccessToken(ctx, matchedUser.ID, "selection:"+req.SelectionToken); err != nil {
		slog.Warn("failed to delete selection token", "error", err)
	}

	// Generate full JWT with tenant_id
	expireTime := time.Now().Add(AccessTokenDuration)
	accessToken, err := GenerateAccessToken(matchedUser.Email, matchedUser.ID, &req.TenantID, expireTime, []byte(s.Secret))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate access token")
	}
	if err := s.UpsertAccessTokenToStore(ctx, matchedUser, accessToken, "tenant-selection"); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store access token")
	}

	// Set cookie
	cookie, err := s.buildAccessTokenCookie(ctx, accessToken, expireTime)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to build cookie")
	}
	c.SetCookie(&http.Cookie{
		Name:     AccessTokenCookieName,
		Value:    accessToken,
		Path:     "/",
		HttpOnly: true,
		Expires:  expireTime,
	})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"access_token": accessToken,
		"cookie":       cookie,
		"tenant_id":    req.TenantID,
	})
}
