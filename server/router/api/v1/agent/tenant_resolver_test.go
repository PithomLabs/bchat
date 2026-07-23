package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/usememos/memos/store"
	teststore "github.com/usememos/memos/store/test"
)

func setupResolverTestStore(t *testing.T, ctx context.Context, slug string, active bool) (*store.Store, *store.AgentTenant) {
	ts := teststore.NewTestingStore(ctx, t)

	tenant, err := ts.CreateAgentTenant(ctx, &store.AgentTenant{
		Slug:        slug,
		CompanyName: slug,
		Vertical:    "test",
		IsActive:    active,
	})
	require.NoError(t, err)

	return ts, tenant
}

func TestResolveSlugTenantMiddleware_ValidTenant(t *testing.T) {
	ctx := context.Background()
	ts, expectedTenant := setupResolverTestStore(t, ctx, "test-slug", true)
	defer ts.Close()

	e := echo.New()
	middleware := ResolveSlugTenantMiddleware(ts)

	var capturedTenantID int32
	handler := middleware(func(c echo.Context) error {
		capturedTenantID = c.Get(tenantContextKey).(int32)
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("test-slug")

	err := handler(c)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, expectedTenant.ID, capturedTenantID)
}

func TestResolveSlugTenantMiddleware_TenantNotFound(t *testing.T) {
	ctx := context.Background()
	ts, _ := setupResolverTestStore(t, ctx, "existing-slug", true)
	defer ts.Close()

	e := echo.New()
	middleware := ResolveSlugTenantMiddleware(ts)

	handler := middleware(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("nonexistent-slug")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusNotFound, echoErr.Code)
}

func TestResolveSlugTenantMiddleware_InactiveTenant(t *testing.T) {
	ctx := context.Background()
	ts, _ := setupResolverTestStore(t, ctx, "inactive-slug", false)
	defer ts.Close()

	e := echo.New()
	middleware := ResolveSlugTenantMiddleware(ts)

	handler := middleware(func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("slug")
	c.SetParamValues("inactive-slug")

	err := handler(c)
	require.Error(t, err)
	var echoErr *echo.HTTPError
	require.True(t, errors.As(err, &echoErr))
	require.Equal(t, http.StatusNotFound, echoErr.Code)
}

func TestResolveSlugTenantMiddleware_EmptySlugSkipsResolution(t *testing.T) {
	ctx := context.Background()
	ts, _ := setupResolverTestStore(t, ctx, "any-slug", true)
	defer ts.Close()

	e := echo.New()
	middleware := ResolveSlugTenantMiddleware(ts)

	nextCalled := false
	handler := middleware(func(c echo.Context) error {
		nextCalled = true
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/playground/catalog", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// No slug param set — simulates routes like /playground/catalog

	err := handler(c)
	require.NoError(t, err)
	require.True(t, nextCalled)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Nil(t, c.Get(tenantContextKey))
}
