package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	baseURL = "https://nofx-gyc567.replit.app/api"
	// 从浏览器获取的token（需要替换）
	authToken = "YOUR_AUTH_TOKEN_HERE"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║            测试所有前端API接口                              ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 1. 测试 /api/competition - 竞赛模式汇总
	fmt.Println("【1】测试 /api/competition (竞赛模式汇总)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	testCompetitionAPI()
	fmt.Println()

	// 2. 测试 /api/account - 账户汇总
	fmt.Println("【2】测试 /api/account (账户汇总)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	testAccountAPI()
	fmt.Println()

	// 3. 测试 /api/my-traders - 所有交易员列表
	fmt.Println("【3】测试 /api/my-traders (交易员列表)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	traderID := testMyTradersAPI()
	fmt.Println()

	// 4. 测试单个交易员详情（如果有traderID）
	if traderID != "" {
		fmt.Println("【4】测试 /api/my-traders/" + traderID + " (交易员详情)")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		testTraderDetailAPI(traderID)
		fmt.Println()
	}

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    分析总结                                 ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("说明：")
	fmt.Println("  - 如果 /api/competition 或 /api/account 显示 0，但交易员详情显示正确")
	fmt.Println("  - 说明主页汇总逻辑和详情页逻辑使用了不同的数据获取方式")
	fmt.Println("  - 需要检查后端代码中汇总统计的实现")
	fmt.Println()
}

func testCompetitionAPI() {
	url := baseURL + "/competition"
	resp, err := makeRequest(url)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Printf("❌ 解析JSON失败: %v\n", err)
		fmt.Printf("原始响应: %s\n", string(resp))
		return
	}

	// 打印关键字段
	if data, ok := result["data"].(map[string]interface{}); ok {
		if traders, ok := data["traders"].([]interface{}); ok && len(traders) > 0 {
			fmt.Printf("✓ 找到 %d 个交易员\n", len(traders))

			// 打印第一个交易员的详细信息
			if trader, ok := traders[0].(map[string]interface{}); ok {
				fmt.Println("\n第一个交易员数据：")
				printTraderInfo(trader)
			}
		}

		// 打印汇总数据
		if summary, ok := data["summary"].(map[string]interface{}); ok {
			fmt.Println("\n汇总数据：")
			printJSON(summary)
		}
	}

	// 打印完整响应以便分析
	fmt.Println("\n完整响应结构：")
	printJSON(result)
}

func testAccountAPI() {
	url := baseURL + "/account"
	resp, err := makeRequest(url)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Printf("❌ 解析JSON失败: %v\n", err)
		fmt.Printf("原始响应: %s\n", string(resp))
		return
	}

	// 打印账户信息
	if data, ok := result["data"].(map[string]interface{}); ok {
		fmt.Println("账户信息：")

		// 重点关注这些字段
		keys := []string{
			"total_equity",
			"wallet_balance",
			"available_balance",
			"total_pnl",
			"total_pnl_pct",
			"unrealized_profit",
		}

		for _, key := range keys {
			if val, ok := data[key]; ok {
				fmt.Printf("  %s: %v\n", key, val)
			}
		}
	}

	fmt.Println("\n完整响应：")
	printJSON(result)
}

func testMyTradersAPI() string {
	url := baseURL + "/my-traders"
	resp, err := makeRequest(url)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return ""
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Printf("❌ 解析JSON失败: %v\n", err)
		fmt.Printf("原始响应: %s\n", string(resp))
		return ""
	}

	var firstTraderID string

	if data, ok := result["data"].([]interface{}); ok {
		fmt.Printf("✓ 找到 %d 个交易员\n", len(data))

		if len(data) > 0 {
			if trader, ok := data[0].(map[string]interface{}); ok {
				if id, ok := trader["id"].(string); ok {
					firstTraderID = id
				}
				fmt.Println("\n第一个交易员概要：")
				printTraderInfo(trader)
			}
		}
	}

	fmt.Println("\n完整响应：")
	printJSON(result)

	return firstTraderID
}

func testTraderDetailAPI(traderID string) {
	url := baseURL + "/my-traders/" + traderID
	resp, err := makeRequest(url)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Printf("❌ 解析JSON失败: %v\n", err)
		fmt.Printf("原始响应: %s\n", string(resp))
		return
	}

	if data, ok := result["data"].(map[string]interface{}); ok {
		fmt.Println("交易员详情：")
		printTraderInfo(data)
	}

	fmt.Println("\n完整响应：")
	printJSON(result)
}

func printTraderInfo(trader map[string]interface{}) {
	keys := []string{
		"id", "name", "exchange",
		"total_equity", "available_balance",
		"total_pnl", "total_pnl_pct",
		"position_count", "margin_used_pct",
	}

	for _, key := range keys {
		if val, ok := trader[key]; ok {
			fmt.Printf("  %s: %v\n", key, val)
		}
	}
}

func makeRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 如果需要认证token，添加到header
	if authToken != "YOUR_AUTH_TOKEN_HERE" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func printJSON(data interface{}) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("无法格式化JSON: %v\n", err)
		return
	}

	// 只打印前2000个字符，避免输出过长
	str := string(jsonBytes)
	if len(str) > 2000 {
		fmt.Println(str[:2000])
		fmt.Printf("\n... (省略 %d 个字符)\n", len(str)-2000)
	} else {
		fmt.Println(str)
	}
}
