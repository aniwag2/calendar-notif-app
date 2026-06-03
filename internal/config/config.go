package config

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration, populated from environment variables.
type Config struct {
	Addr          string // listen address, e.g. ":3002"
	BaseURL       string // public base URL, e.g. https://theralert.aniwaghray.com
	LogoutURL     string // where the logout button redirects
	DatabaseURL   string // postgres connection string
	SessionSecret []byte // key for signing session cookies
	SecureCookies bool   // set Secure flag on cookies (true behind HTTPS)

	// SMTP
	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	MailFrom string

	// Registration IP allowlist. Empty means allow all.
	AllowedCIDRs []*net.IPNet

	// Location is the display timezone for the clock, calendar, and emails.
	Location *time.Location
}

// Load reads configuration from the environment (and a .env file if present).
func Load() (*Config, error) {
	_ = godotenv.Load() // ignore error: .env is optional

	c := &Config{
		Addr:          getenv("ADDR", ":3002"),
		BaseURL:       getenv("BASE_URL", "http://localhost:3002"),
		LogoutURL:     getenv("LOGOUT_URL", "https://theralert.aniwaghray.com"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		SecureCookies: getenv("SECURE_COOKIES", "false") == "true",
		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPPort:      getenv("SMTP_PORT", "587"),
		SMTPUser:      os.Getenv("SMTP_USER"),
		SMTPPass:      os.Getenv("SMTP_PASS"),
		MailFrom:      getenv("MAIL_FROM", os.Getenv("SMTP_USER")),
	}

	// Timezone: explicit TZ env wins; otherwise use the server's local zone.
	if tz := strings.TrimSpace(os.Getenv("TZ")); tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return nil, fmt.Errorf("invalid TZ %q: %w", tz, err)
		}
		c.Location = loc
	} else {
		c.Location = time.Local
	}

	secret := os.Getenv("SESSION_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}
	c.SessionSecret = []byte(secret)

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if raw := strings.TrimSpace(os.Getenv("ALLOWED_CIDRS")); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			// Allow bare IPs as well as CIDR notation.
			if !strings.Contains(part, "/") {
				if strings.Contains(part, ":") {
					part += "/128"
				} else {
					part += "/32"
				}
			}
			_, ipnet, err := net.ParseCIDR(part)
			if err != nil {
				return nil, fmt.Errorf("invalid ALLOWED_CIDRS entry %q: %w", part, err)
			}
			c.AllowedCIDRs = append(c.AllowedCIDRs, ipnet)
		}
	}

	return c, nil
}

// RegistrationAllowed reports whether the given remote IP may register.
// When no allowlist is configured, all IPs are allowed.
func (c *Config) RegistrationAllowed(ip net.IP) bool {
	if len(c.AllowedCIDRs) == 0 {
		return true
	}
	for _, n := range c.AllowedCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
