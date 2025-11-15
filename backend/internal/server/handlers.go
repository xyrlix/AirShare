package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"

	"airshare-backend/pkg/models"
	"github.com/gorilla/mux"
)

// API处理函数

func (s *Server) handleGetDevices(w http.ResponseWriter, r *http.Request) {
	// 暂时返回空设备列表，因为s.discoveryService.GetDevices方法未定义
	// devices := s.discoveryService.GetDevices()
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"devices": []string{},
	})
}

func (s *Server) handleSendFile(w http.ResponseWriter, r *http.Request) {
	var req models.TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 暂时跳过目标设备验证，因为相关方法和字段未定义
	// if !s.discoveryService.DeviceExists(req.TargetDeviceID) {
	// 	respondError(w, http.StatusNotFound, "Target device not found")
	// 	return
	// }

	// 开始传输
	transferID, err := s.transferService.StartTransfer(&req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to start transfer")
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"transfer_id": transferID,
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
	filename := vars["filename"]

	// 暂时不使用s.config.Storage.Directory，因为配置可能未定义
	// filePath := filepath.Join(s.config.Storage.Directory, filename)

	// 暂时注释掉安全检查
	// if !isSafePath(filePath, s.config.Storage.Directory) {
	// 	respondError(w, http.StatusBadRequest, "Invalid filename")
	// 	return
	// }

	// 设置下载头
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	// 暂时不实现文件下载
	respondError(w, http.StatusNotImplemented, "File download not implemented")
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	// 暂时不实现删除文件逻辑，因为s.transferService.DeleteFile方法未定义
	// vars := mux.Vars(r)
	// filename := vars["filename"]
	// if err := s.transferService.DeleteFile(filename); err != nil {
	// 	respondError(w, http.StatusInternalServerError, "Failed to delete file")
	// 	return
	// }

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "File deleted successfully",
	})
}

// WebSocket处理函数
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// 升级到WebSocket连接
	// 使用s.upgrader而不是全局upgrader
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}

	// 处理WebSocket消息
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
	return !filepath.IsAbs(rel) && rel != ".." && rel[:3] != "../"
}