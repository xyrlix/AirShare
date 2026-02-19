package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"airshare-backend/internal/config"
	"airshare-backend/pkg/models"
)

// Service 文件传输服务
type Service struct {
	config        *config.TransferConfig
	transfers      map[string]*models.TransferRequest
	mutex          sync.RWMutex
	stopChan       chan struct{}
}

// NewService 创建新的文件传输服务
func NewService(cfg *config.TransferConfig) (*Service, error) {
	service := &Service{
		config:   cfg,
		transfers: make(map[string]*models.TransferRequest),
		stopChan:  make(chan struct{}),
	}

	// 确保存储目录存在
	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %v", err)
	}

	// 启动清理任务
	go service.startCleanupTask()

	return service, nil
}

// StartTransfer 开始传输
func (s *Service) StartTransfer(req *models.TransferRequest) (*models.TransferRequest, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 验证文件大小
	totalSize := int64(0)
	for _, file := range req.Files {
		totalSize += file.Size
		if file.Size > s.config.MaxFileSize {
			return nil, fmt.Errorf("文件 %s 超过最大大小限制", file.Name)
		}
	}

	// 设置传输信息
	req.ID = generateID()
	req.Status = models.TransferPending
	req.CreatedAt = time.Now()

	// 存储传输请求
	s.transfers[req.ID] = req

	log.Printf("开始传输: %s, 文件数: %d, 总大小: %d", req.ID, len(req.Files), totalSize)

	return req, nil
}

// GetTransferStatus 获取传输状态
func (s *Service) GetTransferStatus(transferID string) (*models.TransferRequest, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	transfer, exists := s.transfers[transferID]
	if !exists {
		return nil, fmt.Errorf("传输不存在: %s", transferID)
	}

	return transfer, nil
}

// UploadFile 上传文件
func (s *Service) UploadFile(transferID string, fileInfo *models.FileInfo, reader io.Reader) error {
	s.mutex.Lock()
	transfer, exists := s.transfers[transferID]
	s.mutex.Unlock()

	if !exists {
		return fmt.Errorf("传输不存在: %s", transferID)
	}

	// 创建文件路径
	filePath := filepath.Join(s.config.StoragePath, fileInfo.ID)

	// 创建文件
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	// 写入文件
	hash := sha256.New()
	writer := io.MultiWriter(file, hash)

	written, err := io.Copy(writer, reader)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	// 验证文件大小
	if written != fileInfo.Size {
		os.Remove(filePath)
		return fmt.Errorf("文件大小不匹配: 期望 %d, 实际 %d", fileInfo.Size, written)
	}

	// 验证校验和
	checksum := hex.EncodeToString(hash.Sum(nil))
	if checksum != fileInfo.Checksum {
		os.Remove(filePath)
		return fmt.Errorf("文件校验和不匹配")
	}

	// 更新传输进度
	s.mutex.Lock()
	for i, f := range transfer.Files {
		if f.ID == fileInfo.ID {
			transfer.Files[i].Progress = 100
			break
		}
	}

	// 检查是否所有文件都完成
	allCompleted := true
	for _, f := range transfer.Files {
		if f.Progress < 100 {
			allCompleted = false
			break
		}
	}

	if allCompleted {
		transfer.Status = models.TransferCompleted
		completed := time.Now()
		transfer.CompletedAt = &completed
	}

	s.mutex.Unlock()

	log.Printf("文件上传完成: %s, 大小: %d", fileInfo.Name, written)
	return nil
}

// DownloadFile 下载文件
func (s *Service) DownloadFile(transferID, fileID string) (io.ReadCloser, *models.FileInfo, error) {
	s.mutex.RLock()
	transfer, exists := s.transfers[transferID]
	s.mutex.RUnlock()

	if !exists {
		return nil, nil, fmt.Errorf("传输不存在: %s", transferID)
	}

	// 查找文件信息
	var fileInfo *models.FileInfo
	for _, f := range transfer.Files {
		if f.ID == fileID {
			fileInfo = &f
			break
		}
	}

	if fileInfo == nil {
		return nil, nil, fmt.Errorf("文件不存在: %s", fileID)
	}

	// 打开文件
	filePath := filepath.Join(s.config.StoragePath, fileID)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("打开文件失败: %v", err)
	}

	return file, fileInfo, nil
}

// CancelTransfer 取消传输
func (s *Service) CancelTransfer(transferID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	transfer, exists := s.transfers[transferID]
	if !exists {
		return fmt.Errorf("传输不存在: %s", transferID)
	}

	transfer.Status = models.TransferCancelled
	
	// 清理文件
	for _, file := range transfer.Files {
		filePath := filepath.Join(s.config.StoragePath, file.ID)
		os.Remove(filePath)
	}

	log.Printf("传输已取消: %s", transferID)
	return nil
}

// startCleanupTask 启动清理任务
func (s *Service) startCleanupTask() {
	ticker := time.NewTicker(time.Duration(s.config.CleanupPeriod) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanupOldTransfers()
		case <-s.stopChan:
			return
		}
	}
}

// cleanupOldTransfers 清理旧的传输记录
func (s *Service) cleanupOldTransfers() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	for id, transfer := range s.transfers {
		// 清理24小时前完成的传输
		if transfer.CompletedAt != nil && now.Sub(*transfer.CompletedAt) > 24*time.Hour {
			delete(s.transfers, id)
			
			// 清理文件
			for _, file := range transfer.Files {
				filePath := filepath.Join(s.config.StoragePath, file.ID)
				os.Remove(filePath)
			}
			
			log.Printf("清理传输记录: %s", id)
		}
	}
}

// Stop 停止服务
func (s *Service) Stop() {
	close(s.stopChan)
}

// generateID 生成唯一ID
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// 辅助函数：计算文件校验和
func calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}