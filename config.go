package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config 程序配置
type Config struct {
	Host string `yaml:"host"` // 绑定地址
	Port int    `yaml:"port"` // 监听端口
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Host: "",
		Port: 9999,
	}
}

// LoadConfig 从 config.yaml 加载配置（如果存在）
func LoadConfig() (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile("config.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			// 配置文件不存在，使用默认配置
			return cfg, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return cfg, nil
}
