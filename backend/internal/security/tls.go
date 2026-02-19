package security

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// TLSService TLS/HTTPS安全传输服务
type TLSService struct {
	certService *CertificateService
	tlsConfigs  map[string]*tls.Config
	mu          sync.RWMutex
}

// NewTLSService 创建新的TLS服务
func NewTLSService(certService *CertificateService) (*TLSService, error) {
	service := &TLSService{
		certService: certService,
		tlsConfigs:  make(map[string]*tls.Config),
	}

	return service, nil
}

// GetTLSConfig 获取设备TLS配置
func (s *TLSService) GetTLSConfig(deviceID string) (*tls.Config, error) {
	s.mu.RLock()
	config, exists := s.tlsConfigs[deviceID]
	s.mu.RUnlock()

	if exists {
		return config, nil
	}

	// 创建新的TLS配置
	cert, exists := s.certService.GetCertificate(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备证书不存在: %s", deviceID)
	}

	privateKey, exists := s.certService.GetPrivateKey(deviceID)
	if !exists {
		return nil, fmt.Errorf("设备私钥不存在: %s", deviceID)
	}

	// 创建TLS证书
	tlsCert := tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  privateKey,
		Leaf:        cert,
	}

	// 创建TLS配置
	config = &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
		PreferServerCipherSuites: true,
		SessionTicketsDisabled:   false,
		ClientSessionCache:       tls.NewLRUClientSessionCache(1000),
		NextProtos:               []string{"h2", "http/1.1"},
	}

	s.mu.Lock()
	s.tlsConfigs[deviceID] = config
	s.mu.Unlock()

	return config, nil
}

// GetClientTLSConfig 获取客户端TLS配置
func (s *TLSService) GetClientTLSConfig() *tls.Config {
	// 获取CA证书
	caCertPEM := s.certService.GetCACertificate()
	
	// 创建证书池
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(caCertPEM))

	return &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: false, // 严格验证证书
		MinVersion:         tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	}
}

// CreateHTTPSClient 创建HTTPS客户端
func (s *TLSService) CreateHTTPSClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: s.GetClientTLSConfig(),
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

// CreateHTTPSServer 创建HTTPS服务器
func (s *TLSService) CreateHTTPSServer(deviceID string, handler http.Handler) (*http.Server, error) {
	tlsConfig, err := s.GetTLSConfig(deviceID)
	if err != nil {
		return nil, err
	}

	server := &http.Server{
		TLSConfig:    tlsConfig,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server, nil
}

// VerifyPeerCertificate 验证对等端证书
func (s *TLSService) VerifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	if len(rawCerts) == 0 {
		return fmt.Errorf("未提供对等端证书")
	}

	// 解析对等端证书
	peerCert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return fmt.Errorf("解析对等端证书失败: %v", err)
	}

	// 验证证书签名
	caCertPEM := s.certService.GetCACertificate()
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(caCertPEM))

	opts := x509.VerifyOptions{
		Roots: caCertPool,
	}

	_, err = peerCert.Verify(opts)
	if err != nil {
		return fmt.Errorf("对等端证书验证失败: %v", err)
	}

	// 验证证书有效期
	if time.Now().Before(peerCert.NotBefore) || time.Now().After(peerCert.NotAfter) {
		return fmt.Errorf("对等端证书已过期或尚未生效")
	}

	return nil
}

// GenerateSecureConnection 创建安全连接
func (s *TLSService) GenerateSecureConnection(deviceID string) (*tls.Conn, error) {
	tlsConfig, err := s.GetTLSConfig(deviceID)
	if err != nil {
		return nil, err
	}

	// 配置双向认证
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.VerifyPeerCertificate = s.VerifyPeerCertificate

	return tls.Dial("tcp", "", tlsConfig)
}

// CreateSecureListener 创建安全监听器
func (s *TLSService) CreateSecureListener(deviceID, address string) (net.Listener, error) {
	tlsConfig, err := s.GetTLSConfig(deviceID)
	if err != nil {
		return nil, err
	}

	// 配置双向认证
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	tlsConfig.VerifyPeerCertificate = s.VerifyPeerCertificate

	return tls.Listen("tcp", address, tlsConfig)
}

// SecureDataTransfer 安全数据传输接口
type SecureDataTransfer struct {
	encryptionService *EncryptionService
	tlsService       *TLSService
}

// NewSecureDataTransfer 创建新的安全数据传输服务
func NewSecureDataTransfer(encryptionService *EncryptionService, tlsService *TLSService) *SecureDataTransfer {
	return &SecureDataTransfer{
		encryptionService: encryptionService,
		tlsService:       tlsService,
	}
}

// EncryptAndSend 加密并发送数据
func (s *SecureDataTransfer) EncryptAndSend(data []byte, recipientID string, conn io.Writer) error {
	// 使用端到端加密
	encryptedMsg, err := s.encryptionService.Encrypt(&EncryptData{
		Plaintext:   data,
		RecipientID: recipientID,
	})
	if err != nil {
		return fmt.Errorf("加密数据失败: %v", err)
	}

	// 序列化加密消息
	msgData, err := json.Marshal(encryptedMsg)
	if err != nil {
		return fmt.Errorf("序列化加密消息失败: %v", err)
	}

	// 通过TLS连接发送
	_, err = conn.Write(msgData)
	if err != nil {
		return fmt.Errorf("发送数据失败: %v", err)
	}

	return nil
}

// ReceiveAndDecrypt 接收并解密数据
func (s *SecureDataTransfer) ReceiveAndDecrypt(conn io.Reader) ([]byte, string, error) {
	// 从连接读取数据
	buffer := make([]byte, 4096)
	n, err := conn.Read(buffer)
	if err != nil {
		return nil, "", fmt.Errorf("读取数据失败: %v", err)
	}

	// 解析加密消息
	var encryptedMsg EncryptedMessage
	if err := json.Unmarshal(buffer[:n], &encryptedMsg); err != nil {
		return nil, "", fmt.Errorf("解析加密消息失败: %v", err)
	}

	// 解密数据
	plaintext, err := s.encryptionService.Decrypt(&encryptedMsg)
	if err != nil {
		return nil, "", fmt.Errorf("解密数据失败: %v", err)
	}

	return plaintext, encryptedMsg.SenderID, nil
}