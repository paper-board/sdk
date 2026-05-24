# API Reference

Exported symbols per package. Internal (unexported) helpers are omitted.
All import paths are rooted at `github.com/paper-board/sdk`.

______________________________________________________________________

## migrator

```go
import "github.com/paper-board/sdk/migrator"
```

### Config

```go
type Config struct {
    DBURL     string    // MIGRATION_DB_URL — port 5432 direct Postgres, never PgBouncer
    Schema    string    // Postgres schema this service owns, e.g. "identity"
    EmbedFS   embed.FS  // embedded migration filesystem (//go:embed schema/*.sql)
    EmbedRoot string    // directory inside EmbedFS containing .sql files, e.g. "schema"
}
```

### Run

```go
func Run(ctx context.Context, cfg Config, args []string) error
```

Entry point for the migrator binary. Validates `Config`, opens a direct Postgres
connection, acquires an advisory lock scoped to the schema, and dispatches to a
cobra subcommand parsed from `args`.

Subcommands:

| Subcommand      | Effect                                                                     |
| --------------- | -------------------------------------------------------------------------- |
| `up`            | Apply all pending migrations                                               |
| `up --dry-run`  | Print SQL that would run, apply nothing                                    |
| `down N`        | Roll back the last N migrations                                            |
| `force VERSION` | Force-set the schema_migrations version without running SQL (dirty repair) |
| `version`       | Print current and pending migration counts                                 |
| `drop`          | Drop all objects in the schema (dev-only; requires `MIGRATOR_ENV=dev`)     |

### Sentinel errors

```go
var (
    ErrMissingEnv     error // MIGRATION_DB_URL not set
    ErrDirty          error // schema is dirty; run 'force <prev>' after fixing
    ErrLockTimeout    error // advisory lock timed out (another migrator running?)
    ErrDropProduction error // 'drop' attempted outside MIGRATOR_ENV=dev
)
```

Use `errors.Is` to detect specific failures in wrapper scripts or tests.

______________________________________________________________________

## log

```go
import "github.com/paper-board/sdk/log"
```

Wraps stdlib `log/slog`. All helpers use the default slog logger unless a logger
created by `New` is passed explicitly.

### New / SetDefault

```go
func New(w io.Writer, service string, level slog.Level) *slog.Logger
func SetDefault(l *slog.Logger)
```

`New` returns a JSON-format `*slog.Logger` with `service` added as a static
attribute. Call `SetDefault` to make it the process-wide default so `slog.Info`
etc. route through it.

### LevelFromEnv

```go
func LevelFromEnv() slog.Level
```

Reads `LOG_LEVEL` env (`debug` | `info` | `warn` | `error`). Defaults to `info`.

### Context helpers

```go
func WithRequestID(ctx context.Context, id string) context.Context
func WithTraceID(ctx context.Context, id string) context.Context
func WithOrg(ctx context.Context, id string) context.Context
func WithUser(ctx context.Context, id string) context.Context
func WithRoles(ctx context.Context, roles []string) context.Context

func RequestIDFromContext(ctx context.Context) string
func TraceIDFromContext(ctx context.Context) string
func OrgFromContext(ctx context.Context) string
func UserFromContext(ctx context.Context) string
func RolesFromContext(ctx context.Context) []string
```

Store/retrieve structured log fields in a `context.Context`.

### Pkg

```go
func Pkg(ctx context.Context) *slog.Logger
```

Returns the default slog logger with `request_id`, `trace_id`, `org_id`,
`user_id`, and `roles` auto-injected from `ctx`. Use this in every handler
instead of `slog.Default()` directly.

### Error attrs

```go
func ErrorAttrs(err error) []any
func ErrorAttrsAttr(err error) []slog.Attr
```

Decompose an error into slog key-value pairs (`error`, `kind`, `message`).
Pass the result directly to a slog call: `log.Pkg(ctx).Error("failed", log.ErrorAttrs(err)...)`.

### RedactingHandler

```go
type RedactOpts struct {
    DenyKeys []string       // attribute keys to redact unconditionally
    Patterns []*regexp.Regexp // patterns to scrub from string values
}

func DefaultDenyKeys() []string          // ["password", "token", "secret", "authorization", ...]
func DefaultPatterns() []*regexp.Regexp  // credit-card, bearer-token patterns

func NewRedactingHandler(inner slog.Handler, opts RedactOpts) *RedactingHandler
```

Wraps any `slog.Handler`. Replaces matching attribute values with `"[REDACTED]"`.
Compose with `New` to get automatic secret scrubbing:

```go
base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
redacted := log.NewRedactingHandler(base, log.RedactOpts{
    DenyKeys: log.DefaultDenyKeys(),
    Patterns: log.DefaultPatterns(),
})
logger := slog.New(redacted)
```

______________________________________________________________________

## obs

```go
import "github.com/paper-board/sdk/obs"
```

### SetupOTel

```go
type Shutdown func(context.Context) error

func SetupOTel(serviceName, version string) (Shutdown, error)
```

Initialises the OpenTelemetry SDK:

- Trace exporter: OTLP/HTTP to `OTEL_EXPORTER_OTLP_ENDPOINT` (no-op if unset).
- Metric exporter: OTLP/HTTP to `OTEL_EXPORTER_OTLP_ENDPOINT` (no Prometheus endpoint).
- Sets global `TracerProvider` and `MeterProvider`.

Call `defer shutdown(ctx)` in `main` to flush pending telemetry before exit.

______________________________________________________________________

## auth

```go
import "github.com/paper-board/sdk/auth"
```

### AuthCtx

```go
type AuthCtx struct {
    UserID string
    OrgID  string
    Roles  []Role
    Env    Env
    Mode   AuthMode
}
```

Populated by the middleware and stored in `context.Context`.

### AuthMode / Role / Env

```go
type AuthMode int
const (
    AuthModeUnknown AuthMode = iota
    AuthModeJWT
    AuthModeAPIKey
)

type Role int
const (
    RoleUnknown Role = iota
    RoleOwner
    RoleAdmin
    RoleMember
)

type Env string
const (
    EnvLive Env = "live"
    EnvTest Env = "test"
)
```

### WithContext / FromContext

```go
func WithContext(ctx context.Context, a AuthCtx) context.Context
func FromContext(ctx context.Context) (AuthCtx, bool)
```

Store/retrieve `AuthCtx` from a `context.Context`.

### Config / Options

```go
type Config struct { /* unexported fields */ }
type Option func(*Config)

func WithClient(c identityv1.AuthServiceClient) Option
func WithKeystore(s *Keystore) Option
func WithKeystoreTTL(d time.Duration) Option

func New(opts ...Option) *Config
```

Build an auth middleware config. Must provide either `WithClient` (SDK manages
the Keystore) or `WithKeystore` (caller manages caching).

### Middleware

```go
func (cfg *Config) Require(modes ...AuthMode) func(http.Handler) http.Handler
func (cfg *Config) RequireRole(roles ...Role) func(http.Handler) http.Handler
```

`Require` verifies the token (JWT or API key) and stores `AuthCtx` in the
request context. Pass one or more `AuthMode` values to restrict which credential
types are accepted; pass none to accept both.

`RequireRole` chains after `Require` and returns 403 if the resolved `AuthCtx`
does not carry at least one of the specified roles.

### Keystore

```go
type Keystore struct { /* unexported fields */ }

func NewKeystore(client identityv1.AuthServiceClient) *Keystore
func NewKeystoreWithTTL(client identityv1.AuthServiceClient, ttl time.Duration) *Keystore

func (k *Keystore) Get(ctx context.Context, kid string) ([]byte, string, error)
func (k *Keystore) Refresh(ctx context.Context, kid string) ([]byte, string, error)
```

In-memory cache of RSA public keys fetched from identity gRPC.
Default TTL is 5 minutes. Use `NewKeystoreWithTTL` to override.

### auth/mock

```go
import "github.com/paper-board/sdk/auth/mock"
```

```go
type AuthClient struct {
    VerifyAPIKeyFn func(ctx context.Context, in *identityv1.VerifyAPIKeyRequest, opts ...grpc.CallOption) (*identityv1.VerifyAPIKeyResponse, error)
    VerifyJWTFn    func(ctx context.Context, in *identityv1.VerifyJWTRequest, opts ...grpc.CallOption) (*identityv1.VerifyJWTResponse, error)
    IssueJWTFn     func(ctx context.Context, in *identityv1.IssueJWTRequest, opts ...grpc.CallOption) (*identityv1.IssueJWTResponse, error)
    GetPublicKeyFn func(ctx context.Context, in *identityv1.GetPublicKeyRequest, opts ...grpc.CallOption) (*identityv1.GetPublicKeyResponse, error)
}
```

Implements `identityv1.AuthServiceClient`. Set the `*Fn` fields in tests.

______________________________________________________________________

## errors

```go
import "github.com/paper-board/sdk/errors"
```

### Sentinels

```go
var (
    ErrNotFound         error // 404 / NOT_FOUND
    ErrConflict         error // 409 / ALREADY_EXISTS
    ErrUnauthorized     error // 401 / UNAUTHENTICATED
    ErrPermissionDenied error // 403 / PERMISSION_DENIED
    ErrInvalidInput     error // 422 / INVALID_ARGUMENT
    ErrRateLimited      error // 429 / RESOURCE_EXHAUSTED
    ErrInternal         error // 500 / INTERNAL
    ErrUnavailable      error // 503 / UNAVAILABLE
)
```

Wrap one of these in every service error so callers can route on type:

```go
return fmt.Errorf("agent not found: %w", pberrs.ErrNotFound)
```

### Wrap

```go
func Wrap(sentinel error, msg string, kvs ...any) error
```

Wraps `sentinel` with a formatted message and optional key-value context.
Result satisfies `errors.Is(err, sentinel)`.

### Status mapping

```go
func ToGRPCStatus(err error) error    // wraps err in a gRPC Status with correct code
func FromGRPCStatus(err error) error  // converts a gRPC Status back to a sentinel error

func ToHTTPStatus(err error) int      // returns the HTTP status code for err
func Kind(err error) string           // returns the sentinel name, e.g. "not_found"
func Message(err error) string        // returns the human-readable message
```

### Re-exports

```go
func Is(err, target error) bool
func As(err error, target any) bool
```

Re-exported from stdlib so callers need only one import.

______________________________________________________________________

## config

```go
import "github.com/paper-board/sdk/config"
```

### MustBind

```go
func MustBind[T any]() *T
```

Populates a fresh `T` from environment variables using struct tags:

| Tag        | Purpose                               |
| ---------- | ------------------------------------- |
| `env`      | Environment variable name             |
| `default`  | Value used when env is unset or empty |
| `validate` | validator/v10 expression              |

Supported field types: `string`, `bool`, `int*`, `uint*`, `float*`,
`time.Duration`, `[]string` (comma-separated).

On any binding or validation error `MustBind` logs to stderr and calls
`os.Exit(1)` — it is designed for use in `main()` only.

```go
type Config struct {
    DatabaseURL string        `env:"DATABASE_URL"  validate:"required,url"`
    Port        int           `env:"SERVER_PORT"   default:"8080"          validate:"gte=1,lte=65535"`
    LogLevel    string        `env:"LOG_LEVEL"     default:"info"           validate:"oneof=debug info warn error"`
    Timeout     time.Duration `env:"TIMEOUT"       default:"30s"`
}

cfg := config.MustBind[Config]()
```

______________________________________________________________________

## healthcheck

```go
import "github.com/paper-board/sdk/healthcheck"
```

### Checker / Result

```go
type Checker interface {
    Name() string
    Check(ctx context.Context) error
}

type Result struct {
    Name    string `json:"name"`
    Healthy bool   `json:"healthy"`
    Error   string `json:"error,omitempty"`
}
```

### Registry

```go
func New(checks ...Checker) *Registry
func (r *Registry) Checks() []Checker
func (r *Registry) Handler() http.HandlerFunc
```

`Handler` runs all checks in parallel with a 500ms per-check timeout.
Returns `200 {"status":"ready",...}` when all pass;
`503 {"status":"not_ready",...}` when any fail.

### Built-in checkers

```go
func DB(pool *pgxpool.Pool) Checker   // SELECT 1 ping
func GRPC(name string, conn *grpc.ClientConn) Checker  // connectivity state probe
```

### IsProbePath

```go
func IsProbePath(r *http.Request) bool
```

Returns `true` for `/livez`, `/readyz`, `/healthz`. Use in middleware to skip
logging and OTLP instrumentation for probe traffic.

______________________________________________________________________

## idempotency

```go
import "github.com/paper-board/sdk/idempotency"
```

### Require

```go
func Require(store Store, opts ...Option) func(http.Handler) http.Handler
```

HTTP middleware. Intercepts `POST`, `PUT`, `PATCH`, `DELETE` requests carrying
an `Idempotency-Key` header. On a matching key + matching body, replays the
stored response byte-for-byte and adds `Idempotent-Replay: true`. On key match

- body mismatch, returns 422.

Keys are scoped per org via `org_id` extracted from the request context
(set upstream by auth middleware). TTL is 24 hours.

### WithExclude

```go
func WithExclude(routes ...string) Option
```

Bypass idempotency for specific routes, e.g. `"POST /v1/auth/login"`.

### Store / Record

```go
type Record struct {
    OrgID           uuid.UUID
    Key             string
    RequestHash     string
    Status          int
    ResponseBody    []byte
    ResponseHeaders map[string][]string
    CreatedAt       time.Time
}

type Store interface {
    Get(ctx context.Context, orgID uuid.UUID, key string) (*Record, error)
    Put(ctx context.Context, rec *Record) error
}

var ErrNotFound error // returned by Store.Get when no record exists
```

### MemoryStore

```go
func NewMemoryStore() *MemoryStore
```

In-process store for unit tests. Not safe for multi-replica use.

### Context helpers

```go
func WithOrgID(ctx context.Context, orgID uuid.UUID) context.Context
func FromContext(ctx context.Context) (uuid.UUID, bool)
```

______________________________________________________________________

## httpmw

```go
import "github.com/paper-board/sdk/httpmw"
```

### Middleware functions

```go
func RequestID(next http.Handler) http.Handler       // injects X-Request-Id; reads if present
func Logger(next http.Handler) http.Handler          // structured access log via log.Pkg
func Recover(svc string) func(http.Handler) http.Handler  // converts panics to 500
func BodyLimit(n int64) func(http.Handler) http.Handler   // 413 on oversized body
func OtelHTTP(serviceName string) func(http.Handler) http.Handler  // OTLP span per request
func TrustHeaders(next http.Handler) http.Handler    // propagates X-User-Id, X-Org-Id, X-Roles from gateway
func AuthStub(orgID uuid.UUID) func(http.Handler) http.Handler     // dev/test only: injects a static AuthCtx
```

`HeaderRequestID = "X-Request-Id"` — constant for the request-id header name.

### Error response helpers

```go
func HandleErr(w http.ResponseWriter, r *http.Request, svc string, err error) // maps err to HTTP status + writes JSON
func WriteError(w http.ResponseWriter, status int, code, message string)       // writes a raw ErrorEnvelope

type ErrorEnvelope struct {
    Error ErrorBody `json:"error"`
}
type ErrorBody struct {
    Code    string            `json:"code"`
    Message string            `json:"message"`
    Details []ValidationDetail `json:"details,omitempty"`
}
type ValidationDetail struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}
```

`HandleErr` calls `errors.ToHTTPStatus` to resolve the status code, then writes
an `ErrorEnvelope` JSON body. On validation errors it unwraps
`validator.ValidationErrors` into `Details`.

______________________________________________________________________

## httpclient

```go
import "github.com/paper-board/sdk/httpclient"
```

### Default

```go
func Default(timeout time.Duration) *http.Client
```

Returns an `*http.Client` with OTLP transport instrumentation and the specified
timeout. Use this for all outbound HTTP calls from service code.

______________________________________________________________________

## store

```go
import "github.com/paper-board/sdk/store"
```

### WithTx

```go
func WithTx[T any](ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) (T, error)) (T, error)
```

Runs `fn` inside a pgx transaction. Commits on success, rolls back on error.
Wraps rollback errors so the caller sees the original error.

______________________________________________________________________

## clock

```go
import "github.com/paper-board/sdk/clock"
```

### Clock interface

```go
type Clock interface {
    Now() time.Time
}
```

### Implementations

```go
type Real struct{}
func (Real) Now() time.Time  // returns time.Now().UTC()

type Fake struct { /* unexported */ }
func NewFake(t time.Time) *Fake
func (f *Fake) Now() time.Time
func (f *Fake) Set(t time.Time)
func (f *Fake) Advance(d time.Duration)
```

Inject `clock.Clock` into structs that need the current time; use `clock.Real{}`
in production and `clock.NewFake(t)` in tests.

______________________________________________________________________

## testfixture

```go
import "github.com/paper-board/sdk/testfixture"
```

Test-only helpers backed by testcontainers-go. Not imported in production code.

### PostgresContainer

```go
func PostgresContainer(t *testing.T, schema string) *pgxpool.Pool
func ConnectionString() string
```

Starts (or reuses) a shared testcontainers Postgres instance for the test
binary. Creates the requested schema and returns a pool scoped to it.
The container lifecycle is managed by the test binary; `t.Cleanup` handles pool
close.

### LoadSchema

```go
func LoadSchema(t *testing.T, pool *pgxpool.Pool, fsys fs.FS, root string) 
```

Applies all `*.up.sql` files from `fsys/root` in lexicographic order.
Used to bootstrap a schema in integration tests without running the migrator
binary.

### Truncate

```go
func Truncate(t *testing.T, pool *pgxpool.Pool, schemas ...string)
```

Truncates all user tables in the listed schemas (cascade). Call at the start of
each test function to achieve isolated state without recreating the schema.
