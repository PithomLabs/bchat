// Command s3probe reproduces the tenant-5 `ListObjectsV2` 403 failure documented in
// bugs/026/s3_error.md, using the credentials/endpoint the reindex job uses on Fly.io
// (via a .env file mirroring the app's env-var wiring).
//
// Local run (explicit Fly-secret keys from .env):
//
//	# Create bugs/026/s3_probe/.env with your Fly secrets:
//	#   AWS_ACCESS_KEY_ID=...
//	#   AWS_SECRET_ACCESS_KEY=...
//	#   LANCEDB_S3_ENDPOINT=t3.storage.dev
//	#   LANCEDB_S3_BUCKET=bchat
//	#   LANCEDB_S3_REGION=auto
//	go run . --tenant 5
//
// To reproduce the PRODUCTION failure (which came from Fly's Tigris IAM role, not the
// explicit .env keys), run on the Fly machine WITHOUT explicit keys / .env:
//
//	fly ssh console -C "/tmp/s3probe-linux --tenant 5 --skip-env"
//
// The verdict distinguishes error codes (InvalidAccessKeyId vs AccessDenied), because
// they have different root causes. See bugs/026/claude.md for the scope theory.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func run() error {
	// --- Flags ---
	envFile := flag.String("env-file", ".env", "path to .env file with Fly secrets (skipped if absent)")
	skipEnv := flag.Bool("skip-env", false, "do not load .env; rely solely on the process environment / default credential chain (use on Fly to exercise the IAM-role path)")
	tenant := flag.Int("tenant", 5, "tenant id used in the list prefix")
	prefixBase := flag.String("prefix", "lancedb", "base prefix before the tenant id")
	pathStyle := flag.Bool("path-style", true, "use path-style addressing (reproduces the observed t3.storage.dev/bchat request)")
	maxKeys := flag.Int("max-keys", 5, "MaxKeys for the list call")
	flag.Parse()

	// Load .env (explicit env vars already set in the shell take precedence).
	// On Fly, pass --skip-env so a stray .env can't inject explicit keys and mask
	// the Tigris IAM role that production actually uses.
	if !*skipEnv {
		if err := loadDotEnv(*envFile); err != nil {
			return err
		}
	}

	// --- Env (mirrors server/router/api/v1/agent/vectordb.go NewVectorDBConfigFromEnv) ---
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	endpoint := getEnvOrDefault("LANCEDB_S3_ENDPOINT", "")
	if endpoint == "" {
		endpoint = getEnvOrDefault("AWS_ENDPOINT_URL_S3", "t3.storage.dev")
	}
	bucket := getEnvOrDefault("LANCEDB_S3_BUCKET", "bchat")
	region := getEnvOrDefault("LANCEDB_S3_REGION", "auto")
	forcePathStyleEnv := getEnvOrDefault("LANCEDB_S3_FORCE_PATH_STYLE", "false") == "true"

	_ = forcePathStyleEnv // env kept for parity; flag default true per plan

	fullPrefix := fmt.Sprintf("%s/%d/", *prefixBase, *tenant)

	// Ensure endpoint has a scheme for BaseEndpoint.
	baseEndpoint := endpoint
	if !strings.HasPrefix(baseEndpoint, "http://") && !strings.HasPrefix(baseEndpoint, "https://") {
		baseEndpoint = "https://" + baseEndpoint
	}

	fmt.Printf("S3 probe config:\n")
	fmt.Printf("  endpoint      : %s\n", baseEndpoint)
	fmt.Printf("  region        : %s\n", region)
	fmt.Printf("  bucket        : %s\n", bucket)
	fmt.Printf("  prefix        : %s\n", fullPrefix)
	fmt.Printf("  path-style    : %v\n", *pathStyle)
	fmt.Printf("  has-creds     : %v\n", accessKey != "" && secretKey != "")
	fmt.Printf("  expected URL  : %s/%s?list-type=2&prefix=%s\n",
		strings.TrimRight(baseEndpoint, "/"), bucket, fullPrefix)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Build aws.Config.
	cfgOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if accessKey != "" && secretKey != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		))
	} else {
		fmt.Println("\nWARNING: no AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY set; SDK will use default credential chain.")
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return fmt.Errorf("failed to load aws config: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(baseEndpoint)
		o.UsePathStyle = *pathStyle
	})

	fmt.Println("\nCalling ListObjectsV2 ...")
	out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		Prefix:  aws.String(fullPrefix),
		MaxKeys: aws.Int32(int32(*maxKeys)),
	})

	if err != nil {
		status, code, msg, body := extractError(err)
		fmt.Printf("\nLIST FAILED\n")
		fmt.Printf("  http_status : %d\n", status)
		fmt.Printf("  api_code    : %s\n", code)
		if msg != "" {
			fmt.Printf("  api_message : %s\n", msg)
		}
		if body != "" {
			fmt.Printf("  response    : %s\n", body)
		}
		fmt.Printf("  error       : %v\n", err)
		switch {
		case code == "InvalidAccessKeyId" || code == "InvalidSecretKey":
			fmt.Println("\nVERDICT: CREDENTIAL INVALID - key/secret unknown to Tigris.")
			fmt.Println("  This is NOT a permission/scope issue. The access key ID does not exist")
			fmt.Println("  in Tigris's records (wrong account, rotated, deleted, or typo'd). The local")
			fmt.Println("  .env credentials differ from what production used, so this does NOT reproduce")
			fmt.Println("  the production 403. Verify the key, or run on Fly with --skip-env to use the")
			fmt.Println("  IAM role production actually uses.")
		case code == "AccessDenied":
			fmt.Println("\nVERDICT: CONFIRMED SCOPE ISSUE - valid key but denied s3:ListBucket.")
			fmt.Println("  Matches bugs/026/claude.md: a prefix-scoped token can Get/PutObject under its")
			fmt.Println("  prefix but lacks ListBucket. Fix the Tigris token scope (grant s3:ListBucket on")
			fmt.Println("  bucket 'bchat' with a 'lancedb/*' prefix condition) - not app code.")
		case status != 0:
			fmt.Printf("\nVERDICT: Unexpected status %d (code %q) - investigate separately.\n", status, code)
		default:
			fmt.Println("\nVERDICT: No HTTP response captured (failed before request) - investigate separately.")
		}
		return nil
	}

	fmt.Printf("\nLIST OK (http 200)\n")
	fmt.Printf("  is_truncated : %v\n", aws.ToBool(out.IsTruncated))
	fmt.Printf("  key_count    : %d\n", len(out.Contents))
	for i, obj := range out.Contents {
		if i >= *maxKeys {
			break
		}
		fmt.Printf("    - %s\n", aws.ToString(obj.Key))
	}
	fmt.Println("\nVERDICT: LIST SUCCEEDED with these credentials.")
	fmt.Println("  The app's 403 must come from a DIFFERENT credential set/context than these")
	fmt.Println("  Fly secrets (e.g. the IAM-role path on Fly). Inspect which secrets the reindex")
	fmt.Println("  job actually loads in vectordb_lance.go.")
	return nil
}

// extractError pulls the HTTP status code, the S3 API error code/message, and the
// raw response body out of an SDK error, so we can print the Tigris XML payload
// verbatim and classify the failure precisely (InvalidAccessKeyId vs AccessDenied, ...).
func extractError(err error) (status int, code string, message string, body string) {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code = apiErr.ErrorCode()
		message = apiErr.ErrorMessage()
	}
	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) {
		status = respErr.HTTPStatusCode()
		if respErr.Response != nil && respErr.Response.Body != nil {
			if b, readErr := io.ReadAll(respErr.Response.Body); readErr == nil {
				body = strings.TrimSpace(string(b))
			}
			_ = respErr.Response.Body.Close()
		}
	}
	return status, code, message, body
}

// loadDotEnv reads a simple .env file (KEY=VALUE lines; # comments and blank
// lines ignored; optional single/double quotes stripped). Values are only set
// when the key is not already present in the environment, so explicit shell
// exports take precedence. A missing file is not an error.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer f.Close()

	abs, _ := filepath.Abs(path)
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			return fmt.Errorf("%s:%d: invalid line (expected KEY=VALUE): %q", abs, lineNo, line)
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = strings.Trim(val, `"'`)
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, val); err != nil {
				return fmt.Errorf("failed to set %s: %w", key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read %s: %w", abs, err)
	}
	return nil
}
