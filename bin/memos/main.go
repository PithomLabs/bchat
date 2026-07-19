package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/usememos/memos/internal/profile"
	"github.com/usememos/memos/internal/version"
	"github.com/usememos/memos/server"
	"github.com/usememos/memos/server/router/api/v1/agent"
	"github.com/usememos/memos/store"
	"github.com/usememos/memos/store/db"
)

const (
	greetingBanner = `
███╗   ███╗███████╗███╗   ███╗ ██████╗ ███████╗
████╗ ████║██╔════╝████╗ ████║██╔═══██╗██╔════╝
██╔████╔██║█████╗  ██╔████╔██║██║   ██║███████╗
██║╚██╔╝██║██╔══╝  ██║╚██╔╝██║██║   ██║╚════██║
██║ ╚═╝ ██║███████╗██║ ╚═╝ ██║╚██████╔╝███████║
╚═╝     ╚═╝╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚══════╝
`
)

var (
	rootCmd = &cobra.Command{
		Use:   "memos",
		Short: `An open source, lightweight note-taking service. Easily capture and share your great thoughts.`,
		Run: func(_ *cobra.Command, _ []string) {
			// Auto-generate encryption key if not set (Issue #5)
			if viper.GetString("encryption-master-key") == "" {
				dataDir := viper.GetString("data")
				if dataDir == "" {
					dataDir = "./build/data"
				}
				encryptionKey := getOrCreateEncryptionKey(dataDir)
				viper.Set("encryption-master-key", encryptionKey)
			}

			instanceProfile := &profile.Profile{
				Mode:                viper.GetString("mode"),
				Addr:                viper.GetString("addr"),
				Port:                viper.GetInt("port"),
				UNIXSock:            viper.GetString("unix-sock"),
				Data:                viper.GetString("data"),
				Driver:              viper.GetString("driver"),
				DSN:                 viper.GetString("dsn"),
				InstanceURL:         viper.GetString("instance-url"),
				Version:             version.GetCurrentVersion(viper.GetString("mode")),
				OpenRouterAPIKey:    viper.GetString("openrouter-api-key"),
				LLMModel:            viper.GetString("llm-model"),
				EncryptionMasterKey: viper.GetString("encryption-master-key"),
			}

			// Issue #10: Validate encryption key strength
			if key := instanceProfile.EncryptionMasterKey; key != "" {
				if len(key) < 16 {
					slog.Warn("ENCRYPTION_MASTER_KEY is too short (< 16 chars). Encrypted tenant API keys may be insecure.",
						"key_length", len(key))
				}
		} else {
			slog.Warn("ENCRYPTION_MASTER_KEY is not set. Tenant API key encryption is disabled.")
		}

		// Log OpenRouter API key status at startup
		if instanceProfile.OpenRouterAPIKey != "" {
			slog.Info("OpenRouter API key loaded", "prefix", instanceProfile.OpenRouterAPIKey[:10]+"...")
		} else {
			slog.Warn("OpenRouter API key is NOT set - chat will be unavailable")
		}

		if err := instanceProfile.Validate(); err != nil {
				panic(err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			dbDriver, err := db.NewDBDriver(instanceProfile)
			if err != nil {
				cancel()
				slog.Error("failed to create db driver", "error", err)
				return
			}

			storeInstance := store.New(dbDriver, instanceProfile)
			if err := storeInstance.Migrate(ctx); err != nil {
				cancel()
				slog.Error("failed to migrate", "error", err)
				return
			}

			s, err := server.NewServer(ctx, instanceProfile, storeInstance)
			if err != nil {
				cancel()
				slog.Error("failed to create server", "error", err)
				return
			}

			c := make(chan os.Signal, 1)
			// Trigger graceful shutdown on SIGINT or SIGTERM.
			// The default signal sent by the `kill` command is SIGTERM,
			// which is taken as the graceful shutdown signal for many systems, eg., Kubernetes, Gunicorn.
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)

			if err := s.Start(ctx); err != nil {
				if err != http.ErrServerClosed {
					slog.Error("failed to start server", "error", err)
					cancel()
				}
			}

			printGreetings(instanceProfile)

			go func() {
				<-c
				s.Shutdown(ctx)
				cancel()
			}()

			// Wait for CTRL-C.
			<-ctx.Done()
		},
	}
)

var rotateKeysCmd = &cobra.Command{
	Use:   "rotate-keys",
	Short: "Re-encrypt all secrets with the current master key (requires ENCRYPTION_MASTER_KEY_BACKUP)",
	RunE: func(cmd *cobra.Command, args []string) error {
		primaryKey := viper.GetString("encryption-master-key")
		backupKey := os.Getenv("ENCRYPTION_MASTER_KEY_BACKUP")
		if primaryKey == "" {
			return fmt.Errorf("ENCRYPTION_MASTER_KEY not set")
		}
		if backupKey == "" {
			return fmt.Errorf("ENCRYPTION_MASTER_KEY_BACKUP not set (nothing to re-encrypt from)")
		}
		if primaryKey == backupKey {
			return fmt.Errorf("primary and backup keys are identical — nothing to rotate")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		dbDriver, err := db.NewDBDriver(&profile.Profile{
			Mode:                viper.GetString("mode"),
			Data:                viper.GetString("data"),
			Driver:              viper.GetString("driver"),
			DSN:                 viper.GetString("dsn"),
			EncryptionMasterKey: primaryKey,
		})
		if err != nil {
			return fmt.Errorf("failed to create db driver: %w", err)
		}
		storeInstance := store.New(dbDriver, &profile.Profile{
			EncryptionMasterKey: primaryKey,
		})
		if err := storeInstance.Migrate(ctx); err != nil {
			return fmt.Errorf("failed to migrate: %w", err)
		}

		slog.Info("Key rotation started — re-encrypting all secrets...")
		svc := newAgentServiceForRotation(storeInstance, primaryKey)
		if err := svc.ReEncryptOnStartup(ctx); err != nil {
			return err
		}
		return nil
	},
}

func newAgentServiceForRotation(s *store.Store, primaryKey string) *agentServiceForRotation {
	return &agentServiceForRotation{store: s, primaryKey: primaryKey}
}

type agentServiceForRotation struct {
	store      *store.Store
	primaryKey string
}

func (r *agentServiceForRotation) ReEncryptOnStartup(ctx context.Context) error {
	p := &profile.Profile{EncryptionMasterKey: r.primaryKey}
	svc := agent.NewService(r.store, p)
	succeeded, failed, reErr := svc.ReEncryptOnStartup(ctx)
	if reErr != nil {
		slog.Error("key rotation aborted", "error", reErr)
		return fmt.Errorf("key rotation aborted: %w", reErr)
	}
	if failed > 0 {
		slog.Error("key rotation completed with failures", "succeeded", succeeded, "failed", failed)
		return fmt.Errorf("key rotation partially failed: %d of %d secrets not re-encrypted", failed, succeeded+failed)
	}
	slog.Info("Key rotation complete", "re_encrypted", succeeded)
	return nil
}

func init() {
	viper.SetDefault("mode", "dev")
	viper.SetDefault("driver", "sqlite")
	viper.SetDefault("port", 8081)

	rootCmd.PersistentFlags().String("mode", "dev", `mode of server, can be "prod" or "dev" or "demo"`)
	rootCmd.PersistentFlags().String("addr", "", "address of server")
	rootCmd.PersistentFlags().Int("port", 8081, "port of server")
	rootCmd.PersistentFlags().String("unix-sock", "", "path to the unix socket, overrides --addr and --port")
	rootCmd.PersistentFlags().String("data", "", "data directory")
	rootCmd.PersistentFlags().String("driver", "sqlite", "database driver")
	rootCmd.PersistentFlags().String("dsn", "", "database source name(aka. DSN)")
	rootCmd.PersistentFlags().String("instance-url", "", "the url of your memos instance")
	rootCmd.PersistentFlags().String("openrouter-api-key", "", "OpenRouter API key for AI chat")
	rootCmd.PersistentFlags().String("llm-model", "openrouter/free", "LLM model identifier for AI chat")
	rootCmd.PersistentFlags().String("encryption-master-key", "", "Master key for encrypting tenant API keys")

	if err := viper.BindPFlag("mode", rootCmd.PersistentFlags().Lookup("mode")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("addr", rootCmd.PersistentFlags().Lookup("addr")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("unix-sock", rootCmd.PersistentFlags().Lookup("unix-sock")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("data", rootCmd.PersistentFlags().Lookup("data")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("driver", rootCmd.PersistentFlags().Lookup("driver")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("dsn", rootCmd.PersistentFlags().Lookup("dsn")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("instance-url", rootCmd.PersistentFlags().Lookup("instance-url")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("openrouter-api-key", rootCmd.PersistentFlags().Lookup("openrouter-api-key")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("llm-model", rootCmd.PersistentFlags().Lookup("llm-model")); err != nil {
		panic(err)
	}
	if err := viper.BindPFlag("encryption-master-key", rootCmd.PersistentFlags().Lookup("encryption-master-key")); err != nil {
		panic(err)
	}

	viper.SetEnvPrefix("memos")
	viper.AutomaticEnv()
	if err := viper.BindEnv("instance-url", "MEMOS_INSTANCE_URL"); err != nil {
		panic(err)
	}
	if err := viper.BindEnv("openrouter-api-key", "OPENROUTER_API_KEY"); err != nil {
		panic(err)
	}
	if err := viper.BindEnv("llm-model", "LLM_MODEL"); err != nil {
		panic(err)
	}
	if err := viper.BindEnv("encryption-master-key", "ENCRYPTION_MASTER_KEY"); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(rotateKeysCmd)
}

func printGreetings(profile *profile.Profile) {
	if profile.IsDev() {
		println("Development mode is enabled")
		println("DSN: ", profile.DSN)
	}
	fmt.Printf(`---
Server profile
version: %s
data: %s
addr: %s
port: %d
unix-sock: %s
mode: %s
driver: %s
---
`, profile.Version, profile.Data, profile.Addr, profile.Port, profile.UNIXSock, profile.Mode, profile.Driver)

	print(greetingBanner)
	if len(profile.UNIXSock) == 0 {
		if len(profile.Addr) == 0 {
			fmt.Printf("Version %s has been started on port %d\n", profile.Version, profile.Port)
		} else {
			fmt.Printf("Version %s has been started on address '%s' and port %d\n", profile.Version, profile.Addr, profile.Port)
		}
	} else {
		fmt.Printf("Version %s has been started on unix socket %s\n", profile.Version, profile.UNIXSock)
	}
	fmt.Printf(`---
See more in:
👉Website: %s
👉GitHub: %s
---
`, "https://usememos.com", "https://github.com/usememos/memos")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}

// getOrCreateEncryptionKey reads or generates a UUID-based encryption key.
// The key file is single-instance per data dir. We must NEVER fall back to a
// locally generated key when the file already exists but is empty/short (e.g.
// a crash between O_EXCL create and write) or when another instance won the
// O_EXCL race — doing so produces divergent master keys and silent, permanent
// loss of all previously encrypted tenant secrets (M7 / H2).
func getOrCreateEncryptionKey(dataDir string) string {
	keyFile := filepath.Join(dataDir, ".encryption_key")
	if data, err := os.ReadFile(keyFile); err == nil {
		key := strings.TrimSpace(string(data))
		if len(key) >= 16 {
			return key
		}
		// Existing file is corrupt/empty (crashed writer) and we own it (we just
		// read it and decided to regenerate). Heal it so we can claim the slot.
		slog.Warn("existing encryption key file is empty or too short; it will be regenerated", "file", keyFile)
		if rmErr := os.Remove(keyFile); rmErr != nil {
			// If we cannot clear the corrupt file we must not proceed to an O_EXCL
			// that will fail and panic anyway — fail loudly here.
			slog.Error("cannot clear corrupt encryption key file; aborting startup", "file", keyFile, "error", rmErr)
			panic(fmt.Sprintf("cannot clear corrupt encryption key file %s: %v", keyFile, rmErr))
		}
	}

	key := uuid.New().String()
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		slog.Warn("failed to create data directory for encryption key", "error", err)
		return key
	}

	// Retry once in case another instance holds the slot transiently.
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(keyFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			// Another instance won the race — adopt its (valid) key.
			if data, readErr := os.ReadFile(keyFile); readErr == nil {
				if k := strings.TrimSpace(string(data)); len(k) >= 16 {
					return k
				}
			}
			// The peer's file is empty/short because it is still crashing. We must
			// NOT remove it — that would unlink a peer's in-flight key and let us
			// create a DIVERGENT key (F1). Just retry; the peer will finish writing
			// and a later ReadFile will adopt its valid key.
			slog.Warn("encryption key file claimed by a peer is unusable; will retry", "error", err)
			continue
		}
		if _, wErr := f.WriteString(key + "\n"); wErr != nil {
			f.Close()
			// Do not leave a partial/empty file behind.
			_ = os.Remove(keyFile)
			slog.Warn("failed to write encryption key file", "error", wErr)
			return key
		}
		if fErr := f.Close(); fErr != nil {
			slog.Warn("failed to close encryption key file", "error", fErr)
		}
		slog.Info("Generated new encryption key", "file", keyFile)
		return key
	}

	// Last resort: another instance keeps winning with a valid file — adopt it.
	if data, readErr := os.ReadFile(keyFile); readErr == nil {
		if k := strings.TrimSpace(string(data)); len(k) >= 16 {
			return k
		}
	}
	// We could not establish a stable key that matches the on-disk file. Returning
	// a locally generated key here would create divergent master keys across
	// processes and silently, permanently lose tenant secrets. Abort instead.
	slog.Error("unable to establish a stable encryption key file; tenant secret encryption may be inconsistent")
	panic("unable to establish a stable encryption key file; refusing to start with a divergent master key")
}
