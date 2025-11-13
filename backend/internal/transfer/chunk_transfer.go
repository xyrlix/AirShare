package transfer

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ChunkTransferService 实现文件分片传输和断点续传功能
type ChunkTransferService struct {
	mu            sync.RWMutex
	chunkSize     int64
	maxRetries    int
	retryInterval time.Duration
	storageDir    string
}

// ChunkTransfer 表示分片传输任务
type ChunkTransfer struct {
	ID          string
	FileName    string
	FileSize    int64
	ChunkSize   int64
	TotalChunks int
	Chunks      []*Chunk
	Status      TransferStatus
	PeerID      string
	StartTime   time.Time
	EndTime     time.Time
	Error       string
	FileHash    string
}

// Chunk 表示文件分片
type Chunk struct {
	Index     int
	Offset    int64
	Size      int64
	Data      []byte
	Checksum  string
	Status    ChunkStatus
	Retries   int
	LastError string
}

// ChunkStatus 分片状态
type ChunkStatus string

const (
	ChunkPending   ChunkStatus = "pending"
	ChunkSending   ChunkStatus = "sending"
	ChunkSent      ChunkStatus = "sent"
	ChunkReceived  ChunkStatus = "received"
	ChunkFailed    ChunkStatus = "failed"
	ChunkVerified  ChunkStatus = "verified"
)

// NewChunkTransferService 创建新的分片传输服务
func NewChunkTransferService(chunkSize int64, maxRetries int, storageDir string) *ChunkTransferService {
	return &ChunkTransferService{
		chunkSize:     chunkSize,
		maxRetries:    maxRetries,
		retryInterval: 5 * time.Second,
		storageDir:    storageDir,
	}
}

// PrepareFileForSending 准备文件用于发送（分片处理）
func (s *ChunkTransferService) PrepareFileForSending(filePath string) (*ChunkTransfer, error) {
	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %v", err)
	}

	fileSize := fileInfo.Size()
	
	// 计算文件哈希
	fileHash, err := s.calculateFileHash(filePath)
	if err != nil {
		return nil, fmt.Errorf("计算文件哈希失败: %v", err)
	}

	// 计算分片数量
	totalChunks := (fileSize + s.chunkSize - 1) / s.chunkSize

	// 创建传输任务
	transfer := &ChunkTransfer{
		ID:          generateTransferID(),
		FileName:    fileInfo.Name(),
		FileSize:    fileSize,
		ChunkSize:   s.chunkSize,
		TotalChunks: int(totalChunks),
		Status:      TransferPending,
		StartTime:   time.Now(),
		FileHash:    fileHash,
		Chunks:      make([]*Chunk, totalChunks),
	}

	// 初始化分片信息
	for i := 0; i < int(totalChunks); i++ {
		offset := int64(i) * s.chunkSize
		chunkSize := s.chunkSize
		if i == int(totalChunks)-1 {
			chunkSize = fileSize - offset
		}

		transfer.Chunks[i] = &Chunk{
			Index:    i,
			Offset:   offset,
			Size:     chunkSize,
			Status:   ChunkPending,
			Retries:  0,
		}
	}

	return transfer, nil
}

// ReadChunkData 读取分片数据
func (s *ChunkTransferService) ReadChunkData(filePath string, chunk *Chunk) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 定位到分片位置
	_, err = file.Seek(chunk.Offset, io.SeekStart)
	if err != nil {
		return nil, fmt.Errorf("文件定位失败: %v", err)
	}

	// 读取分片数据
	data := make([]byte, chunk.Size)
	n, err := file.Read(data)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("读取分片数据失败: %v", err)
	}

	if int64(n) != chunk.Size {
		return nil, fmt.Errorf("读取数据大小不匹配: 期望 %d, 实际 %d", chunk.Size, n)
	}

	// 计算分片校验和
	chunk.Checksum = s.calculateChunkChecksum(data)

	return data, nil
}

// WriteChunkData 写入分片数据
func (s *ChunkTransferService) WriteChunkData(transferID string, chunk *Chunk, data []byte) error {
	// 验证分片数据
	if !s.verifyChunkData(chunk, data) {
		return fmt.Errorf("分片数据校验失败")
	}

	// 确保存储目录存在
	tempDir := filepath.Join(s.storageDir, "temp", transferID)
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %v", err)
	}

	// 写入分片文件
	chunkPath := filepath.Join(tempDir, fmt.Sprintf("chunk_%d.tmp", chunk.Index))
	err = os.WriteFile(chunkPath, data, 0644)
	if err != nil {
		return fmt.Errorf("写入分片文件失败: %v", err)
	}

	chunk.Status = ChunkReceived
	return nil
}

// ReassembleFile 重新组装文件
func (s *ChunkTransferService) ReassembleFile(transfer *ChunkTransfer) (string, error) {
	// 检查所有分片是否都已接收
	for _, chunk := range transfer.Chunks {
		if chunk.Status != ChunkReceived && chunk.Status != ChunkVerified {
			return "", fmt.Errorf("分片 %d 尚未接收", chunk.Index)
		}
	}

	// 创建目标文件
	targetPath := filepath.Join(s.storageDir, transfer.FileName)
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return "", fmt.Errorf("创建目标文件失败: %v", err)
	}
	defer targetFile.Close()

	// 按顺序合并分片
	tempDir := filepath.Join(s.storageDir, "temp", transfer.ID)
	for i := 0; i < transfer.TotalChunks; i++ {
		chunkPath := filepath.Join(tempDir, fmt.Sprintf("chunk_%d.tmp", i))
		
		chunkData, err := os.ReadFile(chunkPath)
		if err != nil {
			return "", fmt.Errorf("读取分片 %d 失败: %v", i, err)
		}

		// 写入目标文件
		_, err = targetFile.Write(chunkData)
		if err != nil {
			return "", fmt.Errorf("写入分片 %d 失败: %v", i, err)
		}

		// 标记分片为已验证
		transfer.Chunks[i].Status = ChunkVerified
	}

	// 验证文件完整性
	finalHash, err := s.calculateFileHash(targetPath)
	if err != nil {
		return "", fmt.Errorf("验证文件完整性失败: %v", err)
	}

	if finalHash != transfer.FileHash {
		return "", fmt.Errorf("文件完整性验证失败")
	}

	transfer.Status = TransferCompleted
	transfer.EndTime = time.Now()

	// 清理临时文件
	s.cleanupTempFiles(transfer.ID)

	return targetPath, nil
}

// ResumeTransfer 恢复传输任务
func (s *ChunkTransferService) ResumeTransfer(transfer *ChunkTransfer) []*Chunk {
	var pendingChunks []*Chunk

	for _, chunk := range transfer.Chunks {
		if chunk.Status == ChunkPending || chunk.Status == ChunkFailed {
			if chunk.Retries < s.maxRetries {
				chunk.Status = ChunkPending
				chunk.LastError = ""
				pendingChunks = append(pendingChunks, chunk)
			}
		}
	}

	transfer.Status = TransferInProgress
	return pendingChunks
}

// MarkChunkFailed 标记分片传输失败
func (s *ChunkTransferService) MarkChunkFailed(chunk *Chunk, err error) {
	chunk.Status = ChunkFailed
	chunk.Retries++
	chunk.LastError = err.Error()

	if chunk.Retries >= s.maxRetries {
		log.Printf("分片 %d 重试次数已达上限，标记为失败", chunk.Index)
	}
}

// GetTransferProgress 获取传输进度
func (s *ChunkTransferService) GetTransferProgress(transfer *ChunkTransfer) (int, int64) {
	completedChunks := 0
	transferredBytes := int64(0)

	for _, chunk := range transfer.Chunks {
		if chunk.Status == ChunkReceived || chunk.Status == ChunkVerified || chunk.Status == ChunkSent {
			completedChunks++
			transferredBytes += chunk.Size
		}
	}

	return completedChunks, transferredBytes
}

// 计算文件哈希
func (s *ChunkTransferService) calculateFileHash(filePath string) (string, error) {
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

// 计算分片校验和
func (s *ChunkTransferService) calculateChunkChecksum(data []byte) string {
	hash := sha256.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// 验证分片数据
func (s *ChunkTransferService) verifyChunkData(chunk *Chunk, data []byte) bool {
	expectedChecksum := s.calculateChunkChecksum(data)
	return expectedChecksum == chunk.Checksum
}

// 清理临时文件
func (s *ChunkTransferService) cleanupTempFiles(transferID string) {
	tempDir := filepath.Join(s.storageDir, "temp", transferID)
	err := os.RemoveAll(tempDir)
	if err != nil {
		log.Printf("清理临时文件失败: %v", err)
	}
}

// 保存传输状态（用于断点续传）
func (s *ChunkTransferService) SaveTransferState(transfer *ChunkTransfer) error {
	// 实现传输状态持久化
	// 可以将传输状态保存到文件或数据库中
	return nil
}

// 加载传输状态
func (s *ChunkTransferService) LoadTransferState(transferID string) (*ChunkTransfer, error) {
	// 实现传输状态加载
	// 可以从文件或数据库中加载传输状态
	return nil, nil
}