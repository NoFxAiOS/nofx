package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func main() {
	SECRETS_DIR := "secrets"
	DATA_KEY_FILE := filepath.Join(SECRETS_DIR, "data_key")

	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   数据加密密钥生成器                             ║")
	fmt.Println("║                    AES-256 数据加密密钥                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 创建 secrets 目录
	if err := os.MkdirAll(SECRETS_DIR, 0700); err != nil {
		log.Fatalf("❌ 创建目录失败: %v", err)
	}
	fmt.Printf("✓ %s 目录已准备\n", SECRETS_DIR)

	// 检查现有密钥
	if _, err := os.Stat(DATA_KEY_FILE); err == nil {
		fmt.Printf("⚠️  检测到现有的数据加密密钥文件: %s\n", DATA_KEY_FILE)
		fmt.Print("是否覆盖现有密钥? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("ℹ️  操作已取消")
			return
		}
		os.Remove(DATA_KEY_FILE)
		fmt.Println("🗑️  已删除现有密钥文件")
	}

	fmt.Println()
	fmt.Println("🔐 开始生成 AES-256 数据加密密钥...")

	// 生成 32 字节的随机密钥
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		log.Fatalf("❌ 生成随机密钥失败: %v", err)
	}

	// Base64 编码
	encoded := base64.StdEncoding.EncodeToString(raw)

	// 保存密钥文件
	if err := ioutil.WriteFile(DATA_KEY_FILE, []byte(encoded+"\n"), 0600); err != nil {
		log.Fatalf("❌ 保存密钥文件失败: %v", err)
	}

	fmt.Println("✓ 数据加密密钥生成成功")
	fmt.Println("✓ 密钥文件权限设置为 600")

	// 显示密钥信息
	fmt.Println()
	fmt.Println("🎉 数据加密密钥生成成功!")
	fmt.Println()
	fmt.Println("📋 密钥信息:")
	fmt.Printf("  密钥文件: %s\n", DATA_KEY_FILE)
	fmt.Printf("  密钥长度: 32 bytes (256 bits)\n")
	fmt.Printf("  编码格式: Base64\n")

	// 显示文件大小
	fileInfo, _ := os.Stat(DATA_KEY_FILE)
	fmt.Println()
	fmt.Println("📏 文件大小:")
	fmt.Printf("  密钥文件: %d bytes\n", fileInfo.Size())

	// 显示密钥值（用于环境变量）
	fmt.Println()
	fmt.Println("📋 环境变量配置:")
	fmt.Printf("  变量名: DATA_ENCRYPTION_KEY\n")
	fmt.Printf("  变量值: %s\n", encoded)
	fmt.Println()
	fmt.Println("💡 使用说明:")
	fmt.Println("  1. 本地开发: 密钥文件已保存，程序会自动读取")
	fmt.Println("  2. Docker环境: 在 docker-compose.yml 中设置环境变量:")
	fmt.Printf("     DATA_ENCRYPTION_KEY=%s\n", encoded)
	fmt.Println("  3. 生产环境: 建议使用密钥管理服务存储密钥")
	fmt.Println()
	fmt.Println("⚠️  安全提醒:")
	fmt.Println("  • 密钥文件权限已设置为 600 (仅所有者可读写)")
	fmt.Println("  • 请定期备份密钥文件")
	fmt.Println("  • 不要将密钥文件提交到版本控制系统")
	fmt.Println("  • 建议在不同环境使用不同的密钥")
	fmt.Println()
	fmt.Println("✅ 数据加密密钥生成完成!")
}
