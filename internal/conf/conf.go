package conf

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
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
	// Disable authentication for any reasons it doesn't work with hostname like "*prod*"
	SecretKeyV1 string `envconfig:"SECRET_KEY"`
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

			switch value.Kind() { //nolint:exhaustive
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
