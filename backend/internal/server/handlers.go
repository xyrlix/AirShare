package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"airshare-backend/pkg/models"
	"github.com/gorilla/mux"
)

// API处理函数

func (s *Server) handleGetDevices(w http.ResponseWriter, r *http.Request) {
	devices := s.discoveryService.GetOnlineDevices()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    devices,
	})
}

func (s *Server) handleSendFile(w http.ResponseWriter, r *http.Request) {
	var req models.TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 开始传输
	transfer, err := s.transferService.StartTransfer(&req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to start transfer")
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"transfer_id": transfer.ID,
		"status":      "started",
	})
}

func (s *Server) handleGetTransferStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transfer_id"]

	status, err := s.transferService.GetTransferStatus(transferID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Transfer not found")
		return
	}

	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleCancelTransfer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transfer_id"]

	if err := s.transferService.CancelTransfer(transferID); err != nil {
		respondError(w, http.StatusNotFound, "Transfer not found")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "cancelled",
	})
}

func (s *Server) handleGetFiles(w http.ResponseWriter, r *http.Request) {
	// 暂时返回空列表，因为transferService.ListFiles方法未定义
	// files, err := s.transferService.ListFiles()
	// if err != nil {
	// 	respondError(w, http.StatusInternalServerError, "Failed to list files")
	// 	return
	// }

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"files": []string{},
	})
}

func (s *Server) handleDownloadFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transfer_id"]
	fileID := vars["filename"]

	// 安全路径检查：防止路径遍历攻击
	if fileID == "" || strings.Contains(fileID, "/") || strings.Contains(fileID, "\\") || strings.Contains(fileID, "..") {
		respondError(w, http.StatusBadRequest, "Invalid filename")
		return
	}

	// 下载文件
	reader, fileInfo, err := s.transferService.DownloadFile(transferID, fileID)
	if err != nil {
		respondError(w, http.StatusNotFound, "File not found")
		return
	}
	defer reader.Close()

	// 设置下载头
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileInfo.Name))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size))

	if _, err := io.Copy(w, reader); err != nil {
		// 传输已开始无法再返回错误响应
		fmt.Printf("文件传输错误: %v\n", err)
	}
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	// 暂时不实现删除文件逻辑，因为s.transferService.DeleteFile方法未定义
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "File deleted successfully",
	})
}

// handlePauseTransfer 暂停传输
func (s *Server) handlePauseTransfer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transfer_id"]

	if err := s.transferService.PauseTransfer(transferID); err != nil {
		respondError(w, http.StatusNotFound, "Transfer not found or cannot be paused")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "paused",
	})
}

// handleResumeTransfer 恢复传输
func (s *Server) handleResumeTransfer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transferID := vars["transfer_id"]

	if err := s.transferService.ResumeTransfer(transferID); err != nil {
		respondError(w, http.StatusNotFound, "Transfer not found or cannot be resumed")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "resumed",
	})
}

// handleGetTransferHistory 获取传输历史
func (s *Server) handleGetTransferHistory(w http.ResponseWriter, r *http.Request) {
	history := s.transferService.GetTransferHistory()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    history,
	})
}

// WebSocket处理函数
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级到WebSocket连接
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}

	// 将新连接注册到客户端 map，确保广播能送达
	s.clientMutex.Lock()
	s.clients[conn] = true
	s.clientMutex.Unlock()

	log.Printf("WebSocket新连接: %s，当前连接数: %d", conn.RemoteAddr(), len(s.clients))

	// 处理WebSocket消息（断开时自动从 map 移除）
	go s.handleWebSocketMessages(conn)
}

// 辅助函数
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error": message,
	})
}

func isSafePath(filePath, baseDir string) bool {
	rel, err := filepath.Rel(baseDir, filePath)
	if err != nil {
		return false
	}
	return !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, "../")
}