package discovery

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/airshare/backend/pkg/models"
)

// DiscoveryService 设备发现服务接口
type DiscoveryService interface {
	// Start 启动设备发现服务
	Start() error
	
	// Stop 停止设备发现服务
	Stop() error
	
	// GetOnlineDevices 获取在线设备列表
	GetOnlineDevices() []*models.Device
	
	// RegisterCallback 注册设备发现回调
	RegisterCallback(callback DeviceCallback)
	
	// IsRunning 检查服务是否正在运行
	IsRunning() bool
	
	// GetStats 获取服务统计信息
	GetStats() map[string]interface{}
	
	// GetDeviceByIP 根据IP地址获取设备
	GetDeviceByIP(ip string) *models.Device
	
	// GetDeviceByID 根据设备ID获取设备
	GetDeviceByID(id string) *models.Device
}

// ServiceConfig 设备发现服务配置
type ServiceConfig struct {
	// MDNS配置
	MDNSScanInterval time.Duration `yaml:"mdns_scan_interval"`
	
	// HTTP配置
	HTTPScanTimeout  time.Duration `yaml:"http_scan_timeout"`
	HTTPBasePort     int           `yaml:"http_base_port"`
	
	// 通用配置
	AutoStart        bool          `yaml:"auto_start"`
}

// DefaultServiceConfig 默认服务配置
func DefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		MDNSScanInterval: 10 * time.Second,
		HTTPScanTimeout:  30 * time.Second,
		HTTPBasePort:     8080,
		AutoStart:        true,
	}
}

// serviceImpl 设备发现服务实现
type serviceImpl struct {
	manager      *DiscoveryManager
	config       *ServiceConfig
	callbacks    []DeviceCallback
	isRunning    bool
	mu           sync.RWMutex
}

// NewDiscoveryService 创建新的设备发现服务
func NewDiscoveryService(config *ServiceConfig) DiscoveryService {
	if config == nil {
		config = DefaultServiceConfig()
	}
	
	manager := NewDiscoveryManager(
		config.MDNSScanInterval,
		config.HTTPScanTimeout,
		config.HTTPBasePort,
	)
	
	service := &serviceImpl{
		manager:   manager,
		config:    config,
		callbacks: make([]DeviceCallback, 0),
	}
	
	// 注册内部回调，用于将管理器事件转发给服务回调
	manager.RegisterCallback(service.handleDeviceChange)
	
	return service
}

// Start 启动设备发现服务
func (s *serviceImpl) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if s.isRunning {
		return fmt.Errorf("discovery service is already running")
	}
	
	if err := s.manager.Start(); err != nil {
		return fmt.Errorf("failed to start discovery manager: %w", err)
	}
	
	s.isRunning = true
	log.Println("Discovery service started")
	
	return nil
}

// Stop 停止设备发现服务
func (s *serviceImpl) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if !s.isRunning {
		return nil
	}
	
	if err := s.manager.Stop(); err != nil {
		return fmt.Errorf("failed to stop discovery manager: %w", err)
	}
	
	s.isRunning = false
	log.Println("Discovery service stopped")
	
	return nil
}

// GetOnlineDevices 获取在线设备列表
func (s *serviceImpl) GetOnlineDevices() []*models.Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if !s.isRunning {
		return []*models.Device{}
	}
	
	return s.manager.GetOnlineDevices()
}

// RegisterCallback 注册设备发现回调
func (s *serviceImpl) RegisterCallback(callback DeviceCallback) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.callbacks = append(s.callbacks, callback)
}

// IsRunning 检查服务是否正在运行
func (s *serviceImpl) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return s.isRunning
}

// GetStats 获取服务统计信息
func (s *serviceImpl) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if !s.isRunning {
		return map[string]interface{}{
			"isRunning": false,
			"message":  "Service is not running",
		}
	}
	
	stats := s.manager.GetStats()
	stats["config"] = s.config
	
	return stats
}

// GetDeviceByIP 根据IP地址获取设备
func (s *serviceImpl) GetDeviceByIP(ip string) *models.Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if !s.isRunning {
		return nil
	}
	
	return s.manager.GetDeviceByIP(ip)
}

// GetDeviceByID 根据设备ID获取设备
func (s *serviceImpl) GetDeviceByID(id string) *models.Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if !s.isRunning {
		return nil
	}
	
	return s.manager.GetDeviceByID(id)
}

// handleDeviceChange 处理设备变化事件
func (s *serviceImpl) handleDeviceChange(device *models.Device, action Action) {
	s.mu.RLock()
	callbacks := make([]DeviceCallback, len(s.callbacks))
	copy(callbacks, s.callbacks)
	s.mu.RUnlock()
	
	// 通知所有注册的回调
	for _, callback := range callbacks {
		go func(cb DeviceCallback) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Discovery service callback panic: %v", r)
				}
			}()
			
			cb(device, action)
		}(callback)
	}
}

// ServiceFactory 设备发现服务工厂
func ServiceFactory(config *ServiceConfig) DiscoveryService {
	service := NewDiscoveryService(config)
	
	// 如果配置为自动启动，则立即启动服务
	if config != nil && config.AutoStart {
		if err := service.Start(); err != nil {
			log.Printf("Failed to auto-start discovery service: %v", err)
		}
	}
	
	return service
}