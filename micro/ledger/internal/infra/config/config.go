// micro\ledger\internal\infra\config\config.go
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv      string
	GrpcPort    string
	ServiceName string

	// AWS
	AWSRegion string

	// JWT (Cognito)
	JWTIssuer   string
	JWTAudience string
	JWTJWKSURL  string

	// AVP
	AVPPolicyStoreID string

	// Audit EventBridge
	AuditEventBusName string
	AuditSource       string
	AuditDetailType   string

	// Hash salts (no PII cruda)
	HashSaltIP string
	HashSaltUA string
	// DB
	DBReadDSN  string
	DBWriteDSN string

	// Accounts (REST internal)
	AccountsBaseURL     string
	AccountsInternalTok string

	// EventBridge
	DomainEventBusName string

	// Outbox worker
	OutboxBatchSize    int
	OutboxPollInterval time.Duration

	// Security (opcional)
	InternalTokenHeader string

	// Accounts (gRPC internal)
	AccountsGrpcTarget        string
	AccountsGrpcInsecure      bool
	AccountsGrpcTimeout       time.Duration
	AccountsGrpcTLSServerName string
	AccountsGrpcTLSCaFile     string

	LedgerSystemCreditClientIDs []string
	LedgerSystemCreditScope     []string
}

func Load() Config {
	return Config{
		AppEnv:      getEnv("APP_ENV", "dev"),
		GrpcPort:    getEnv("GRPC_PORT", "8082"),
		ServiceName: getEnv("SERVICE_NAME", "ledger-payments"),

		AWSRegion: getEnv("AWS_REGION", "us-east-1"),

		JWTIssuer:   getEnv("JWT_ISSUER", ""),
		JWTAudience: getEnv("JWT_AUDIENCE", ""),
		JWTJWKSURL:  getEnv("JWT_JWKS_URL", ""),

		AVPPolicyStoreID: getEnv("AVP_POLICY_STORE_ID", ""),

		AuditEventBusName: getEnv("AUDIT_EVENTBUS_NAME", "default"),
		AuditSource:       getEnv("AUDIT_SOURCE", "banking.ledger"),
		AuditDetailType:   getEnv("AUDIT_DETAIL_TYPE", "AuditEvent"),

		HashSaltIP: getEnv("HASH_SALT_IP", "ip_salt"),
		HashSaltUA: getEnv("HASH_SALT_UA", "ua_salt"),
		DBReadDSN:  getEnv("DB_READ_DSN", getEnv("DB_DSN", "")),
		DBWriteDSN: getEnv("DB_WRITE_DSN", getEnv("DB_DSN", "")),

		AccountsBaseURL:     strings.TrimRight(getEnv("ACCOUNTS_BASE_URL", "http://accounts:8080"), "/"),
		AccountsInternalTok: getEnv("ACCOUNTS_INTERNAL_TOKEN", ""),

		DomainEventBusName: getEnv("DOMAIN_EVENTBUS_NAME", "default"),

		OutboxBatchSize:    getEnvInt("OUTBOX_BATCH_SIZE", 50),
		OutboxPollInterval: getEnvDuration("OUTBOX_POLL_INTERVAL", 200*time.Millisecond),

		InternalTokenHeader: getEnv("INTERNAL_TOKEN_HEADER", "X-Internal-Token"),

		// Accounts (gRPC internal)
		AccountsGrpcTarget:          getEnv("ACCOUNTS_GRPC_TARGET", "account:9092"),
		AccountsGrpcInsecure:        getEnvBool("ACCOUNTS_GRPC_INSECURE", true),
		AccountsGrpcTimeout:         getEnvDuration("ACCOUNTS_GRPC_TIMEOUT", 30*time.Second),
		AccountsGrpcTLSServerName:   getEnv("ACCOUNTS_GRPC_TLS_SERVER_NAME", "accounts"),
		AccountsGrpcTLSCaFile:       getEnv("ACCOUNTS_GRPC_TLS_CA_FILE", ""),
		LedgerSystemCreditClientIDs: splitCSV(getEnv("LEDGER_SYSTEM_CREDIT_CLIENT_IDS", "")),
		LedgerSystemCreditScope:     splitCSV(getEnv("LEDGER_SYSTEM_CREDIT_SCOPE", "")),
	}
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getEnvInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func getEnvDuration(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnvBool(k string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(k)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}
