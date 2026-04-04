package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// FlexibleStringSlice is a []string that also accepts JSON numbers,
// so allow_from can contain both "123" and 123.
type FlexibleStringSlice []string

func (f *FlexibleStringSlice) UnmarshalJSON(data []byte) error {
	// Try []string first
	var ss []string
	if err := json.Unmarshal(data, &ss); err == nil {
		*f = ss
		return nil
	}

	// Try []interface{} to handle mixed types
	var raw []interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	result := make([]string, 0, len(raw))
	for _, v := range raw {
		switch val := v.(type) {
		case string:
			result = append(result, val)
		case float64:
			result = append(result, fmt.Sprintf("%.0f", val))
		default:
			result = append(result, fmt.Sprintf("%v", val))
		}
	}
	*f = result
	return nil
}

type Config struct {
	Agents    AgentsConfig    `json:"agents"`
	Channels  ChannelsConfig  `json:"channels"`
	Providers ProvidersConfig `json:"providers"`
	Gateway   GatewayConfig   `json:"gateway"`
	API       APIConfig       `json:"api"`
	Tools     ToolsConfig     `json:"tools"`
	Heartbeat HeartbeatConfig `json:"heartbeat"`
	Devices   DevicesConfig   `json:"devices"`
	mu        sync.RWMutex
}

type AgentsConfig struct {
	Defaults AgentDefaults `json:"defaults"`
}

type AgentDefaults struct {
	Workspace           string  `json:"workspace" env:"RDXCLAW_WORKSPACE"`
	RestrictToWorkspace bool    `json:"restrict_to_workspace" env:"RDXCLAW_RESTRICT_TO_WORKSPACE"`
	Provider            string  `json:"provider" env:"RDXCLAW_PROVIDER"`
	Model               string  `json:"model" env:"RDXCLAW_MODEL"`
	MaxTokens           int     `json:"max_tokens" env:"RDXCLAW_MAX_TOKENS"`
	Temperature         float64 `json:"temperature" env:"RDXCLAW_TEMPERATURE"`
	MaxToolIterations   int     `json:"max_tool_iterations" env:"RDXCLAW_MAX_TOOL_ITERATIONS"`
	Timeout             int     `json:"timeout" env:"RDXCLAW_TIMEOUT"` // In seconds
}

type ChannelsConfig struct {
	WhatsApp WhatsAppConfig `json:"whatsapp"`
	Telegram TelegramConfig `json:"telegram"`
	Discord  DiscordConfig  `json:"discord"`
	Slack    SlackConfig    `json:"slack"`
	LINE     LINEConfig     `json:"line"`
}

type WhatsAppConfig struct {
	Enabled   bool                `json:"enabled" env:"RDXCLAW_CHANNELS_WHATSAPP_ENABLED"`
	BridgeURL string              `json:"bridge_url" env:"RDXCLAW_CHANNELS_WHATSAPP_BRIDGE_URL"`
	AllowFrom FlexibleStringSlice `json:"allow_from" env:"RDXCLAW_CHANNELS_WHATSAPP_ALLOW_FROM"`
}

type TelegramConfig struct {
	Enabled   bool                `json:"enabled" env:"RDXCLAW_CHANNELS_TELEGRAM_ENABLED"`
	Token     string              `json:"token" env:"RDXCLAW_CHANNELS_TELEGRAM_TOKEN"`
	Proxy     string              `json:"proxy" env:"RDXCLAW_CHANNELS_TELEGRAM_PROXY"`
	AllowFrom FlexibleStringSlice `json:"allow_from" env:"RDXCLAW_CHANNELS_TELEGRAM_ALLOW_FROM"`
}

type DiscordConfig struct {
	Enabled   bool                `json:"enabled" env:"RDXCLAW_CHANNELS_DISCORD_ENABLED"`
	Token     string              `json:"token" env:"RDXCLAW_CHANNELS_DISCORD_TOKEN"`
	AllowFrom FlexibleStringSlice `json:"allow_from" env:"RDXCLAW_CHANNELS_DISCORD_ALLOW_FROM"`
}

type SlackConfig struct {
	Enabled   bool                `json:"enabled" env:"RDXCLAW_CHANNELS_SLACK_ENABLED"`
	BotToken  string              `json:"bot_token" env:"RDXCLAW_CHANNELS_SLACK_BOT_TOKEN"`
	AppToken  string              `json:"app_token" env:"RDXCLAW_CHANNELS_SLACK_APP_TOKEN"`
	AllowFrom FlexibleStringSlice `json:"allow_from" env:"RDXCLAW_CHANNELS_SLACK_ALLOW_FROM"`
}

type LINEConfig struct {
	Enabled            bool                `json:"enabled" env:"RDXCLAW_CHANNELS_LINE_ENABLED"`
	ChannelSecret      string              `json:"channel_secret" env:"RDXCLAW_CHANNELS_LINE_CHANNEL_SECRET"`
	ChannelAccessToken string              `json:"channel_access_token" env:"RDXCLAW_CHANNELS_LINE_CHANNEL_ACCESS_TOKEN"`
	WebhookHost        string              `json:"webhook_host" env:"RDXCLAW_CHANNELS_LINE_WEBHOOK_HOST"`
	WebhookPort        int                 `json:"webhook_port" env:"RDXCLAW_CHANNELS_LINE_WEBHOOK_PORT"`
	WebhookPath        string              `json:"webhook_path" env:"RDXCLAW_CHANNELS_LINE_WEBHOOK_PATH"`
	AllowFrom          FlexibleStringSlice `json:"allow_from" env:"RDXCLAW_CHANNELS_LINE_ALLOW_FROM"`
}

type HeartbeatConfig struct {
	Enabled  bool `json:"enabled" env:"RDXCLAW_HEARTBEAT_ENABLED"`
	Interval int  `json:"interval" env:"RDXCLAW_HEARTBEAT_INTERVAL"` // minutes, min 5
}

type DevicesConfig struct {
	Enabled    bool `json:"enabled" env:"RDXCLAW_DEVICES_ENABLED"`
	MonitorUSB bool `json:"monitor_usb" env:"RDXCLAW_DEVICES_MONITOR_USB"`
}

type ProvidersConfig struct {
	Anthropic     ProviderConfig `json:"anthropic" envPrefix:"RDXCLAW_ANTHROPIC_"`
	OpenAI        ProviderConfig `json:"openai" envPrefix:"RDXCLAW_OPENAI_"`
	OpenRouter    ProviderConfig `json:"openrouter" envPrefix:"RDXCLAW_OPENROUTER_"`
	Groq          ProviderConfig `json:"groq" envPrefix:"RDXCLAW_GROQ_"`
	VLLM          ProviderConfig `json:"vllm" envPrefix:"RDXCLAW_VLLM_"`
	Gemini        ProviderConfig `json:"gemini" envPrefix:"RDXCLAW_GEMINI_"`
	Nvidia        ProviderConfig `json:"nvidia" envPrefix:"RDXCLAW_NVIDIA_"`
	Ollama        ProviderConfig `json:"ollama" envPrefix:"RDXCLAW_OLLAMA_"`
	DeepSeek      ProviderConfig `json:"deepseek" envPrefix:"RDXCLAW_DEEPSEEK_"`
	GitHubCopilot ProviderConfig `json:"github_copilot" envPrefix:"RDXCLAW_GITHUB_COPILOT_"`
	ShengSuanYun  ProviderConfig `json:"shengsuanyun" envPrefix:"RDXCLAW_SHENGSUANYUN_"`
}

type ProviderConfig struct {
	APIKey      string `json:"api_key" env:"API_KEY"`
	APIBase     string `json:"api_base" env:"API_BASE"`
	Proxy       string `json:"proxy,omitempty" env:"PROXY"`
	AuthMethod  string `json:"auth_method,omitempty" env:"AUTH_METHOD"`
	ConnectMode string `json:"connect_mode,omitempty" env:"CONNECT_MODE"` //only for Github Copilot, `stdio` or `grpc`
}

type GatewayConfig struct {
	Host string `json:"host" env:"RDXCLAW_GATEWAY_HOST"`
	Port int    `json:"port" env:"RDXCLAW_GATEWAY_PORT"`
}

type APIConfig struct {
	Enabled     bool                `json:"enabled" env:"RDXCLAW_API_ENABLED"`
	Host        string              `json:"host" env:"RDXCLAW_API_HOST"`
	Port        int                 `json:"port" env:"RDXCLAW_API_PORT"`
	APIKey      string              `json:"api_key" env:"RDXCLAW_SERVER_API_KEY"`
	RateLimit   int                 `json:"rate_limit" env:"RDXCLAW_API_RATE_LIMIT"` // requests per minute
	CORSOrigins FlexibleStringSlice `json:"cors_origins" env:"RDXCLAW_API_CORS_ORIGINS"`
}

type BraveConfig struct {
	Enabled    bool   `json:"enabled" env:"RDXCLAW_TOOLS_WEB_BRAVE_ENABLED"`
	APIKey     string `json:"api_key" env:"RDXCLAW_TOOLS_WEB_BRAVE_API_KEY"`
	MaxResults int    `json:"max_results" env:"RDXCLAW_TOOLS_WEB_BRAVE_MAX_RESULTS"`
}

type DuckDuckGoConfig struct {
	Enabled    bool `json:"enabled" env:"RDXCLAW_TOOLS_WEB_DUCKDUCKGO_ENABLED"`
	MaxResults int  `json:"max_results" env:"RDXCLAW_TOOLS_WEB_DUCKDUCKGO_MAX_RESULTS"`
}

type WebToolsConfig struct {
	Brave      BraveConfig      `json:"brave"`
	DuckDuckGo DuckDuckGoConfig `json:"duckduckgo"`
}

type ToolsConfig struct {
	Web WebToolsConfig `json:"web"`
}

func DefaultConfig() *Config {
	return &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:           "~/.rdxclaw/workspace",
				RestrictToWorkspace: true,
				Provider:            "",
				Model:               "gpt-4o",
				MaxTokens:           8192,
				Temperature:         0.7,
				MaxToolIterations:   20,
				Timeout:             600,
			},
		},
		Channels: ChannelsConfig{
			WhatsApp: WhatsAppConfig{
				Enabled:   false,
				BridgeURL: "ws://localhost:3001",
				AllowFrom: FlexibleStringSlice{},
			},
			Telegram: TelegramConfig{
				Enabled:   false,
				Token:     "",
				AllowFrom: FlexibleStringSlice{},
			},
			Discord: DiscordConfig{
				Enabled:   false,
				Token:     "",
				AllowFrom: FlexibleStringSlice{},
			},
			Slack: SlackConfig{
				Enabled:   false,
				BotToken:  "",
				AppToken:  "",
				AllowFrom: FlexibleStringSlice{},
			},
			LINE: LINEConfig{
				Enabled:            false,
				ChannelSecret:      "",
				ChannelAccessToken: "",
				WebhookHost:        "0.0.0.0",
				WebhookPort:        18791,
				WebhookPath:        "/webhook/line",
				AllowFrom:          FlexibleStringSlice{},
			},
		},
		Providers: ProvidersConfig{
			Anthropic:  ProviderConfig{},
			OpenAI:     ProviderConfig{},
			OpenRouter: ProviderConfig{},
			Groq:       ProviderConfig{},
			VLLM:       ProviderConfig{},
			Gemini:     ProviderConfig{},
			Nvidia:     ProviderConfig{},
		},
		Gateway: GatewayConfig{
			Host: "0.0.0.0",
			Port: 18790,
		},
		API: APIConfig{
			Enabled:     true,
			Host:        "0.0.0.0",
			Port:        8080,
			APIKey:      "", // generated or set by user
			RateLimit:   60,
			CORSOrigins: FlexibleStringSlice{"*"},
		},
		Tools: ToolsConfig{
			Web: WebToolsConfig{
				Brave: BraveConfig{
					Enabled:    false,
					APIKey:     "",
					MaxResults: 5,
				},
				DuckDuckGo: DuckDuckGoConfig{
					Enabled:    true,
					MaxResults: 5,
				},
			},
		},
		Heartbeat: HeartbeatConfig{
			Enabled:  true,
			Interval: 30, // default 30 minutes
		},
		Devices: DevicesConfig{
			Enabled:    false,
			MonitorUSB: true,
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	// Attempt to load environment variables from file first for seamless experience
	// Load from current dir .env or home dir config env
	_ = godotenv.Load() // Loads .env from current dir if exists
	home, err := os.UserHomeDir()
	if err == nil {
		envPath := filepath.Join(home, ".rdxclaw", "env")
		_ = godotenv.Load(envPath)
	}

	cfg := DefaultConfig()

	// Gosec ignored: path is the application config path.
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			// If file doesn't exist, we still want to parse env vars
		} else {
			return nil, err
		}
	} else if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	// Handle global RDXCLAW_API_KEY / RDXCLAW_APIBASE overrides for the active provider
	globalKey := os.Getenv("RDXCLAW_API_KEY")
	globalBase := os.Getenv("RDXCLAW_APIBASE")

	// Fallback for server API key: if RDXCLAW_SERVER_API_KEY is not set, 
	// use RDXCLAW_API_KEY (seamless experience for single-key setups)
	if cfg.API.APIKey == "" && globalKey != "" {
		cfg.API.APIKey = globalKey
	}

	if globalKey != "" || globalBase != "" {
		provider := cfg.Agents.Defaults.Provider
		if provider == "" {
			provider = "openai" // default fallback
		}

		// Apply overrides to the active provider configuration
		updateProviderConfig(cfg, provider, globalKey, globalBase)
	}

	return cfg, nil
}

func updateProviderConfig(cfg *Config, providerName string, key string, base string) {
	var target *ProviderConfig

	switch providerName {
	case "anthropic":
		target = &cfg.Providers.Anthropic
	case "openai":
		target = &cfg.Providers.OpenAI
	case "openrouter":
		target = &cfg.Providers.OpenRouter
	case "groq":
		target = &cfg.Providers.Groq
	case "vllm":
		target = &cfg.Providers.VLLM
	case "gemini":
		target = &cfg.Providers.Gemini
	case "nvidia":
		target = &cfg.Providers.Nvidia
	case "ollama":
		target = &cfg.Providers.Ollama
	case "deepseek":
		target = &cfg.Providers.DeepSeek
	case "github_copilot":
		target = &cfg.Providers.GitHubCopilot
	case "shengsuanyun":
		target = &cfg.Providers.ShengSuanYun
	}

	if target != nil {
		if key != "" {
			target.APIKey = key
		}
		if base != "" {
			target.APIBase = base
		}
	}
}

func SaveConfig(path string, cfg *Config) error {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil { // #nosec G301
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func (c *Config) WorkspacePath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return expandHome(c.Agents.Defaults.Workspace)
}

func (c *Config) GetAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Providers.OpenRouter.APIKey != "" {
		return c.Providers.OpenRouter.APIKey
	}
	if c.Providers.Anthropic.APIKey != "" {
		return c.Providers.Anthropic.APIKey
	}
	if c.Providers.OpenAI.APIKey != "" {
		return c.Providers.OpenAI.APIKey
	}
	if c.Providers.Gemini.APIKey != "" {
		return c.Providers.Gemini.APIKey
	}
	if c.Providers.Groq.APIKey != "" {
		return c.Providers.Groq.APIKey
	}
	if c.Providers.VLLM.APIKey != "" {
		return c.Providers.VLLM.APIKey
	}
	if c.Providers.ShengSuanYun.APIKey != "" {
		return c.Providers.ShengSuanYun.APIKey
	}
	return ""
}

func (c *Config) GetAPIBase() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Providers.OpenRouter.APIKey != "" {
		if c.Providers.OpenRouter.APIBase != "" {
			return c.Providers.OpenRouter.APIBase
		}
		return "https://openrouter.ai/api/v1"
	}
	if c.Providers.VLLM.APIKey != "" && c.Providers.VLLM.APIBase != "" {
		return c.Providers.VLLM.APIBase
	}
	return ""
}

func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return home + path[1:]
		}
		return home
	}
	return path
}
