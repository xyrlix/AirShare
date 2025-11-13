package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EncryptionService 提供端到端加密功能
type EncryptionService struct {
	mu            sync.RWMutex
	privateKey    *rsa.PrivateKey
	publicKey     *rsa.PublicKey
	certificates  map[string]*x509.Certificate
	sharedSecrets map[string][]byte // 与对等端的共享密钥
	keyDir        string
}

// EncryptedMessage 加密消息结构
type EncryptedMessage struct {
	Type        string `json:"type"`         // 消息类型
	Data        string `json:"data"`         // 加密数据（base64编码）
	IV          string `json:"iv"`           // 初始化向量（base64编码）
	Key         string `json:"key"`          // 加密的AES密钥（base64编码）
	Signature   string `json:"signature"`    // 数字签名（base64编码）
	Timestamp   int64  `json:"timestamp"`    // 时间戳
	SenderID    string `json:"sender_id"`    // 发送者ID
	RecipientID string `json:"recipient_id"` // 接收者ID
}

// KeyPair 密钥对
type KeyPair struct {
	PrivateKeyPEM string `json:"private_key"` // PEM格式的私钥
	PublicKeyPEM  string `json:"public_key"`  // PEM格式的公钥
	Fingerprint   string `json:"fingerprint"` // 密钥指纹
	CreatedAt     int64  `json:"created_at"`  // 创建时间
}

// EncryptData 加密数据请求
type EncryptData struct {
	Plaintext    []byte `json:"plaintext"`    // 明文数据
	RecipientID  string `json:"recipient_id"` // 接收者ID
}

// NewEncryptionService 创建新的加密服务
func NewEncryptionService(keyDir string) (*EncryptionService, error) {
	service := &EncryptionService{
		certificates:  make(map[string]*x509.Certificate),
		sharedSecrets: make(map[string][]byte),
		keyDir:        keyDir,
	}

	// 确保密钥目录存在
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, fmt.Errorf("创建密钥目录失败: %v", err)
	}

	// 加载或生成密钥对
	if err := service.loadOrGenerateKeys(); err != nil {
		return nil, err
	}

	return service, nil
}

// Encrypt 加密数据
func (s *EncryptionService) Encrypt(data *EncryptData) (*EncryptedMessage, error) {
	// 生成随机AES密钥
	aesKey := make([]byte, 32) // AES-256
	if _, err := rand.Read(aesKey); err != nil {
		return nil, fmt.Errorf("生成AES密钥失败: %v", err)
	}

	// 生成初始化向量
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("生成IV失败: %v", err)
	}

	// 创建AES加密器
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("创建AES加密器失败: %v", err)
	}

	// 使用CTR模式加密
	stream := cipher.NewCTR(block, iv)
	ciphertext := make([]byte, len(data.Plaintext))
	stream.XORKeyStream(ciphertext, data.Plaintext)

	// 使用接收者的公钥加密AES密钥
	s.mu.RLock()
	cert, exists := s.certificates[data.RecipientID]
	s.mu.RUnlock()

	if !exists {
		// 如果没有证书，使用自己的公钥（用于测试）
		cert = &x509.Certificate{
			PublicKey: s.publicKey,
		}
	}

	publicKey := cert.PublicKey.(*rsa.PublicKey)
	encryptedKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publicKey, aesKey, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA加密失败: %v", err)
	}

	// 使用私钥签名
	signature, err := s.sign(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("签名失败: %v", err)
	}

	// 构建加密消息
	msg := &EncryptedMessage{
		Type:        "encrypted_data",
		Data:        base64.StdEncoding.EncodeToString(ciphertext),
		IV:          base64.StdEncoding.EncodeToString(iv),
		Key:         base64.StdEncoding.EncodeToString(encryptedKey),
		Signature:   base64.StdEncoding.EncodeToString(signature),
		Timestamp:   time.Now().Unix(),
		SenderID:    s.getFingerprint(),
		RecipientID: data.RecipientID,
	}

	return msg, nil
}

// Decrypt 解密数据
func (s *EncryptionService) Decrypt(msg *EncryptedMessage) ([]byte, error) {
	// 解码密文
	ciphertext, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		return nil, fmt.Errorf("解码密文失败: %v", err)
	}

	// 解码签名
	signature, err := base64.StdEncoding.DecodeString(msg.Signature)
	if err != nil {
		return nil, fmt.Errorf("解码签名失败: %v", err)
	}

	// 解码加密密钥
	encryptedKey, err := base64.StdEncoding.DecodeString(msg.Key)
	if err != nil {
		return nil, fmt.Errorf("解码加密密钥失败: %v", err)
	}

	// 解码IV
	iv, err := base64.StdEncoding.DecodeString(msg.IV)
	if err != nil {
		return nil, fmt.Errorf("解码IV失败: %v", err)
	}

	// 验证签名
	if err := s.verifySignature(ciphertext, signature, msg.SenderID); err != nil {
		return nil, fmt.Errorf("签名验证失败: %v", err)
	}

	// 解密AES密钥
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, s.privateKey, encryptedKey, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA解密失败: %v", err)
	}

	// 检查是否有共享密钥
	s.mu.RLock()
	if sharedSecret, exists := s.sharedSecrets[msg.SenderID]; exists {
		// 使用共享密钥解密
		block, err := aes.NewCipher(sharedSecret)
		if err != nil {
			return nil, fmt.Errorf("创建共享密钥加密器失败: %v", err)
		}

		stream := cipher.NewCTR(block, iv)
		plaintext := make([]byte, len(ciphertext))
		stream.XORKeyStream(plaintext, ciphertext)
		s.mu.RUnlock()

		return plaintext, nil
	}
	s.mu.RUnlock()

	// 使用AES密钥解密
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("创建AES解密器失败: %v", err)
	}

	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return plaintext, nil
}

// GenerateKeyPair 生成新的RSA密钥对
func (s *EncryptionService) GenerateKeyPair() (*KeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("生成RSA密钥对失败: %v", err)
	}

	// 生成公钥PEM
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("序列化公钥失败: %v", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	// 生成私钥PEM
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// 计算密钥指纹
	fingerprint := s.calculateFingerprint(publicKeyDER)

	keyPair := &KeyPair{
		PrivateKeyPEM: string(privateKeyPEM),
		PublicKeyPEM:  string(publicKeyPEM),
		Fingerprint:   fingerprint,
		CreatedAt:     time.Now().Unix(),
	}

	// 保存密钥对
	if err := s.saveKeyPair(keyPair); err != nil {
		return nil, err
	}

	// 更新服务密钥
	s.mu.Lock()
	s.privateKey = privateKey
	s.publicKey = &privateKey.PublicKey
	s.mu.Unlock()

	return keyPair, nil
}

// GetPublicKey 获取公钥PEM字符串
func (s *EncryptionService) GetPublicKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	publicKeyDER, err := x509.MarshalPKIXPublicKey(s.publicKey)
	if err != nil {
		return ""
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	return string(publicKeyPEM)
}

// GetFingerprint 获取密钥指纹
func (s *EncryptionService) GetFingerprint() string {
	return s.getFingerprint()
}

// 加载或生成密钥对
func (s *EncryptionService) loadOrGenerateKeys() error {
	keyFile := filepath.Join(s.keyDir, "keypair.json")

	// 检查密钥文件是否存在
	if _, err := os.Stat(keyFile); err == nil {
		// 加载现有密钥
		data, err := os.ReadFile(keyFile)
		if err != nil {
			return fmt.Errorf("读取密钥文件失败: %v", err)
		}

		var keyPair KeyPair
		if err := json.Unmarshal(data, &keyPair); err != nil {
			return fmt.Errorf("解析密钥文件失败: %v", err)
		}

		// 解析私钥
		block, _ := pem.Decode([]byte(keyPair.PrivateKeyPEM))
		if block == nil {
			return errors.New("无效的私钥PEM格式")
		}

		privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("解析私钥失败: %v", err)
		}

		s.mu.Lock()
		s.privateKey = privateKey
		s.publicKey = &privateKey.PublicKey
		s.mu.Unlock()

		log.Printf("已加载现有密钥对，指纹: %s", keyPair.Fingerprint)
		return nil
	}

	// 生成新密钥对
	log.Println("未找到现有密钥对，正在生成新密钥对...")
	keyPair, err := s.GenerateKeyPair()
	if err != nil {
		return err
	}

	log.Printf("已生成新密钥对，指纹: %s", keyPair.Fingerprint)
	return nil
}

// 保存密钥对
func (s *EncryptionService) saveKeyPair(keyPair *KeyPair) error {
	keyFile := filepath.Join(s.keyDir, "keypair.json")

	data, err := json.MarshalIndent(keyPair, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化密钥对失败: %v", err)
	}

	if err := os.WriteFile(keyFile, data, 0600); err != nil {
		return fmt.Errorf("写入密钥文件失败: %v", err)
	}

	return nil
}

// 计算密钥指纹
func (s *EncryptionService) calculateFingerprint(publicKeyDER []byte) string {
	hash := sha256.Sum256(publicKeyDER)
	return fmt.Sprintf("%x", hash[:8]) // 使用前8字节作为指纹
}

// 获取指纹
func (s *EncryptionService) getFingerprint() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	publicKeyDER, err := x509.MarshalPKIXPublicKey(s.publicKey)
	if err != nil {
		return ""
	}

	return s.calculateFingerprint(publicKeyDER)
}

// 使用私钥签名
func (s *EncryptionService) sign(data []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hash := sha256.Sum256(data)
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, err
	}

	return signature, nil
}

// 验证签名
func (s *EncryptionService) verifySignature(data, signature []byte, senderID string) error {
	hash := sha256.Sum256(data)

	s.mu.RLock()
	cert, exists := s.certificates[senderID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("发送者证书不存在: %s", senderID)
	}

	publicKey := cert.PublicKey.(*rsa.PublicKey)
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], signature)
}

// AddCertificate 添加证书
func (s *EncryptionService) AddCertificate(cert *x509.Certificate, fingerprint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.certificates[fingerprint] = cert
}

// GenerateSharedSecret 生成共享密钥
func (s *EncryptionService) GenerateSharedSecret(peerPublicKeyPEM string) (string, error) {
	block, _ := pem.Decode([]byte(peerPublicKeyPEM))
	if block == nil {
		return "", errors.New("无效的公钥PEM格式")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("解析公钥失败: %v", err)
	}

	peerPublicKey := publicKey.(*rsa.PublicKey)

	// 使用ECDH或其他密钥交换协议建立共享密钥
	// 这里简化实现，实际应该使用更安全的密钥交换协议
	sharedSecret := make([]byte, 32)
	if _, err := rand.Read(sharedSecret); err != nil {
		return "", fmt.Errorf("生成共享密钥失败: %v", err)
	}

	fingerprint := s.calculateFingerprint(block.Bytes)

	s.mu.Lock()
	s.sharedSecrets[fingerprint] = sharedSecret
	s.mu.Unlock()

	return fingerprint, nil
}