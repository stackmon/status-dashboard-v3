package conf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRBACConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    RBACConfig
		expectErr bool
		errSubstr string
	}{
		{
			name: "All groups configured",
			config: RBACConfig{
				Creators:  "sd_creators",
				Operators: "sd_operators",
				Admins:    "sd_admins",
			},
			expectErr: false,
		},
		{
			name: "Only Admins configured (minimum required)",
			config: RBACConfig{
				Admins: "sd_admins",
			},
			expectErr: false,
		},
		{
			name:      "Missing Admins fails validation",
			config:    RBACConfig{},
			expectErr: true,
			errSubstr: "SD_RBAC_GROUPS_ADMINS",
		},
		{
			name: "Missing Admins but other groups set fails",
			config: RBACConfig{
				Creators:  "sd_creators",
				Operators: "sd_operators",
			},
			expectErr: true,
			errSubstr: "SD_RBAC_GROUPS_ADMINS",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_PropagatesRBACError(t *testing.T) {
	cfg := &Config{
		Port:        "8000",
		SecretKeyV1: "test-secret-key-minimum-length!!", // 32 chars
		RBAC:        RBACConfig{},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SD_RBAC_GROUPS_ADMINS")
}

func TestConfig_Validate_RequiresProvider(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		expectErr bool
		errSubstr string
	}{
		{
			name: "No provider configured fails",
			cfg: Config{
				Port: "8000",
				RBAC: RBACConfig{Admins: "sd_admins"},
			},
			expectErr: true,
			errSubstr: "at least one authentication provider",
		},
		{
			name: "Local HMAC provider passes",
			cfg: Config{
				Port:        "8000",
				SecretKeyV1: "my-secret-key-that-is-32-chars!!", // 32 chars
				RBAC:        RBACConfig{Admins: "sd_admins"},
			},
			expectErr: false,
		},
		{
			name: "Keycloak provider passes",
			cfg: Config{
				Port: "8000",
				Keycloak: &Keycloak{
					URL:          "https://kc.example.com",
					Realm:        "myrealm",
					ClientID:     "client",
					ClientSecret: "secret",
				},
				RBAC: RBACConfig{Admins: "sd_admins"},
			},
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_MinSecretKeyLength(t *testing.T) {
	tests := []struct {
		name      string
		secret    string
		expectErr bool
		errSubstr string
	}{
		{
			name:      "Short secret fails",
			secret:    "too-short",
			expectErr: true,
			errSubstr: "at least 32 characters",
		},
		{
			name:      "31-char secret fails",
			secret:    "1234567890123456789012345678901", // 31 chars
			expectErr: true,
			errSubstr: "at least 32 characters",
		},
		{
			name:      "32-char secret passes",
			secret:    "12345678901234567890123456789012", // 32 chars
			expectErr: false,
		},
		{
			name:      "64-char secret passes",
			secret:    "1234567890123456789012345678901234567890123456789012345678901234", // 64 chars
			expectErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{
				Port:        "8000",
				SecretKeyV1: tc.secret,
				RBAC:        RBACConfig{Admins: "admins"},
			}
			err := cfg.Validate()
			if tc.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validate_PortValidation(t *testing.T) {
	base := Config{
		SecretKeyV1: "secret-key-that-is-32-chars-long", // 32 chars
		RBAC:        RBACConfig{Admins: "admins"},
	}

	tests := []struct {
		name      string
		port      string
		expectErr bool
		errSubstr string
	}{
		{name: "Valid port", port: "8000", expectErr: false},
		{name: "Non-numeric port", port: "abc", expectErr: true, errSubstr: "wrong SD_PORT format"},
		{name: "Port too low", port: "80", expectErr: true, errSubstr: "wrong port"},
		{name: "Port too high", port: "60000", expectErr: true, errSubstr: "wrong port"},
		{name: "Port at lower boundary", port: "1025", expectErr: false},
		{name: "Port at upper boundary", port: "50000", expectErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			cfg.Port = tc.port
			err := cfg.Validate()
			if tc.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFillDefaults(t *testing.T) {
	t.Run("fills all empty fields", func(t *testing.T) {
		c := &Config{}
		c.FillDefaults()

		assert.Equal(t, DevelopMode, c.LogLevel)
		assert.Equal(t, DefaultPort, c.Port)
		assert.Equal(t, DefaultHostname, c.Hostname)
		assert.Equal(t, DefaultWebURL, c.WebURL)
	})

	t.Run("preserves existing values", func(t *testing.T) {
		c := &Config{
			LogLevel: "info",
			Port:     "9090",
			Hostname: "api.example.com",
			WebURL:   "https://web.example.com",
		}
		c.FillDefaults()

		assert.Equal(t, "info", c.LogLevel)
		assert.Equal(t, "9090", c.Port)
		assert.Equal(t, "api.example.com", c.Hostname)
		assert.Equal(t, "https://web.example.com", c.WebURL)
	})
}

func TestMaskSecret(t *testing.T) {
	assert.Empty(t, maskSecret(""))
	assert.Equal(t, "<hidden>", maskSecret("my-secret"))
	assert.Equal(t, "<hidden>", maskSecret("x"))
}

func TestSanitizeDBString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "Empty string", input: "", expect: ""},
		{
			name:   "Full connection string strips credentials",
			input:  "postgresql://user:pass@localhost:5432/mydb",
			expect: "postgresql://localhost:5432/mydb",
		},
		{
			name:   "URL without credentials",
			input:  "postgresql://localhost:5432/mydb",
			expect: "postgresql://localhost:5432/mydb",
		},
		{
			name:   "Invalid URL returns raw string",
			input:  "://broken",
			expect: "://broken",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, sanitizeDBString(tc.input))
		})
	}
}

func TestMergeConfigs(t *testing.T) {
	t.Run("nil env map is no-op", func(t *testing.T) {
		c := &Config{}
		err := mergeConfigs(nil, c, "SD")
		require.NoError(t, err)
	})

	t.Run("non-pointer returns error", func(t *testing.T) {
		err := mergeConfigs(map[string]string{}, Config{}, "SD")
		require.ErrorIs(t, err, ErrInvalidDataMerge)
	})

	t.Run("pointer to non-struct returns error", func(t *testing.T) {
		s := "hello"
		err := mergeConfigs(map[string]string{}, &s, "SD")
		require.ErrorIs(t, err, ErrInvalidDataMerge)
	})

	t.Run("fills empty string fields from env map", func(t *testing.T) {
		c := &Config{Keycloak: &Keycloak{}}
		env := map[string]string{
			"SD_DB":        "postgresql://localhost/test",
			"SD_LOG_LEVEL": "info",
		}
		err := mergeConfigs(env, c, "SD")
		require.NoError(t, err)
		assert.Equal(t, "postgresql://localhost/test", c.DB)
		assert.Equal(t, "info", c.LogLevel)
	})

	t.Run("does not overwrite existing values", func(t *testing.T) {
		c := &Config{DB: "existing", Keycloak: &Keycloak{}}
		env := map[string]string{
			"SD_DB": "overwritten",
		}
		err := mergeConfigs(env, c, "SD")
		require.NoError(t, err)
		assert.Equal(t, "existing", c.DB)
	})

	t.Run("merges into embedded struct (RBACConfig)", func(t *testing.T) {
		c := &Config{Keycloak: &Keycloak{}}
		env := map[string]string{
			"SD_RBAC_GROUPS_ADMINS": "my-admins",
		}
		err := mergeConfigs(env, c, "SD")
		require.NoError(t, err)
		assert.Equal(t, "my-admins", c.RBAC.Admins)
	})

	t.Run("merges into pointer struct (Keycloak)", func(t *testing.T) {
		kc := &Keycloak{}
		c := &Config{Keycloak: kc}
		env := map[string]string{
			"SD_KEYCLOAK_URL":   "http://kc.local",
			"SD_KEYCLOAK_REALM": "test",
		}
		err := mergeConfigs(env, c, "SD")
		require.NoError(t, err)
		assert.Equal(t, "http://kc.local", c.Keycloak.URL)
		assert.Equal(t, "test", c.Keycloak.Realm)
	})

	t.Run("merges untagged SMTP fields by field name", func(t *testing.T) {
		c := &Config{Keycloak: &Keycloak{}}
		env := map[string]string{
			"SD_SMTP_HOST":    "smtp.local",
			"SD_SMTP_USER":    "mailer",
			"SD_SMTP_TIMEOUT": "15s",
		}
		err := mergeConfigs(env, c, "SD")
		require.NoError(t, err)
		assert.Equal(t, "smtp.local", c.SMTP.Host)
		assert.Equal(t, "mailer", c.SMTP.User)
		assert.Equal(t, "15s", c.SMTP.Timeout)
	})
}

// TestLoadConf_IgnoresBareEnvNames guards against envconfig's fallback to the bare tag
// name: a tag of "USER" would otherwise inherit the shell's $USER and enable SMTP AUTH
// against a server that offers none.
func TestLoadConf_IgnoresBareEnvNames(t *testing.T) {
	t.Setenv("USER", "shell-user")
	t.Setenv("PASSWORD", "shell-password")
	t.Setenv("SD_SECRET_KEY", "my-secret-key-that-is-32-chars!!")
	t.Setenv("SD_RBAC_GROUPS_ADMINS", "sd_admins")
	t.Setenv("SD_SMTP_HOST", "127.0.0.1")

	c, err := LoadConf()
	require.NoError(t, err)

	assert.Empty(t, c.SMTP.User)
	assert.Empty(t, c.SMTP.Password)
	assert.Equal(t, "127.0.0.1", c.SMTP.Host)
}

func baseNotifConfig() Config {
	return Config{
		Port:        "8000",
		SecretKeyV1: "my-secret-key-that-is-32-chars!!", // 32 chars
		RBAC:        RBACConfig{Admins: "sd_admins"},
	}
}

func TestValidateNotifications(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(c *Config)
		expectErr bool
		errSubstr string
	}{
		{
			name:      "disabled skips all notification checks",
			mutate:    func(c *Config) { c.Notifications.Enabled = false },
			expectErr: false,
		},
		{
			name: "enabled with full valid config passes",
			mutate: func(c *Config) {
				c.Notifications = NotificationsConfig{
					Enabled:         true,
					LeaseTimeout:    "60s",
					MaxAttempts:     "5",
					BackoffInterval: "5m",
					SmodEmail:       "support@com.com",
				}
				c.SMTP = SMTPConfig{Host: "smtp.otc", Port: "587", From: "sd@com.com", Timeout: "30s"}
			},
			expectErr: false,
		},
		{
			name: "enabled without SMTP host fails",
			mutate: func(c *Config) {
				c.Notifications = NotificationsConfig{Enabled: true, SmodEmail: "support@com.com"}
				c.SMTP = SMTPConfig{Port: "587", From: "sd@com.com", Timeout: "30s"}
			},
			expectErr: true,
			errSubstr: "SD_SMTP_HOST",
		},
		{
			name: "enabled without any review address fails",
			mutate: func(c *Config) {
				c.Notifications = NotificationsConfig{Enabled: true, LeaseTimeout: "60s", MaxAttempts: "5", BackoffInterval: "5m"}
				c.SMTP = SMTPConfig{Host: "smtp.otc", Port: "587", From: "sd@com.com", Timeout: "30s"}
			},
			expectErr: true,
			errSubstr: "at least one review address",
		},
		{
			name: "lease timeout not greater than smtp timeout fails",
			mutate: func(c *Config) {
				c.Notifications = NotificationsConfig{
					Enabled: true, LeaseTimeout: "30s", MaxAttempts: "5", BackoffInterval: "5m", SmodEmail: "support@com.com",
				}
				c.SMTP = SMTPConfig{Host: "smtp.otc", Port: "587", From: "sd@com.com", Timeout: "30s"}
			},
			expectErr: true,
			errSubstr: "must be greater than",
		},
		{
			name: "invalid smtp timeout fails",
			mutate: func(c *Config) {
				c.Notifications = NotificationsConfig{
					Enabled: true, LeaseTimeout: "60s", MaxAttempts: "5", BackoffInterval: "5m", SmodEmail: "support@com.com",
				}
				c.SMTP = SMTPConfig{Host: "smtp.otc", Port: "587", From: "sd@com.com", Timeout: "notaduration"}
			},
			expectErr: true,
			errSubstr: "SD_SMTP_TIMEOUT",
		},
		{
			name: "non-positive max attempts fails",
			mutate: func(c *Config) {
				c.Notifications = NotificationsConfig{
					Enabled: true, LeaseTimeout: "60s", MaxAttempts: "0", BackoffInterval: "5m", SmodEmail: "support@com.com",
				}
				c.SMTP = SMTPConfig{Host: "smtp.otc", Port: "587", From: "sd@com.com", Timeout: "30s"}
			},
			expectErr: true,
			errSubstr: "SD_NOTIFICATIONS_MAX_ATTEMPTS",
		},
		{
			name: "invalid backoff interval fails",
			mutate: func(c *Config) {
				c.Notifications = NotificationsConfig{
					Enabled: true, LeaseTimeout: "60s", MaxAttempts: "5", BackoffInterval: "bad", SmodEmail: "support@com.com",
				}
				c.SMTP = SMTPConfig{Host: "smtp.otc", Port: "587", From: "sd@com.com", Timeout: "30s"}
			},
			expectErr: true,
			errSubstr: "SD_NOTIFICATIONS_BACKOFF_INTERVAL",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseNotifConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if tc.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFillDefaults_Notifications(t *testing.T) {
	t.Run("fills notification timing defaults when enabled", func(t *testing.T) {
		c := &Config{Notifications: NotificationsConfig{Enabled: true}}
		c.FillDefaults()

		assert.Equal(t, DefaultSMTPTimeout, c.SMTP.Timeout)
		assert.Equal(t, DefaultLeaseTimeout, c.Notifications.LeaseTimeout)
		assert.Equal(t, DefaultMaxAttempts, c.Notifications.MaxAttempts)
		assert.Equal(t, DefaultBackoffInterval, c.Notifications.BackoffInterval)
	})

	t.Run("leaves notification timing empty when disabled", func(t *testing.T) {
		c := &Config{Notifications: NotificationsConfig{Enabled: false}}
		c.FillDefaults()

		assert.Empty(t, c.SMTP.Timeout)
		assert.Empty(t, c.Notifications.LeaseTimeout)
	})
}

func TestConfig_Log(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("logs without keycloak", func(t *testing.T) {
		c := &Config{
			Hostname:    "localhost",
			Port:        "8000",
			WebURL:      "http://localhost:9000",
			SecretKeyV1: "secret-key-that-is-32-chars-long",
			DB:          "postgresql://user:pass@localhost:5432/db",
			LogLevel:    "devel",
			RBAC:        RBACConfig{Admins: "admins"},
		}
		assert.NotPanics(t, func() { c.Log(logger) })
	})

	t.Run("logs with keycloak", func(t *testing.T) {
		c := &Config{
			Hostname:    "localhost",
			Port:        "8000",
			WebURL:      "http://localhost:9000",
			SecretKeyV1: "secret-key-that-is-32-chars-long",
			DB:          "postgresql://localhost:5432/db",
			LogLevel:    "devel",
			RBAC:        RBACConfig{Admins: "admins"},
			Keycloak: &Keycloak{
				URL:          "http://kc.local",
				Realm:        "test",
				ClientID:     "client",
				ClientSecret: "secret",
			},
		}
		assert.NotPanics(t, func() { c.Log(logger) })
	})
}
