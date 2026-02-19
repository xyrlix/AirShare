package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 全局配置结构体
type Config struct {
	Server    ServerConfig    "yaml:\"server\""
	Discovery DiscoveryConfig "yaml:\"discovery\""
	Transfer  TransferConfig  "yaml:\"transfer\""
	Security  SecurityConfig  "yaml:\"security\""
}

// ServerConfig HTTP服务器配置
type ServerConfig struct {
	Port    int    "yaml:\"port\""
	Host    string "yaml:\"host\""
	WebRoot string "yaml:\"web_root\""
}

// DiscoveryConfig 设备发现配置
type DiscoveryConfig struct {
	ServiceName string "yaml:\"service_name\""
	Domain      string "yaml:\"domain\""
	Port        int    "yaml:\"port\""
	Enabled     bool   "yaml:\"enabled\""
}

// TransferConfig 文件传输配置
type TransferConfig struct {
	StoragePath   string "yaml:\"storage_path\""
	MaxFileSize   int64  "yaml:\"max_file_size\""
	ChunkSize     int    "yaml:\"chunk_size\""
	EnableResume  bool   "yaml:\"enable_resume\""
	CleanupPeriod int    "yaml:\"cleanup_period\""
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	EnableTLS     bool   "yaml:\"enable_tls\""
	CertFile      string "yaml:\"cert_file\""
	KeyFile       string "yaml:\"key_file\""
	EnableCORS    bool   "yaml:\"enable_cors\""
	AllowedOrigins []string "yaml:\"allowed_origins\""
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	cwd, _ := os.Getwd()
	storagePath := filepath.Join(cwd, "storage")

	return &Config{
		Server: ServerConfig{
			Port:    8080,
			Host:    "0.0.0.0",
			WebRoot: filepath.Join(cwd, "../frontend/build/web"),
		},
		Discovery: DiscoveryConfig{
			ServiceName: "_airshare._tcp",
			Domain:      "local.",
			Port:        5353,
			Enabled:     true,
		},
		Transfer: TransferConfig{
			StoragePath:   storagePath,
			MaxFileSize:   1024 * 1024 * 1024, // 1GB
			ChunkSize:     64 * 1024,          // 64KB
			EnableResume:  true,
			CleanupPeriod: 24, // 小时
		},
		Security: SecurityConfig{
			EnableTLS:      false,
			EnableCORS:     true,
			AllowedOrigins: []string{"*"},
		},
	}
}

// LoadConfig 从文件加载配置
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(cfg *Config, filename string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}