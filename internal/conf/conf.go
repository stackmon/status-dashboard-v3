package conf

import (
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

const osPref = "SD"
const DevelopMode = "devel"

const (
	DefaultWebURL   = "http://localhost:9000"
	DefaultHostname = "localhost"
	DefaultPort     = "8000"
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
	// Disable authentication for any reasons it doesn't work with hostname like "*prod*"
	AuthenticationDisabled bool `envconfig:"AUTHENTICATION_DISABLED"`
	// Secret key for V1 authentication (deprecated)
	SecretKeyV1 string `envconfig:"SECRET_KEY"`
	// Auth group name that users must belong to for authorization (optional)
	AuthGroup string `envconfig:"AUTH_GROUP"`
	// RBAC: Creators group name
	CreatorsGroup string `envconfig:"GROUP_CREATORS"`
	// RBAC: Operators group name
	OperatorsGroup string `envconfig:"GROUP_OPERATORS"`
	// RBAC: Admins group name
	AdminsGroup string `envconfig:"GROUP_ADMINS"`
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
		return fmt.Errorf("wront SD_PORT format, should be a number in range 1025:50000")
	}
	if p < 1024 || p > 50000 {
		return fmt.Errorf("wrong port for http server")
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

		if value.Kind() == reflect.Ptr && value.Elem().Kind() == reflect.Struct {
			envValueTag := field.Tag.Get(envConfigTag)
			confPrefix := fmt.Sprintf("%s_%s", prefix, envValueTag)
			err := mergeConfigs(env, value.Interface(), confPrefix)
			if err != nil {
				return err
			}

			continue
		}

		if value.IsZero() && value.IsValid() && value.CanSet() {
			envValueTag := field.Tag.Get(envConfigTag)
			mapKey := strings.ToUpper(fmt.Sprintf("%s_%s", prefix, envValueTag))

			//nolint:exhaustive
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
		zap.Bool("authentication_disabled", c.AuthenticationDisabled),
		zap.String("auth_group", c.AuthGroup),
		zap.String("creators_group", c.CreatorsGroup),
		zap.String("operators_group", c.OperatorsGroup),
		zap.String("admins_group", c.AdminsGroup),
		zap.String("secret_key_v1", maskSecret(c.SecretKeyV1)),
	)

	logger.Info("Storage and logging configuration",
		zap.String("db", sanitizeDBString(c.DB)),
		// zap.String("cache", c.Cache),
		zap.String("log_level", c.LogLevel),
	)

	if c.Keycloak != nil {
		logger.Info("Keycloak configuration",
			zap.String("url", c.Keycloak.URL),
			zap.String("realm", c.Keycloak.Realm),
			zap.String("client_id", c.Keycloak.ClientID),
			zap.String("client_secret", maskSecret(c.Keycloak.ClientSecret)),
		)
	}
}
