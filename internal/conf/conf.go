package conf

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

const osPref = "SD"
const DevelopMode = "devel"

const (
	DefaultWebURL          = "http://localhost:9000"
	DefaultHostname        = "localhost"
	DefaultPort            = "8000"
	DefaultOpenAPISpecPath = "openapi.yaml"

	// MinSecretKeyLength is the minimum required length for the HMAC secret key.
	// HMAC-SHA256 requires at least 32 bytes for cryptographic strength.
	MinSecretKeyLength = 32
)

// Notification delivery defaults, applied when notifications are enabled.
const (
	DefaultSMTPTimeout     = "30s"
	DefaultLeaseTimeout    = "60s"
	DefaultMaxAttempts     = "5"
	DefaultBackoffInterval = "5m"
)

type Config struct {
	// DB connection uri
	// format is `postgresql://user:pass@host:port/db_name`
	DB string `envconfig:"DB"`
	// Cache connection uri
	// It can be redis format or internal
	Cache string `envconfig:"CACHE"`
	// Keycloak settings
	Keycloak *Keycloak `envconfig:"KEYCLOAK"`
	// Log level for verbosity
	LogLevel string `envconfig:"LOG_LEVEL"`
	// App port
	Port string `envconfig:"PORT"`
	// Hostname for the app, used to generate a callback URL for keycloak
	// Example: https://api.example.com
	Hostname string `envconfig:"HOSTNAME"`
	// Web URL for the app
	// Example: https://web.example.com
	WebURL string `envconfig:"WEB_URL"`
	// Secret key for local HMAC authentication (dev, tests, service-to-service)
	SecretKeyV1 string `envconfig:"SECRET_KEY"`
	// OpenAPISpecPath is the filesystem path to the OpenAPI spec served at
	// /openapi.json. Defaults to "openapi.yaml" (resolved relative to the
	// process working directory, matching the container's WORKDIR layout).
	// Override via SD_OPENAPI_SPEC_PATH for tests or non-standard deployments.
	OpenAPISpecPath string `envconfig:"OPENAPI_SPEC_PATH"`
	// RBAC configuration
	RBAC RBACConfig `envconfig:"RBAC"`
	// SMTP transport settings for outgoing mail
	SMTP SMTPConfig `envconfig:"SMTP"`
	// Notifications feature settings
	Notifications NotificationsConfig `envconfig:"NOTIFICATIONS"`
}

// SMTPConfig holds the direct OTC SMTP transport settings.
//
// No envconfig tags: a tag like "USER" makes envconfig fall back to the shell's $USER
// when SD_SMTP_USER is unset. Field names yield the same keys without that fallback.
type SMTPConfig struct {
	Host     string
	Port     string
	From     string
	User     string
	Password string
	TLS      bool
	// Timeout is a Go duration string (e.g. "30s") for the SMTP connect/send.
	Timeout string
}

// NotificationsConfig holds the maintenance email notification settings.
type NotificationsConfig struct {
	// Enabled is the master on/off switch. Untagged for the same reason as SMTPConfig.
	Enabled bool
	// LeaseTimeout is a Go duration string; must exceed the SMTP timeout.
	LeaseTimeout string `envconfig:"LEASE_TIMEOUT"`
	// MaxAttempts is the finite retry limit before a row is marked failed.
	MaxAttempts string `envconfig:"MAX_ATTEMPTS"`
	// BackoffInterval is the base delay (Go duration string) for retry backoff.
	BackoffInterval string `envconfig:"BACKOFF_INTERVAL"`
	// SmodEmail is the fixed SMOD team review recipient.
	SmodEmail string `envconfig:"SMOD_EMAIL"`
	// EmailsOperators is the review recipient list for the Operator role.
	EmailsOperators string `envconfig:"EMAILS_OPERATORS"`
	// EmailsAdmins is the review recipient list for the Admin role.
	EmailsAdmins string `envconfig:"EMAILS_ADMINS"`
}

type RBACConfig struct {
	// Creators group name
	Creators string `envconfig:"GROUPS_CREATORS"`
	// Operators group name
	Operators string `envconfig:"GROUPS_OPERATORS"`
	// Admins group name (mandatory)
	Admins string `envconfig:"GROUPS_ADMINS"`
}

type Keycloak struct {
	URL          string `envconfig:"URL"`
	Realm        string `envconfig:"REALM"`
	ClientID     string `envconfig:"CLIENT_ID"`
	ClientSecret string `envconfig:"CLIENT_SECRET"`
}

func (c *Config) Validate() error {
	p, err := strconv.Atoi(c.Port)
	if err != nil {
		return fmt.Errorf("wrong SD_PORT format, should be a number in range 1025:50000")
	}
	if p < 1024 || p > 50000 {
		return fmt.Errorf("wrong port for http server")
	}

	if provErr := c.validateProviders(); provErr != nil {
		return provErr
	}

	if rbacErr := c.RBAC.Validate(); rbacErr != nil {
		return rbacErr
	}

	if notifErr := c.validateNotifications(); notifErr != nil {
		return notifErr
	}

	return nil
}

// validateNotifications enforces SMTP and review-audience requirements when
// notifications are enabled. When disabled, the feature stays inert and no
// notification settings are required.
func (c *Config) validateNotifications() error {
	if !c.Notifications.Enabled {
		return nil
	}

	if c.SMTP.Host == "" || c.SMTP.Port == "" || c.SMTP.From == "" {
		return fmt.Errorf("notifications enabled: SD_SMTP_HOST, SD_SMTP_PORT and SD_SMTP_FROM are required")
	}

	if c.Notifications.SmodEmail == "" &&
		c.Notifications.EmailsOperators == "" &&
		c.Notifications.EmailsAdmins == "" {
		return fmt.Errorf("notifications enabled: at least one review address must be set " +
			"(SD_NOTIFICATIONS_SMOD_EMAIL, SD_NOTIFICATIONS_EMAILS_OPERATORS or SD_NOTIFICATIONS_EMAILS_ADMINS)")
	}

	smtpTimeout, err := time.ParseDuration(c.SMTP.Timeout)
	if err != nil {
		return fmt.Errorf("invalid SD_SMTP_TIMEOUT: %w", err)
	}

	leaseTimeout, err := time.ParseDuration(c.Notifications.LeaseTimeout)
	if err != nil {
		return fmt.Errorf("invalid SD_NOTIFICATIONS_LEASE_TIMEOUT: %w", err)
	}

	if leaseTimeout <= smtpTimeout {
		return fmt.Errorf("SD_NOTIFICATIONS_LEASE_TIMEOUT (%s) must be greater than SD_SMTP_TIMEOUT (%s)",
			leaseTimeout, smtpTimeout)
	}

	attempts, err := strconv.Atoi(c.Notifications.MaxAttempts)
	if err != nil || attempts < 1 {
		return fmt.Errorf("SD_NOTIFICATIONS_MAX_ATTEMPTS must be a positive integer")
	}

	if _, parseErr := time.ParseDuration(c.Notifications.BackoffInterval); parseErr != nil {
		return fmt.Errorf("invalid SD_NOTIFICATIONS_BACKOFF_INTERVAL: %w", parseErr)
	}

	return nil
}

// validateProviders ensures at least one authentication provider is configured.
func (c *Config) validateProviders() error {
	hasKeycloak := c.Keycloak != nil && c.Keycloak.URL != "" && c.Keycloak.Realm != "" &&
		c.Keycloak.ClientID != "" && c.Keycloak.ClientSecret != ""
	hasLocal := c.SecretKeyV1 != ""

	if !hasKeycloak && !hasLocal {
		return fmt.Errorf("at least one authentication provider must be configured: " +
			"set SD_KEYCLOAK_* for Keycloak or SD_SECRET_KEY for local HMAC")
	}

	if hasLocal && len(c.SecretKeyV1) < MinSecretKeyLength {
		return fmt.Errorf("SD_SECRET_KEY must be at least %d characters for HMAC-SHA256 security", MinSecretKeyLength)
	}

	return nil
}

func (r *RBACConfig) Validate() error {
	if r.Admins == "" {
		return fmt.Errorf("SD_RBAC_GROUPS_ADMINS is required")
	}
	return nil
}

func (c *Config) FillDefaults() {
	if c.LogLevel == "" {
		c.LogLevel = DevelopMode
	}

	if c.Port == "" {
		c.Port = DefaultPort
	}

	if c.Hostname == "" {
		c.Hostname = DefaultHostname
	}

	if c.WebURL == "" {
		c.WebURL = DefaultWebURL
	}

	if c.OpenAPISpecPath == "" {
		c.OpenAPISpecPath = DefaultOpenAPISpecPath
	}

	if c.Notifications.Enabled {
		if c.SMTP.Timeout == "" {
			c.SMTP.Timeout = DefaultSMTPTimeout
		}
		if c.Notifications.LeaseTimeout == "" {
			c.Notifications.LeaseTimeout = DefaultLeaseTimeout
		}
		if c.Notifications.MaxAttempts == "" {
			c.Notifications.MaxAttempts = DefaultMaxAttempts
		}
		if c.Notifications.BackoffInterval == "" {
			c.Notifications.BackoffInterval = DefaultBackoffInterval
		}
	}
}

// LoadConf loads configuration from .env file and environment.
// Env variables are preferred.
func LoadConf() (*Config, error) {
	var envMap map[string]string
	envMap, _ = godotenv.Read()

	var c Config
	err := envconfig.Process(osPref, &c)
	if err != nil {
		return nil, err
	}

	if err = mergeConfigs(envMap, &c, osPref); err != nil {
		return nil, err
	}

	c.FillDefaults()

	if err = c.Validate(); err != nil {
		return nil, err
	}

	return &c, nil
}

var ErrInvalidDataMerge = errors.New("could not merge config, the obj must be a point to a struct")

const envConfigTag = "envconfig"

// envKeyPart mirrors envconfig's key derivation: the tag when present, the field
// name otherwise. Untagged fields are how we avoid envconfig's bare-name fallback.
func envKeyPart(field reflect.StructField) string {
	if tag := field.Tag.Get(envConfigTag); tag != "" {
		return tag
	}

	return field.Name
}

// mergeConfigs allow to merge config params from env variables and .env file.
// It checks the Config struct and if the value is missing, it set up the value from .env file.
func mergeConfigs(env map[string]string, obj any, prefix string) error { //nolint:gocognit
	if env == nil {
		return nil
	}

	v := reflect.ValueOf(obj)

	if v.Kind() != reflect.Ptr {
		return ErrInvalidDataMerge
	}

	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return ErrInvalidDataMerge
	}

	t := v.Type()

	// Iterate through the fields
	for i := 0; i < v.NumField(); i++ { //nolint:intrange
		field := t.Field(i)
		value := v.Field(i)

		// Handle pointer to struct (e.g., *Keycloak)
		if value.Kind() == reflect.Ptr && value.Elem().Kind() == reflect.Struct {
			confPrefix := fmt.Sprintf("%s_%s", prefix, envKeyPart(field))
			err := mergeConfigs(env, value.Interface(), confPrefix)
			if err != nil {
				return err
			}

			continue
		}

		// Handle embedded struct (e.g., RBACConfig)
		// For struct values (not pointers), we need to pass a pointer
		if value.Kind() == reflect.Struct {
			confPrefix := fmt.Sprintf("%s_%s", prefix, envKeyPart(field))
			err := mergeConfigs(env, value.Addr().Interface(), confPrefix)
			if err != nil {
				return err
			}

			continue
		}

		if value.IsZero() && value.IsValid() && value.CanSet() {
			mapKey := strings.ToUpper(fmt.Sprintf("%s_%s", prefix, envKeyPart(field)))

			switch value.Kind() {
			case reflect.String:
				value.SetString(env[mapKey])
			case reflect.Bool:
				if env[mapKey] == "true" {
					value.SetBool(true)
				}
			default:
				return fmt.Errorf("unsupported type for config field %s", field.Name)
			}
		}
	}

	return nil
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	return "<hidden>"
}

func sanitizeDBString(dbURL string) string {
	if dbURL == "" {
		return ""
	}
	u, err := url.Parse(dbURL)
	if err != nil {
		return dbURL
	}
	return fmt.Sprintf("%s://%s%s",
		u.Scheme,
		u.Host,
		u.Path,
	)
}

func (c *Config) Log(logger *zap.Logger) {
	logger.Info("Application starting with the following configuration:")

	logger.Info("Endpoint configuration",
		zap.String("hostname", c.Hostname),
		zap.String("port", c.Port),
		zap.String("web_url", c.WebURL),
	)

	logger.Info("Authentication configuration",
		zap.Bool("keycloak_configured", c.Keycloak != nil && c.Keycloak.URL != ""),
		zap.Bool("local_hmac_configured", c.SecretKeyV1 != ""),
		zap.String("creators_group", c.RBAC.Creators),
		zap.String("operators_group", c.RBAC.Operators),
		zap.String("admins_group", c.RBAC.Admins),
		zap.String("secret_key_v1", maskSecret(c.SecretKeyV1)),
	)

	logger.Info("Storage and logging configuration",
		zap.String("db", sanitizeDBString(c.DB)),
		// zap.String("cache", c.Cache),
		zap.String("log_level", c.LogLevel),
		zap.String("openapi_spec_path", c.OpenAPISpecPath),
	)

	if c.Keycloak != nil {
		logger.Info("Keycloak configuration",
			zap.String("url", c.Keycloak.URL),
			zap.String("realm", c.Keycloak.Realm),
			zap.String("client_id", c.Keycloak.ClientID),
			zap.String("client_secret", maskSecret(c.Keycloak.ClientSecret)),
		)
	}

	logger.Info("Notifications configuration",
		zap.Bool("enabled", c.Notifications.Enabled),
		zap.String("smtp_host", c.SMTP.Host),
		zap.String("smtp_port", c.SMTP.Port),
		zap.String("smtp_from", c.SMTP.From),
		zap.String("smtp_user", c.SMTP.User),
		zap.String("smtp_password", maskSecret(c.SMTP.Password)),
		zap.Bool("smtp_tls", c.SMTP.TLS),
	)
}
