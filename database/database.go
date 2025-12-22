package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

// DBConfig 数据库配置
type DBConfig struct {
	UseNeon    bool   // 是否使用Neon PostgreSQL
	NeonDSN    string // Neon连接字符串
	SQLitePath string // SQLite数据库路径
}

// Database 数据库接口
type Database interface {
	// 这里需要包含原来所有的数据库操作方法
	GetUserByEmail(email string) (*User, error)
	CreateUser(user *User) error
	// ... 其他方法
}

// User 用户结构体
type User struct {
	ID           int
	Email        string
	PasswordHash string
	IsAdmin      bool
	// ... 其他字段
}

// DatabaseImpl 双数据库实现
type DatabaseImpl struct {
	neonDB    *sql.DB // Neon PostgreSQL连接
	sqliteDB  *sql.DB // SQLite连接
	currentDB *sql.DB // 当前使用的数据库
	usingNeon bool    // 当前是否使用Neon
}

// NewDatabase 创建数据库实例
func NewDatabase(config DBConfig) (*DatabaseImpl, error) {
	dbImpl := &DatabaseImpl{}

	// 尝试连接Neon
	if config.UseNeon {
		neonDB, err := sql.Open("pgx", config.NeonDSN)
		if err != nil {
			log.Printf("⚠️  连接Neon失败: %v", err)
		} else {
			// 测试连接
			if err := neonDB.Ping(); err == nil {
				dbImpl.neonDB = neonDB
				dbImpl.currentDB = neonDB
				dbImpl.usingNeon = true
				log.Println("✅ 成功连接Neon PostgreSQL")
			} else {
				log.Printf("⚠️  Neon连接测试失败: %v", err)
			}
		}
	}

	// 如果Neon连接失败，连接SQLite
	if dbImpl.currentDB == nil {
		sqliteDB, err := sql.Open("sqlite3", config.SQLitePath)
		if err != nil {
			return nil, fmt.Errorf("同时连接Neon和SQLite失败: %w", err)
		}
		dbImpl.sqliteDB = sqliteDB
		dbImpl.currentDB = sqliteDB
		dbImpl.usingNeon = false
		log.Println("✅ 成功连接SQLite")
	}

	return dbImpl, nil
}

// Close 关闭数据库连接
func (db *DatabaseImpl) Close() error {
	var err error
	if db.neonDB != nil {
		if cerr := db.neonDB.Close(); cerr != nil {
			err = cerr
		}
	}
	if db.sqliteDB != nil {
		if cerr := db.sqliteDB.Close(); cerr != nil {
			err = cerr
		}
	}
	return err
}

// GetDB 获取底层的sql.DB连接
func (db *DatabaseImpl) GetDB() *sql.DB {
	return db.currentDB
}

// 示例方法：获取用户
func (db *DatabaseImpl) GetUserByEmail(email string) (*User, error) {
	query := "SELECT id, email, password_hash, is_admin FROM users WHERE email = $1"
	// 如果是SQLite，使用?代替$1
	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
	}

	row := db.currentDB.QueryRow(query, email)

	user := &User{}
	if err := row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.IsAdmin); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return user, nil
}

// GetSystemConfig 获取系统配置
func (db *DatabaseImpl) GetSystemConfig(key string) (string, error) {
	query := "SELECT value FROM system_config WHERE key = $1"
	// 如果是SQLite，使用?代替$1
	if !db.usingNeon {
		query = strings.ReplaceAll(query, "$1", "?")
	}

	row := db.currentDB.QueryRow(query, key)

	var value string
	if err := row.Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			// 如果 key 不存在，返回空字符串和 nil 错误
			return "", nil
		}
		return "", err
	}

	return value, nil
}

// 其他方法需要类似处理，确保SQL语法兼容
// ...
