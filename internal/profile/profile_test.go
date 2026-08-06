package profile

import (
	"strings"
	"testing"
)

func TestValidateDevModeRemoteDSNGuard(t *testing.T) {
	const remoteDSN = "postgresql://bchat_user:secret@great-goat-30894.j77.cockroachlabs.cloud:26257/bchat?sslmode=verify-full"

	cases := []struct {
		name      string
		mode      string
		driver    string
		dsn       string
		allowEnv  string
		wantError bool
		wantHost  string
	}{
		{
			name:   "dev cockroach loopback localhost",
			mode:   "dev",
			driver: "cockroach",
			dsn:    "postgresql://root@localhost:26257/bchat_test?sslmode=disable",
		},
		{
			name:   "dev postgres loopback 127.0.0.1",
			mode:   "dev",
			driver: "postgres",
			dsn:    "postgresql://root@127.0.0.1:5432/memos?sslmode=disable",
		},
		{
			name:   "dev cockroach loopback IPv6",
			mode:   "dev",
			driver: "cockroach",
			dsn:    "postgresql://root@[::1]:26257/bchat_test?sslmode=disable",
		},
		{
			name:      "dev cockroach remote rejected",
			mode:      "dev",
			driver:    "cockroach",
			dsn:       remoteDSN,
			wantError: true,
			wantHost:  "great-goat-30894.j77.cockroachlabs.cloud:26257",
		},
		{
			name:      "default mode remote rejected",
			mode:      "",
			driver:    "cockroach",
			dsn:       remoteDSN,
			wantError: true,
			wantHost:  "great-goat-30894.j77.cockroachlabs.cloud:26257",
		},
		{
			name:      "dev postgres remote rejected",
			mode:      "dev",
			driver:    "postgres",
			dsn:       "postgresql://user:pass@neon.tech.example:5432/memos?sslmode=require",
			wantError: true,
			wantHost:  "neon.tech.example:5432",
		},
		{
			name:     "opt-out allows remote",
			mode:     "dev",
			driver:   "cockroach",
			dsn:      remoteDSN,
			allowEnv: "true",
		},
		{
			name:      "unparseable DSN rejected",
			mode:      "dev",
			driver:    "cockroach",
			dsn:       "not a url",
			wantError: true,
		},
		{
			name:   "prod allows remote",
			mode:   "prod",
			driver: "cockroach",
			dsn:    remoteDSN,
		},
		{
			name:   "sqlite unaffected",
			mode:   "dev",
			driver: "sqlite",
			dsn:    "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MEMOS_ALLOW_REMOTE_DSN", tc.allowEnv)
			p := &Profile{
				Mode:   tc.mode,
				Driver: tc.driver,
				DSN:    tc.dsn,
				Data:   t.TempDir(),
			}
			err := p.Validate()
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tc.wantHost != "" && !strings.Contains(err.Error(), tc.wantHost) {
					t.Fatalf("error %q does not mention host %q", err.Error(), tc.wantHost)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
