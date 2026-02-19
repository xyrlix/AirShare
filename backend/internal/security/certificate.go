package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// CertificateService 证书管理服务
type CertificateService struct {
	caCert     *x509.Certificate
	caKey      *rsa.PrivateKey
	certificates map[string]*x509.Certificate
	privateKeys  map[string]*rsa.PrivateKey
}

// NewCertificateService 创建新的证书服务
func NewCertificateService() (*CertificateService, error) {
	service := &CertificateService{
		certificates: make(map[string]*x509.Certificate),
		privateKeys:  make(map[string]*rsa.PrivateKey),
	}

	// 生成CA证书
	if err := service.generateCACertificate(); err != nil {
		return nil, fmt.Errorf("生成CA证书失败: %v", err)
	}

	return service, nil
}

// GenerateDeviceCertificate 生成设备证书
func (s *CertificateService) GenerateDeviceCertificate(deviceID string) (string, string, error) {
	// 生成设备密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("生成设备密钥对失败: %v", err)
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName:         deviceID,
			Organization:       []string{"AirShare"},
			OrganizationalUnit: []string{"Device"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0), // 1年有效期
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	// 使用CA证书签名
	certDER, err := x509.CreateCertificate(rand.Reader, &template, s.caCert, &privateKey.PublicKey, s.caKey)
	if err != nil {
		return "", "", fmt.Errorf("创建设备证书失败: %v", err)
	}

	// 转换为PEM格式
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	// 保存证书和密钥
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return "", "", fmt.Errorf("解析证书失败: %v", err)
	}

	s.certificates[deviceID] = cert
	s.privateKeys[deviceID] = privateKey

	return string(certPEM), string(privateKeyPEM), nil
}

// VerifyCertificate 验证证书
func (s *CertificateService) VerifyCertificate(certPEM string) (bool, string, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return false, "", fmt.Errorf("无效的证书PEM格式")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, "", fmt.Errorf("解析证书失败: %v", err)
	}

	// 验证证书签名
	err = cert.CheckSignatureFrom(s.caCert)
	if err != nil {
		return false, "", fmt.Errorf("证书签名验证失败: %v", err)
	}

	// 验证证书有效期
	if time.Now().Before(cert.NotBefore) || time.Now().After(cert.NotAfter) {
		return false, "", fmt.Errorf("证书已过期或尚未生效")
	}

	return true, cert.Subject.CommonName, nil
}

// GetCACertificate 获取CA证书
func (s *CertificateService) GetCACertificate() string {
	caCertPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: s.caCert.Raw,
	})

	return string(caCertPEM)
}

// 生成CA证书
func (s *CertificateService) generateCACertificate() error {
	// 生成CA密钥对
	caKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return fmt.Errorf("生成CA密钥对失败: %v", err)
	}

	// 创建CA证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:         "AirShare CA",
			Organization:       []string{"AirShare"},
			OrganizationalUnit: []string{"Certificate Authority"},
			Country:            []string{"CN"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10年有效期
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// 自签名CA证书
	caCertDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("创建CA证书失败: %v", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return fmt.Errorf("解析CA证书失败: %v", err)
	}

	s.caCert = caCert
	s.caKey = caKey

	return nil
}

// GetCertificate 获取设备证书
func (s *CertificateService) GetCertificate(deviceID string) (*x509.Certificate, bool) {
	cert, exists := s.certificates[deviceID]
	return cert, exists
}

// GetPrivateKey 获取设备私钥
func (s *CertificateService) GetPrivateKey(deviceID string) (*rsa.PrivateKey, bool) {
	key, exists := s.privateKeys[deviceID]
	return key, exists
}

// GenerateTemporaryCertificate 生成临时证书（用于测试）
func (s *CertificateService) GenerateTemporaryCertificate(deviceID string) (string, string, error) {
	// 生成临时密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("生成临时密钥对失败: %v", err)
	}

	// 创建临时证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: deviceID,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(24 * time.Hour), // 24小时有效期
		KeyUsage:   x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}

	// 自签名临时证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", fmt.Errorf("创建临时证书失败: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	})

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	return string(certPEM), string(privateKeyPEM), nil
}

// RevokeCertificate 吊销证书
func (s *CertificateService) RevokeCertificate(deviceID string) error {
	delete(s.certificates, deviceID)
	delete(s.privateKeys, deviceID)
	return nil
}

// ExportCertificate 导出证书和私钥
func (s *CertificateService) ExportCertificate(deviceID string) (string, string, error) {
	cert, exists := s.certificates[deviceID]
	if !exists {
		return "", "", fmt.Errorf("证书不存在: %s", deviceID)
	}

	key, exists := s.privateKeys[deviceID]
	if !exists {
		return "", "", fmt.Errorf("私钥不存在: %s", deviceID)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	})

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return string(certPEM), string(privateKeyPEM), nil
}

// ImportCertificate 导入证书和私钥
func (s *CertificateService) ImportCertificate(deviceID, certPEM, privateKeyPEM string) error {
	// 解析证书
	certBlock, _ := pem.Decode([]byte(certPEM))
	if certBlock == nil {
		return fmt.Errorf("无效的证书PEM格式")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析证书失败: %v", err)
	}

	// 解析私钥
	keyBlock, _ := pem.Decode([]byte(privateKeyPEM))
	if keyBlock == nil {
		return fmt.Errorf("无效的私钥PEM格式")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析私钥失败: %v", err)
	}

	// 验证证书和私钥匹配
	if !cert.PublicKey.(*rsa.PublicKey).Equal(&privateKey.PublicKey) {
		return fmt.Errorf("证书和私钥不匹配")
	}

	// 保存证书和私钥
	s.certificates[deviceID] = cert
	s.privateKeys[deviceID] = privateKey

	return nil
}