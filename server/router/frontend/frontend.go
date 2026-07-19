package frontend

import (
	"context"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/internal/util"
	"github.com/usememos/memos/store"
)

//go:generate npm --prefix ../../../web install
//go:generate npm --prefix ../../../web run release
//go:embed dist/*
var embeddedFiles embed.FS

type FrontendService struct {
	Profile *profile.Profile
	Store   *store.Store
}

func NewFrontendService(profile *profile.Profile, store *store.Store) *FrontendService {
	return &FrontendService{
		Profile: profile,
		Store:   store,
	}
}

func (*FrontendService) Serve(_ context.Context, e *echo.Echo) {
	distFS := getFileSystem("dist")

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqPath := c.Request().URL.Path

			// Skip API routes — let the next middleware handle them.
			if util.HasPrefixes(reqPath, "/api", "/memos.api.v1", "/widget/") {
				return next(c)
			}

			// Serve file from embedded FS.
			filePath := strings.TrimPrefix(reqPath, "/")
			if filePath == "" {
				filePath = "index.html"
			}

			file, err := distFS.Open(filePath)
			if err != nil {
				// File not found — decide: asset 404 or SPA fallback.
				if isAssetRequest(reqPath) {
					return echo.NewHTTPError(http.StatusNotFound, "Asset not found")
				}
				// SPA fallback: serve index.html for navigation requests.
				return serveIndex(c, distFS)
			}
			defer file.Close()

			stat, err := file.Stat()
			if err != nil || stat.IsDir() {
				// Directory or stat error — try SPA fallback for non-asset paths.
				if isAssetRequest(reqPath) {
					return echo.NewHTTPError(http.StatusNotFound, "Asset not found")
				}
				return serveIndex(c, distFS)
			}

			// Determine cache strategy.
			isHashed := isHashedAsset(filePath)
			setCacheHeaders(c, filePath, isHashed)

			// Content-Type from file extension.
			contentType := mime.TypeByExtension(path.Ext(filePath))
			if contentType == "" {
				contentType = "application/octet-stream"
			}
			c.Response().Header().Set(echo.HeaderContentType, contentType)
			c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(stat.Size(), 10))

			http.ServeContent(c.Response(), c.Request(), filePath, stat.ModTime(), file)
			return nil
		}
	})
}

// serveIndex serves index.html with no-cache headers.
func serveIndex(c echo.Context, fs http.FileSystem) error {
	file, err := fs.Open("index.html")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to load index.html")
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to stat index.html")
	}

	c.Response().Header().Set(echo.HeaderContentType, "text/html; charset=utf-8")
	c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(stat.Size(), 10))

	http.ServeContent(c.Response(), c.Request(), "index.html", stat.ModTime(), file)
	return nil
}

// setCacheHeaders sets Cache-Control based on asset type.
func setCacheHeaders(c echo.Context, filePath string, immutable bool) {
	switch {
	case filePath == "index.html":
		c.Response().Header().Set(echo.HeaderCacheControl, "no-cache")
	case immutable:
		// Vite-hashed assets: cache for 7 days, immutable.
		c.Response().Header().Set(echo.HeaderCacheControl, "public, max-age=604800, immutable")
	default:
		// Non-hashed static files (favicon, logo, etc.).
		c.Response().Header().Set(echo.HeaderCacheControl, "public, max-age=3600")
	}
}

// isAssetRequest returns true for paths that have a known static-file extension.
// Missing assets must 404 — never fall back to index.html for these.
func isAssetRequest(p string) bool {
	assetExts := []string{
		".js", ".css", ".mjs",
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", ".avif",
		".woff", ".woff2", ".ttf", ".eot", ".otf",
		".map", ".json", ".wasm",
	}
	lower := strings.ToLower(p)
	for _, ext := range assetExts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// isHashedAsset returns true for Vite-generated assets that include a content hash.
// Example: "assets/app.CU7C7QZX.js" or "assets/mui-vendor.CJePl92-.js"
func isHashedAsset(filePath string) bool {
	if !strings.HasPrefix(filePath, "assets/") {
		return false
	}
	base := path.Base(filePath)
	// Vite hash format: name.HASH.ext where HASH is ~8 chars of [A-Za-z0-9_-]
	name := strings.TrimSuffix(base, path.Ext(base))
	parts := strings.SplitN(name, ".", 2)
	if len(parts) != 2 {
		return false
	}
	hash := parts[1]
	if len(hash) < 4 || len(hash) > 12 {
		return false
	}
	for _, c := range hash {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func getFileSystem(p string) http.FileSystem {
	sub, err := fs.Sub(embeddedFiles, p)
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}
