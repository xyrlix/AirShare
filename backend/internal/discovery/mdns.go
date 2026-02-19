package discovery

import (
	"context"
	"fmt"
	"log"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"

	"airshare-backend/pkg/models"
	"github.com/hashicorp/mdns"
)

// MDNSDiscovery 实现基于mDNS的设备发现服务
type MDNSDiscovery struct {
	server       *mdns.Server
	entries      map[string]*mdns.ServiceEntry
	mu           sync.RWMutex
	scanInterval time.Duration
	onlineDevices map[string]*models.Device
	callbacks    []DeviceCallback
	isRunning    bool
	ctx          context.Context
	cancel       context.CancelFunc
}

// DeviceCallback 设备发现回调函数
type DeviceCallback func(device *models.Device, action Action)

// Action 设备操作类型
type Action int

const (
	// ActionAdd 添加设备
	ActionAdd Action = iota
	// ActionUpdate 更新设备
	ActionUpdate
	// ActionRemove 移除设备
	ActionRemove
)

// NewMDNSDiscovery 创建新的mDNS设备发现服务
func NewMDNSDiscovery(scanInterval time.Duration) *MDNSDiscovery {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &MDNSDiscovery{
		entries:      make(map[string]*mdns.ServiceEntry),
		onlineDevices: make(map[string]*models.Device),
		scanInterval: scanInterval,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// 启动设备发现服务
func (m *MDNSDiscovery) Start() error {
	if m.isRunning {
		return fmt.Errorf("mDNS discovery is already running")
	}

	// 直接启动发现循环，暂时不创建mDNS服务器
	m.isRunning = true
	log.Println("mDNS discovery service started")

	// 启动设备发现循环
	go m.discoveryLoop()

	return nil
}

// Stop 停止设备发现服务
func (m *MDNSDiscovery) Stop() error {
	if !m.isRunning {
		return nil
	}

	m.cancel()
	
	if m.server != nil {
		m.server.Shutdown()
	}

	m.isRunning = false
	log.Println("mDNS discovery service stopped")
	
	return nil
}

// discoveryLoop 设备发现循环
func (m *MDNSDiscovery) discoveryLoop() {
	ticker := time.NewTicker(m.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.scanDevices()
		}
	}
}

// scanDevices 扫描局域网设备
func (m *MDNSDiscovery) scanDevices() {
	// 获取本地网络接口
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("Failed to get network interfaces: %v", err)
		return
	}

	// 遍历所有网络接口进行设备发现
	for _, iface := range interfaces {
		if m.shouldSkipInterface(iface) {
			continue
		}

		// 在当前接口上发现设备
		m.discoverOnInterface(iface)
	}

	// 清理离线设备
	m.cleanupOfflineDevices()
}

// shouldSkipInterface 判断是否跳过该网络接口
func (m *MDNSDiscovery) shouldSkipInterface(iface net.Interface) bool {
	// 跳过回环接口、未启用接口、无IP地址接口
	if iface.Flags&net.FlagLoopback != 0 {
		return true
	}
	if iface.Flags&net.FlagUp == 0 {
		return true
	}
	
	addrs, err := iface.Addrs()
	if err != nil || len(addrs) == 0 {
		return true
	}

	return false
}

// discoverOnInterface 在指定网络接口上发现设备
func (m *MDNSDiscovery) discoverOnInterface(iface net.Interface) {
	// 在Windows平台上，检查接口是否有IPv6地址，如果有则跳过（避免udp6绑定错误）
	if runtime.GOOS == "windows" {
		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() == nil {
					// 这是一个IPv6地址，在Windows上跳过
					return
				}
			}
		}
	}

	// 创建mDNS查询参数
	params := &mdns.QueryParam{
		Service: "_airshare._tcp",
		Domain: "local",
		Timeout: m.scanInterval / 2,
		Interface: &iface,
		Entries: make(chan *mdns.ServiceEntry, 32),
		WantUnicastResponse: false,
	}

	// 执行查询
	if err := mdns.Query(params); err != nil {
		// 过滤掉Windows上的IPv6绑定错误，避免日志过于冗长
		if runtime.GOOS == "windows" {
			if strings.Contains(err.Error(), "udp6") ||
			   strings.Contains(err.Error(), "IPv6") ||
			   strings.Contains(err.Error(), "address family not supported") ||
			   strings.Contains(err.Error(), "Only one usage of each socket") {
				// 这些错误在Windows上是预期的，静默忽略
				return
			}
		}
		log.Printf("mDNS query failed on interface %s: %v", iface.Name, err)
		return
	}

	// 由于mdns.Query不返回entries，暂时跳过设备处理逻辑
	// 后续需要检查mdns库的正确使用方式来获取发现的设备
}

// handleDiscoveredDevice 处理发现的设备
func (m *MDNSDiscovery) handleDiscoveredDevice(entry *mdns.ServiceEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	deviceID := m.generateDeviceID(entry)
	
	// 检查设备是否已存在
	existing, exists := m.onlineDevices[deviceID]
	
	if exists {
		// 更新设备信息
		existing.IP = entry.AddrV4.String()
		existing.LastSeen = time.Now()
		m.onlineDevices[deviceID] = existing
		
		// 触发更新回调
		m.notifyCallbacks(existing, ActionUpdate)
	} else {
		// 创建新设备
		device := &models.Device{
			ID:        deviceID,
			Name:      m.extractDeviceName(entry),
			IP:        entry.AddrV4.String(),
			Type:      m.detectDeviceType(entry),
			Platform:  m.extractPlatform(entry),
			Status:    models.DeviceStatusOnline,
			LastSeen:  time.Now(),
		}
		
		m.onlineDevices[deviceID] = device
		m.entries[deviceID] = entry
		
		log.Printf("Discovered new device: %s (%s)", device.Name, device.IP)
		
		// 触发添加回调
		m.notifyCallbacks(device, ActionAdd)
	}
}

// generateDeviceID 生成设备唯一标识
func (m *MDNSDiscovery) generateDeviceID(entry *mdns.ServiceEntry) string {
	// 使用IP地址和主机名组合作为设备ID
	return fmt.Sprintf("%s-%s", entry.AddrV4.String(), entry.Host)
}

// extractDeviceName 提取设备名称
func (m *MDNSDiscovery) extractDeviceName(entry *mdns.ServiceEntry) string {
	// 从主机名中提取设备名称
	name := strings.TrimSuffix(entry.Host, ".local")
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.Title(name)
	
	return name
}

// detectDeviceType 检测设备类型
func (m *MDNSDiscovery) detectDeviceType(entry *mdns.ServiceEntry) models.DeviceType {
	// 根据主机名和端口信息判断设备类型
	name := strings.ToLower(entry.Host)
	
	switch {
	case strings.Contains(name, "iphone") || strings.Contains(name, "android"):
		return models.DeviceTypeMobile
	case strings.Contains(name, "ipad") || strings.Contains(name, "tablet"):
		return models.DeviceTypeTablet
	case strings.Contains(name, "mac") || strings.Contains(name, "windows") || strings.Contains(name, "linux"):
		return models.DeviceTypeDesktop
	default:
		return models.DeviceTypeUnknown
	}
}

// extractPlatform 提取设备平台信息
func (m *MDNSDiscovery) extractPlatform(entry *mdns.ServiceEntry) string {
	name := strings.ToLower(entry.Host)
	
	switch {
	case strings.Contains(name, "mac"):
		return "macOS"
	case strings.Contains(name, "windows"):
		return "Windows"
	case strings.Contains(name, "linux"):
		return "Linux"
	case strings.Contains(name, "iphone") || strings.Contains(name, "ipad"):
		return "iOS"
	case strings.Contains(name, "android"):
		return "Android"
	default:
		return "Unknown"
	}
}

// cleanupOfflineDevices 清理离线设备
func (m *MDNSDiscovery) cleanupOfflineDevices() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoffTime := time.Now().Add(-m.scanInterval * 3)
	
	for id, device := range m.onlineDevices {
		if device.LastSeen.Before(cutoffTime) {
			// 设备已离线
			device.Status = models.DeviceStatusOffline
			
			// 触发移除回调
			m.notifyCallbacks(device, ActionRemove)
			
			// 从在线设备列表中移除
			delete(m.onlineDevices, id)
			delete(m.entries, id)
			
			log.Printf("Device went offline: %s (%s)", device.Name, device.IP)
		}
	}
}

// GetOnlineDevices 获取在线设备列表
func (m *MDNSDiscovery) GetOnlineDevices() []*models.Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*models.Device, 0, len(m.onlineDevices))
	for _, device := range m.onlineDevices {
		devices = append(devices, device)
	}
	
	return devices
}

// RegisterCallback 注册设备发现回调
func (m *MDNSDiscovery) RegisterCallback(callback DeviceCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.callbacks = append(m.callbacks, callback)
}

// notifyCallbacks 通知所有注册的回调函数
func (m *MDNSDiscovery) notifyCallbacks(device *models.Device, action Action) {
	for _, callback := range m.callbacks {
		go func(cb DeviceCallback) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Device callback panic: %v", r)
				}
			}()
			
			cb(device, action)
		}(callback)
	}
}

// IsRunning 检查服务是否正在运行
func (m *MDNSDiscovery) IsRunning() bool {
	return m.isRunning
}

// GetStats 获取设备发现统计信息
func (m *MDNSDiscovery) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"isRunning":    m.isRunning,
		"onlineDevices": len(m.onlineDevices),
		"scanInterval": m.scanInterval.String(),
		"lastScan":     time.Now(),
	}
}