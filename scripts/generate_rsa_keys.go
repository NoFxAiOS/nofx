package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func main() {
	RSA_KEY_SIZE := 2048
	SECRETS_DIR := "secrets"
	PRIVATE_KEY_FILE := filepath.Join(SECRETS_DIR, "rsa_key")
	PUBLIC_KEY_FILE := filepath.Join(SECRETS_DIR, "rsa_key.pub")

	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   RSA密钥生成器                                  ║")
	fmt.Println("║                     RSA-2048 混合加密密钥对                       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 创建 secrets 目录
	if err := os.MkdirAll(SECRETS_DIR, 0700); err != nil {
		log.Fatalf("❌ 创建目录失败: %v", err)
	}
	fmt.Printf("✓ %s 目录已准备\n", SECRETS_DIR)

	// 检查现有密钥
	if _, err := os.Stat(PRIVATE_KEY_FILE); err == nil {
		fmt.Printf("⚠️  检测到现有的RSA密钥文件: %s\n", PRIVATE_KEY_FILE)
		fmt.Print("是否覆盖现有密钥? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("ℹ️  操作已取消")
			return
		}
		os.Remove(PRIVATE_KEY_FILE)
		os.Remove(PUBLIC_KEY_FILE)
		fmt.Println("🗑️  已删除现有密钥文件")
	}

	fmt.Println()
	fmt.Printf("🔐 开始生成 RSA-%d 密钥对...\n", RSA_KEY_SIZE)

	// 生成 RSA 密钥对
	fmt.Printf("📝 步骤 1/3: 生成 RSA 私钥 (%d bits)...\n", RSA_KEY_SIZE)
	privateKey, err := rsa.GenerateKey(rand.Reader, RSA_KEY_SIZE)
	if err != nil {
		log.Fatalf("❌ 私钥生成失败: %v", err)
	}
	fmt.Println("✓ 私钥生成成功")

	// 编码私钥
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// 保存私钥
	if err := ioutil.WriteFile(PRIVATE_KEY_FILE, privateKeyPEM, 0600); err != nil {
		log.Fatalf("❌ 保存私钥失败: %v", err)
	}
	fmt.Println("✓ 私钥权限设置为 600")

	// 生成公钥
	fmt.Printf("📝 步骤 2/3: 从私钥提取公钥...\n")
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		log.Fatalf("❌ 公钥编码失败: %v", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	// 保存公钥
	if err := ioutil.WriteFile(PUBLIC_KEY_FILE, publicKeyPEM, 0644); err != nil {
		log.Fatalf("❌ 保存公钥失败: %v", err)
	}
	fmt.Println("✓ 公钥生成成功")
	fmt.Println("✓ 公钥权限设置为 644")

	// 验证密钥
	fmt.Printf("📝 步骤 3/3: 验证密钥对...\n")
	// 读取并解析私钥验证
	readPrivateKeyPEM, err := ioutil.ReadFile(PRIVATE_KEY_FILE)
	if err != nil {
		log.Fatalf("❌ 读取私钥失败: %v", err)
	}
	block, _ := pem.Decode(readPrivateKeyPEM)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		log.Fatalf("❌ 私钥格式无效")
	}
	_, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("❌ 私钥验证失败: %v", err)
	}
	fmt.Println("✓ 私钥验证通过")

	// 读取并解析公钥验证
	readPublicKeyPEM, err := ioutil.ReadFile(PUBLIC_KEY_FILE)
	if err != nil {
		log.Fatalf("❌ 读取公钥失败: %v", err)
	}
	block, _ = pem.Decode(readPublicKeyPEM)
	if block == nil || block.Type != "PUBLIC KEY" {
		log.Fatalf("❌ 公钥格式无效")
	}
	_, err = x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		log.Fatalf("❌ 公钥验证失败: %v", err)
	}
	fmt.Println("✓ 公钥验证通过")

	// 显示密钥信息
	fmt.Println()
	fmt.Println("🎉 RSA密钥对生成成功!")
	fmt.Println()
	fmt.Println("📋 密钥信息:")
	fmt.Printf("  私钥文件: %s\n", PRIVATE_KEY_FILE)
	fmt.Printf("  公钥文件: %s\n", PUBLIC_KEY_FILE)
	fmt.Printf("  密钥大小: %d bits\n", RSA_KEY_SIZE)

	// 显示文件大小
	privateInfo, _ := os.Stat(PRIVATE_KEY_FILE)
	publicInfo, _ := os.Stat(PUBLIC_KEY_FILE)
	fmt.Println()
	fmt.Println("📏 文件大小:")
	fmt.Printf("  私钥: %d bytes\n", privateInfo.Size())
	fmt.Printf("  公钥: %d bytes\n", publicInfo.Size())

	fmt.Println()
	fmt.Println("✅ RSA密钥对生成完成!")
	fmt.Println()
	fmt.Println("📋 使用说明:")
	fmt.Printf("  1. 私钥文件 (%s) 用于服务器端解密\n", PRIVATE_KEY_FILE)
	fmt.Printf("  2. 公钥文件 (%s) 可以分发给客户端用于加密\n", PUBLIC_KEY_FILE)
	fmt.Println("  3. 确保私钥文件的安全性，不要泄露给第三方")
	fmt.Println("  4. 在生产环境中，建议将私钥存储在安全的密钥管理服务中")
	fmt.Println()
	fmt.Println("⚠️  安全提醒:")
	fmt.Println("  • 私钥文件权限已设置为 600 (仅所有者可读写)")
	fmt.Println("  • 请定期备份密钥文件")
	fmt.Println("  • 建议在不同环境使用不同的密钥对")
}
