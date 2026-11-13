package config

import (
	"os"
	"strconv"
)

type Config struct {
	HTTPAddr string

	Env string // local|prod

	CookieSecure   bool
	CookieSameSite string // lax|strict|none

	RedirectAllowlist []string // ej: "/home", "/"

	RateLimitRPS int

	IdentityGRPCAddr       string
	AccountsGRPCAddr       string
	LedgerPaymentsGRPCAddr string

	// TLS
	GRPCUseTLS         bool
	GRPCCACertPath     string
	GRPCClientCertPath string
	GRPCClientKeyPath  string

	// --- Redis (opcional)
	RedisEnabled  bool
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	RedisTLS      bool

	TokenCachePrefix      string
	TokenCacheSkewSeconds int

	CSRFSecret string
}

func Load() Config {
	return Config{
		HTTPAddr: getenv("HTTP_ADDR", ":8080"),
		Env:      getenv("ENV", "local"),

		CookieSecure:   getenv("COOKIE_SECURE", "false") == "true",
		CookieSameSite: getenv("COOKIE_SAMESITE", "lax"),

		RedirectAllowlist: splitCSV(getenv("REDIRECT_ALLOWLIST", "/,/home")),

		RateLimitRPS: mustInt(getenv("RATE_LIMIT_RPS", "50")),

		IdentityGRPCAddr:       getenv("IDENTITY_GRPC_ADDR", "identity:9090"),
		AccountsGRPCAddr:       getenv("ACCOUNT_GRPC_ADDR", "accounts:9090"),
		LedgerPaymentsGRPCAddr: getenv("LEDGER_GRPC_ADDR", "ledger:9090"),

		GRPCUseTLS:         getenv("GRPC_USE_TLS", "false") == "true",
		GRPCCACertPath:     getenv("GRPC_CA_CERT", ""),
		GRPCClientCertPath: getenv("GRPC_CLIENT_CERT", ""),
		GRPCClientKeyPath:  getenv("GRPC_CLIENT_KEY", ""),

		RedisEnabled:  getenv("REDIS_ENABLED", "false") == "true",
		RedisAddr:     getenv("REDIS_ADDR", "redis:6379"),
		RedisPassword: getenv("REDIS_PASSWORD", ""),
		RedisDB:       mustInt(getenv("REDIS_DB", "0")),
		RedisTLS:      getenv("REDIS_TLS", "false") == "true",

		TokenCachePrefix:      getenv("TOKEN_CACHE_PREFIX", "bff:tok:"),
		TokenCacheSkewSeconds: mustInt(getenv("TOKEN_CACHE_SKEW_SECONDS", "30")),

		CSRFSecret: getenv("CSRF_SECRET", ""),
	}
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func mustInt(s string) int {
	n, _ := strconv.Atoi(s)
	if n <= 0 {
		n = 1
	}
	return n
}

func splitCSV(s string) []string {
	out := []string{}
	cur := ""
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if cur != "" {
				out = append(out, cur)
			}
			cur = ""
			continue
		}
		cur += string(s[i])
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
