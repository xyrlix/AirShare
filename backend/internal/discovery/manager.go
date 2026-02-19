package discovery

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"airshare-backend/pkg/models"
)

// DiscoveryManager 设备发现管理器
type DiscoveryManager struct {
	mdnsDiscovery   *MDNSDiscovery
	httpDiscovery   *HTTPDiscovery
	combinedDevices map[string]*models.Device
	callbacks       []DeviceCallback
	mu              sync.RWMutex
	isRunning       bool
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewDiscoveryManager 创建新的设备发现管理器
func NewDiscoveryManager(mdnsScanInterval, httpScanTimeout time.Duration, httpBasePort int) *DiscoveryManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &DiscoveryManager{
		mdnsDiscovery:   NewMDNSDiscovery(mdnsScanInterval),
		httpDiscovery:   NewHTTPDiscovery(httpBasePort, httpScanTimeout),
		combinedDevices: make(map[string]*models.Device),
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start 启动设备发现管理器
func (dm *DiscoveryManager) Start() error {
	if dm.isRunning {
		return fmt.Errorf("discovery manager is already running")
	}

	// 启动mDNS发现服务
	if err := dm.mdnsDiscovery.Start(); err != nil {
		log.Printf("Failed to start mDNS discovery: %v", err)
		return err
	}

	// 启动HTTP发现服务
	if err := dm.httpDiscovery.Start(); err != nil {
		log.Printf("Failed to start HTTP discovery: %v", err)
		_ = dm.mdnsDiscovery.Stop()
		return err
	}

	// 注册mDNS回调
	dm.mdnsDiscovery.RegisterCallback(dm.handleDeviceChange)

	dm.isRunning = true
	log.Println("Discovery manager started")

	// 启动设备合并循环
	go dm.deviceMergeLoop()

	return nil
}

// Stop 停止设备发现管理器
func (dm *DiscoveryManager) Stop() error {
	if !dm.isRunning {
		return nil
	}

	dm.cancel()
	
	// 停止所有发现服务
	_ = dm.mdnsDiscovery.Stop()
	_ = dm.httpDiscovery.Stop()

	dm.isRunning = false
	log.Println("Discovery manager stopped")
	
	return nil
}

// deviceMergeLoop 设备合并循环
func (dm *DiscoveryManager) deviceMergeLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-dm.ctx.Done():
			return
		case <-ticker.C:
			dm.mergeDevices()
		}
	}
}

// mergeDevices 合并来自不同发现服务的设备
func (dm *DiscoveryManager) mergeDevices() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 获取所有在线设备
	allDevices := make(map[string]*models.Device)
	
	// 添加mDNS发现的设备
	mdnsDevices := dm.mdnsDiscovery.GetOnlineDevices()
	for _, device := range mdnsDevices {
		allDevices[device.ID] = device
	}
	
	// 添加HTTP发现的设备
	httpDevices := dm.httpDiscovery.GetOnlineDevices()
	for _, device := range httpDevices {
		allDevices[device.ID] = device
	}
	
	// 检测并合并重复设备
	dm.detectAndMergeDuplicates(allDevices)
	
	// 更新合并后的设备列表
	dm.updateCombinedDevices(allDevices)
}

// detectAndMergeDuplicates 检测并合并重复设备
func (dm *DiscoveryManager) detectAndMergeDuplicates(devices map[string]*models.Device) {
	// 基于IP地址检测重复设备
	ipMap := make(map[string][]string)
	
	for deviceID, device := range devices {
		ipMap[device.IP] = append(ipMap[device.IP], deviceID)
	}
	
	for ip, deviceIDs := range ipMap {
		if len(deviceIDs) > 1 {
			// 发现重复设备，进行合并
			dm.mergeDuplicateDevices(devices, deviceIDs, ip)
		}
	}
}

// mergeDuplicateDevices 合并重复设备
func (dm *DiscoveryManager) mergeDuplicateDevices(devices map[string]*models.Device, deviceIDs []string, ip string) {
	// 选择最完整的设备信息作为主设备
	var primaryDevice *models.Device
	var maxScore int
	
	for _, deviceID := range deviceIDs {
		device := devices[deviceID]
		score := dm.calculateDeviceInfoScore(device)
		
		if score > maxScore {
			maxScore = score
			primaryDevice = device
		}
	}
	
	if primaryDevice != nil {
		// 更新主设备信息
		primaryDevice.IP = ip
		primaryDevice.LastSeen = time.Now()
		
		// 删除其他重复设备
		for _, deviceID := range deviceIDs {
			if deviceID != primaryDevice.ID {
				delete(devices, deviceID)
			}
		}
	}
}

// calculateDeviceInfoScore 计算设备信息完整度分数
func (dm *DiscoveryManager) calculateDeviceInfoScore(device *models.Device) int {
	score := 0
	
	if device.Name != "" && device.Name != fmt.Sprintf("设备-%s", device.IP) {
		score += 10
	}
	
	if device.Type != models.DeviceTypeUnknown {
		score += 5
	}
	
	if device.Platform != "Unknown" {
		score += 5
	}
	
	return score
}

// updateCombinedDevices 更新合并后的设备列表
func (dm *DiscoveryManager) updateCombinedDevices(newDevices map[string]*models.Device) {
	// 检测新增设备
	for deviceID, device := range newDevices {
		if _, exists := dm.combinedDevices[deviceID]; !exists {
			// 新增设备
			dm.combinedDevices[deviceID] = device
			dm.notifyCallbacks(device, ActionAdd)
		}
	}
	
	// 检测离线设备
	for deviceID, oldDevice := range dm.combinedDevices {
		if _, exists := newDevices[deviceID]; !exists {
			// 设备离线
			oldDevice.Status = models.DeviceStatusOffline
			dm.notifyCallbacks(oldDevice, ActionRemove)
			delete(dm.combinedDevices, deviceID)
		}
	}
}

// handleDeviceChange 处理设备变化
func (dm *DiscoveryManager) handleDeviceChange(device *models.Device, action Action) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	// 根据动作类型处理设备
	switch action {
	case ActionAdd:
		// 新设备加入
		if _, exists := dm.combinedDevices[device.ID]; !exists {
			dm.combinedDevices[device.ID] = device
			dm.notifyCallbacks(device, ActionAdd)
		}
	case ActionUpdate:
		// 设备信息更新
		if existing, exists := dm.combinedDevices[device.ID]; exists {
			*existing = *device
			dm.notifyCallbacks(device, ActionUpdate)
		}
	case ActionRemove:
		// 设备离线
		if existing, exists := dm.combinedDevices[device.ID]; exists {
			existing.Status = models.DeviceStatusOffline
			dm.notifyCallbacks(existing, ActionRemove)
			delete(dm.combinedDevices, device.ID)
		}
	}
}

// GetOnlineDevices 获取所有在线设备
func (dm *DiscoveryManager) GetOnlineDevices() []*models.Device {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	devices := make([]*models.Device, 0, len(dm.combinedDevices))
	for _, device := range dm.combinedDevices {
		if device.Status == models.DeviceStatusOnline {
			devices = append(devices, device)
		}
	}
	
	return devices
}

// RegisterCallback 注册设备发现回调
func (dm *DiscoveryManager) RegisterCallback(callback DeviceCallback) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	
	dm.callbacks = append(dm.callbacks, callback)
}

// notifyCallbacks 通知所有注册的回调函数
func (dm *DiscoveryManager) notifyCallbacks(device *models.Device, action Action) {
	for _, callback := range dm.callbacks {
		go func(cb DeviceCallback) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Discovery manager callback panic: %v", r)
				}
			}()
			
			cb(device, action)
		}(callback)
	}
}

// IsRunning 检查管理器是否正在运行
func (dm *DiscoveryManager) IsRunning() bool {
	return dm.isRunning
}

// GetStats 获取设备发现统计信息
func (dm *DiscoveryManager) GetStats() map[string]interface{} {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	mdnsStats := dm.mdnsDiscovery.GetStats()
	httpStats := dm.httpDiscovery.GetStats()
	
	return map[string]interface{}{
		"isRunning":       dm.isRunning,
		"totalDevices":    len(dm.combinedDevices),
		"onlineDevices":   len(dm.GetOnlineDevices()),
		"mdnsDiscovery":  mdnsStats,
		"httpDiscovery":  httpStats,
		"lastUpdate":      time.Now(),
	}
}

// GetDeviceByIP 根据IP地址获取设备
func (dm *DiscoveryManager) GetDeviceByIP(ip string) *models.Device {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	for _, device := range dm.combinedDevices {
		if device.IP == ip && device.Status == models.DeviceStatusOnline {
			return device
		}
	}
	
	return nil
}

// GetDeviceByID 根据设备ID获取设备
func (dm *DiscoveryManager) GetDeviceByID(id string) *models.Device {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if device, exists := dm.combinedDevices[id]; exists && device.Status == models.DeviceStatusOnline {
		return device
	}
	
	return nil
}