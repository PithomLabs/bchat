package profile

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pkg/errors"
)

// Profile is the configuration to start main server.
type Profile struct {
	// Mode can be "prod" or "dev" or "demo"
	Mode string
	// Addr is the binding address for server
	Addr string
	// Port is the binding port for server
	Port int
	// UNIXSock is the IPC binding path. Overrides Addr and Port
	UNIXSock string
	// Data is the data directory
	Data string
	// DSN points to where memos stores its own data
	DSN string
	// Driver is the database driver
	// sqlite, mysql, postgres, cockroach
	Driver string
	// Version is the current version of server
	Version string
	// InstanceURL is the url of your memos instance.
	InstanceURL string
	// OpenRouterAPIKey is the API key for OpenRouter AI chat service
	OpenRouterAPIKey string
	// LLMModel is the model identifier for the LLM (e.g., "openrouter/free")
	LLMModel string
	// EncryptionMasterKey is the master key for encrypting tenant API keys (AES-256)
	EncryptionMasterKey string
}

func (p *Profile) IsDev() bool {
	return p.Mode != "prod"
}

func checkDataDir(dataDir string) (string, error) {
	// Convert to absolute path if relative path is supplied.
	if !filepath.IsAbs(dataDir) {
		relativeDir := filepath.Join(filepath.Dir(os.Args[0]), dataDir)
		absDir, err := filepath.Abs(relativeDir)
		if err != nil {
			return "", err
		}
		dataDir = absDir
	}

	// Trim trailing \ or / in case user supplies
	dataDir = strings.TrimRight(dataDir, "\\/")
	if _, err := os.Stat(dataDir); err != nil {
		return "", errors.Wrapf(err, "unable to access data folder %s", dataDir)
	}
	return dataDir, nil
}

func (p *Profile) Validate() error {
	if p.Mode != "demo" && p.Mode != "dev" && p.Mode != "prod" {
		p.Mode = "demo"
	}

	if p.Mode == "prod" && p.Data == "" {
		if runtime.GOOS == "windows" {
			p.Data = filepath.Join(os.Getenv("ProgramData"), "memos")
			if _, err := os.Stat(p.Data); os.IsNotExist(err) {
				if err := os.MkdirAll(p.Data, 0770); err != nil {
					slog.Error("failed to create data directory", slog.String("data", p.Data), slog.String("error", err.Error()))
					return err
				}
			}
		} else {
			p.Data = "/var/opt/memos"
		}
	}

	dataDir, err := checkDataDir(p.Data)
	if err != nil {
		slog.Error("failed to check dsn", slog.String("data", dataDir), slog.String("error", err.Error()))
		return err
	}

	p.Data = dataDir
	if p.Driver == "sqlite" && p.DSN == "" {
		dbFile := fmt.Sprintf("memos_%s.db", p.Mode)
		p.DSN = filepath.Join(dataDir, dbFile)
	}

	if p.Driver == "postgres" && p.DSN == "" {
		p.DSN = os.Getenv("DATABASE_URL")
		if p.DSN == "" {
			return errors.New("postgres driver requires DSN or DATABASE_URL environment variable")
		}
	}

	if p.Driver == "cockroach" && p.DSN == "" {
		p.DSN = os.Getenv("COCKROACH_DSN")
		if p.DSN == "" {
			return errors.New("cockroach driver requires DSN or COCKROACH_DSN environment variable")
		}
	}

	if p.IsDev() && (p.Driver == "cockroach" || p.Driver == "postgres") && os.Getenv("MEMOS_ALLOW_REMOTE_DSN") != "true" {
		if !isLoopbackDSN(p.DSN) {
			return errors.Errorf("refusing to start in %s mode: DSN host %q is not loopback (localhost/127.0.0.1/::1). Local dev runs must not touch remote databases; set MEMOS_ALLOW_REMOTE_DSN=true to override", p.Mode, dsnHost(p.DSN))
		}
	}

	return nil
}

func isLoopbackDSN(dsn string) bool {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return false
	}
	host := u.Host
	if h, _, err := net.SplitHostPort(u.Host); err == nil {
		host = h
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func dsnHost(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Host == "" {
		return dsn
	}
	return u.Host
}
