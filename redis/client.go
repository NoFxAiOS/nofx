package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client Redis客户端封装
type Client struct {
	rdb *redis.Client
	ctx context.Context
}

// NewClient 创建Redis客户端
func NewClient(addr string, password string, db int) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx := context.Background()

	// 测试连接
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("连接Redis失败: %w", err)
	}

	log.Printf("✓ Redis连接成功: %s (DB: %d)", addr, db)

	return &Client{
		rdb: rdb,
		ctx: ctx,
	}, nil
}

// Close 关闭Redis连接
func (c *Client) Close() error {
	return c.rdb.Close()
}

// SetRegistrationData 存储注册信息到Redis（带过期时间）
func (c *Client) SetRegistrationData(registrationID string, data map[string]interface{}, expiration time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化注册数据失败: %w", err)
	}

	key := fmt.Sprintf("registration:%s", registrationID)
	err = c.rdb.Set(c.ctx, key, jsonData, expiration).Err()
	if err != nil {
		return fmt.Errorf("存储注册数据到Redis失败: %w", err)
	}

	return nil
}

// GetRegistrationData 从Redis获取注册信息
func (c *Client) GetRegistrationData(registrationID string) (map[string]interface{}, error) {
	key := fmt.Sprintf("registration:%s", registrationID)
	val, err := c.rdb.Get(c.ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("注册信息不存在或已过期")
	} else if err != nil {
		return nil, fmt.Errorf("获取注册数据失败: %w", err)
	}

	var data map[string]interface{}
	err = json.Unmarshal([]byte(val), &data)
	if err != nil {
		return nil, fmt.Errorf("反序列化注册数据失败: %w", err)
	}

	return data, nil
}

// DeleteRegistrationData 删除注册信息
func (c *Client) DeleteRegistrationData(registrationID string) error {
	key := fmt.Sprintf("registration:%s", registrationID)
	err := c.rdb.Del(c.ctx, key).Err()
	if err != nil {
		return fmt.Errorf("删除注册数据失败: %w", err)
	}
	return nil
}

