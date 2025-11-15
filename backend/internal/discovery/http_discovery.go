package discovery

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"airshare-backend/pkg/models"
)

// HTTPDiscovery 实现基于HTTP的设备发现服务
type HTTPDiscovery struct {
	client       *http.Client
	basePort     int
	scanTimeout  time.Duration
	onlineDevices map[string]*models.Device
	mu           sync.RWMutex
	isRunning    bool
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewHTTPDiscovery 创建新的HTTP设备发现服务
func NewHTTPDiscovery(basePort int, scanTimeout time.Duration) *HTTPDiscovery {
	ctx, cancel := context.WithCancel(context.Background())
	
	client := &http.Client{
		Timeout: scanTimeout / 2,
	}
	
	return &HTTPDiscovery{
		client:       client,
		basePort:     basePort,
		scanTimeout:  scanTimeout,
		onlineDevices: make(map[string]*models.Device),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start 启动HTTP设备发现服务
func (h *HTTPDiscovery) Start() error {
	if h.isRunning {
		return fmt.Errorf("HTTP discovery is already running")
	}

	h.isRunning = true
	log.Println("HTTP discovery service started")

	// 启动设备发现循环
	go h.discoveryLoop()

	return nil
}

// Stop 停止HTTP设备发现服务
func (h *HTTPDiscovery) Stop() error {
	if !h.isRunning {
		return nil
	}

	h.cancel()
	h.isRunning = false
	log.Println("HTTP discovery service stopped")
	
	return nil
}

// discoveryLoop 设备发现循环
func (h *HTTPDiscovery) discoveryLoop() {
	ticker := time.NewTicker(h.scanTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.scanNetwork()
		}
	}
}

// scanNetwork 扫描网络中的设备
func (h *HTTPDiscovery) scanNetwork() {
	// 获取本地IP地址
	localIP, err := h.getLocalIP()
	if err != nil {
		log.Printf("Failed to get local IP: %v", err)
		return
	}

	// 生成扫描IP范围
	ipRange := h.generateIPRange(localIP)
	
	// 并发扫描所有IP地址
	var wg sync.WaitGroup
	results := make(chan *models.Device, 256)
	
	for _, ip := range ipRange {
		wg.Add(1)
		go h.scanIP(ip, results, &wg)
	}
	
	wg.Wait()
	close(results)
	
	// 处理扫描结果
	h.processScanResults(results)
	
	// 清理离线设备
	h.cleanupOfflineDevices()
}

// getLocalIP 获取本地IP地址
func (h *HTTPDiscovery) getLocalIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// generateIPRange 生成IP扫描范围
func (h *HTTPDiscovery) generateIPRange(localIP net.IP) []string {
	var ips []string
	
	// 假设局域网为 /24 子网
	ip := localIP.To4()
	if ip == nil {
		return ips
	}
	
	// 生成局域网内所有可能的IP地址
	for i := 1; i <= 254; i++ {
		newIP := fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], i)
		ips = append(ips, newIP)
	}
	
	return ips
}

// scanIP 扫描单个IP地址
func (h *HTTPDiscovery) scanIP(ip string, results chan<- *models.Device, wg *sync.WaitGroup) {
	defer wg.Done()

	// 尝试连接到设备服务端口
	url := fmt.Sprintf("http://%s:%d/api/status", ip, h.basePort)
	
	resp, err := h.client.Get(url)
	if err != nil {
		return // 设备不可达
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return // 设备服务未运行
	}
	
	// 解析设备信息
	device := &models.Device{
		ID:        fmt.Sprintf("http-%s", ip),
		Name:      fmt.Sprintf("设备-%s", ip),
		IP:        ip,
		Type:      models.DeviceTypeUnknown,
		Platform:  "Unknown",
		Status:    models.DeviceStatusOnline,
		LastSeen:  time.Now(),
	}
	
	results <- device
}

// processScanResults 处理扫描结果
func (h *HTTPDiscovery) processScanResults(results <-chan *models.Device) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for device := range results {
		// 检查设备是否已存在
		existing, exists := h.onlineDevices[device.ID]
		
		if exists {
			// 更新设备信息
			existing.LastSeen = time.Now()
			h.onlineDevices[device.ID] = existing
		} else {
			// 添加新设备
			h.onlineDevices[device.ID] = device
			log.Printf("Discovered HTTP device: %s (%s)", device.Name, device.IP)
		}
	}
}

// cleanupOfflineDevices 清理离线设备
func (h *HTTPDiscovery) cleanupOfflineDevices() {
	h.mu.Lock()
	defer h.mu.Unlock()

	cutoffTime := time.Now().Add(-h.scanTimeout * 3)
	
	for id, device := range h.onlineDevices {
		if device.LastSeen.Before(cutoffTime) {
			// 设备已离线
			delete(h.onlineDevices, id)
			log.Printf("HTTP device went offline: %s (%s)", device.Name, device.IP)
		}
	}
}

// GetOnlineDevices 获取在线设备列表
func (h *HTTPDiscovery) GetOnlineDevices() []*models.Device {
	h.mu.RLock()
	defer h.mu.RUnlock()

	devices := make([]*models.Device, 0, len(h.onlineDevices))
	for _, device := range h.onlineDevices {
		devices = append(devices, device)
	}
	
	return devices
}

// IsRunning 检查服务是否正在运行
func (h *HTTPDiscovery) IsRunning() bool {
	return h.isRunning
}

// GetStats 获取设备发现统计信息
func (h *HTTPDiscovery) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"isRunning":    h.isRunning,
		"onlineDevices": len(h.onlineDevices),
		"scanTimeout":  h.scanTimeout.String(),
		"basePort":     h.basePort,
		"lastScan":     time.Now(),
	}
}