package main

import (
	"fmt"
	"log"
	"nofx/config"
	"nofx/crypto"
	"os"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                  API Key 加密/解密测试工具                       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 1. 初始化加密服务
	fmt.Println("📝 步骤 1/4: 初始化加密服务...")
	cryptoService, err := crypto.NewCryptoService("secrets/rsa_key")
	if err != nil {
		log.Fatalf("❌ 初始化加密服务失败: %v", err)
	}
	fmt.Println("✅ 加密服务初始化成功")
	fmt.Println()

	// 2. 初始化数据库
	fmt.Println("📝 步骤 2/4: 初始化数据库...")
	dbPath := "config.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}
	database, err := config.NewDatabase(dbPath)
	if err != nil {
		log.Fatalf("❌ 初始化数据库失败: %v", err)
	}
	defer database.Close()
	database.SetCryptoService(cryptoService)
	fmt.Println("✅ 数据库初始化成功")
	fmt.Println()

	// 3. 测试加密/解密功能
	fmt.Println("📝 步骤 3/4: 测试加密/解密功能...")
	testAPIKey := "sk-aaf1dfce40b743109919afe20668b320"

	// 加密
	encrypted, err := cryptoService.EncryptForStorage(testAPIKey)
	if err != nil {
		log.Fatalf("❌ 加密失败: %v", err)
	}
	fmt.Printf("   原始 API Key: %s\n", testAPIKey)
	fmt.Printf("   加密后: %s\n", encrypted[:50]+"...")
	fmt.Printf("   加密后: %s\n", encrypted)

	// 解密
	decrypted, err := cryptoService.DecryptFromStorage(encrypted)
	if err != nil {
		log.Fatalf("❌ 解密失败: %v", err)
	}

	if decrypted != testAPIKey {
		log.Fatalf("❌ 解密结果不匹配: 期望 %s, 得到 %s", testAPIKey, decrypted)
	}
	fmt.Println("✅ 加密/解密测试通过")
	fmt.Println()

	// 4. 查询数据库中的AI模型API Key
	fmt.Println("📝 步骤 4/4: 查询数据库中的AI模型配置...")
	userID := "default"
	models, err := database.GetAIModels(userID)
	if err != nil {
		log.Fatalf("❌ 查询AI模型失败: %v", err)
	}

	if len(models) == 0 {
		fmt.Println("⚠️  数据库中没有配置AI模型")
		fmt.Println()
		fmt.Println("💡 提示: 可以通过以下方式添加AI模型:")
		fmt.Println("   1. 通过Web界面添加")
		fmt.Println("   2. 使用API: POST /api/ai-models")
		fmt.Println("   3. 直接操作数据库")
	} else {
		fmt.Printf("✅ 找到 %d 个AI模型配置:\n", len(models))
		fmt.Println()
		for i, model := range models {
			fmt.Printf("   [%d] %s (%s)\n", i+1, model.Name, model.ID)
			fmt.Printf("       提供商: %s\n", model.Provider)
			fmt.Printf("       状态: %s\n", map[bool]string{true: "启用", false: "禁用"}[model.Enabled])

			// 显示API Key（部分隐藏）
			if model.APIKey != "" {
				maskedKey := maskAPIKey(model.APIKey)
				fmt.Printf("       API Key: %s\n", maskedKey)

				// 验证解密后的API Key是否有效
				if len(model.APIKey) > 0 {
					fmt.Printf("       ✅ API Key 已正确解密\n")
				}
			} else {
				fmt.Printf("       API Key: (未设置)\n")
			}
			fmt.Println()
		}
	}

	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                          测试完成                                ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
}

// maskAPIKey 隐藏API Key的中间部分
func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	if len(key) <= 16 {
		return key[:4] + "****" + key[len(key)-4:]
	}
	return key[:6] + "****" + key[len(key)-6:]
}
