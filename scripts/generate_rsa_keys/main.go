package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func main() {
	keysDir := "keys"
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		fmt.Printf("创建keys目录失败: %v\n", err)
		return
	}

	privateKeyPath := filepath.Join(keysDir, "rsa_private.key")
	publicKeyPath := filepath.Join(keysDir, "rsa_private.key.pub")

	if _, err := os.Stat(privateKeyPath); err == nil {
		fmt.Println("RSA密钥对已存在:")
		fmt.Printf("  私钥: %s\n", privateKeyPath)
		fmt.Printf("  公钥: %s\n", publicKeyPath)

		publicKeyPEM, err := ioutil.ReadFile(publicKeyPath)
		if err == nil {
			fmt.Println("\n公钥内容:")
			fmt.Println(string(publicKeyPEM))
		}
		return
	}

	fmt.Println("生成新的RSA密钥对...")
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Printf("生成RSA密钥失败: %v\n", err)
		return
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	if err := ioutil.WriteFile(privateKeyPath, privateKeyPEM, 0600); err != nil {
		fmt.Printf("保存私钥失败: %v\n", err)
		return
	}

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		fmt.Printf("编码公钥失败: %v\n", err)
		return
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	if err := ioutil.WriteFile(publicKeyPath, publicKeyPEM, 0644); err != nil {
		fmt.Printf("保存公钥失败: %v\n", err)
		return
	}

	fmt.Println("✓ RSA密钥对生成成功!")
	fmt.Printf("  私钥: %s\n", privateKeyPath)
	fmt.Printf("  公钥: %s\n", publicKeyPath)
	fmt.Println("\n公钥内容（可用于前端配置）:")
	fmt.Println(string(publicKeyPEM))
	fmt.Println("\n注意: 请妥善保管私钥文件，不要提交到版本控制系统中！")
}
