package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/j1udu/cloud-storage-system/backend/internal/config"
)

// InitMySQL 创建并返回一个 MySQL 连接池
func InitMySQL(cfg config.MySQLConfig) (*sql.DB, error) {
	// 拼接连接字符串: user:password@tcp(host:port)/database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

	// 打开数据库连接
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 配置连接池
	db.SetMaxOpenConns(cfg.MaxOpenConns)     // 最大同时打开的连接数
	db.SetMaxIdleConns(cfg.MaxIdleConns)     // 最大空闲连接数
	db.SetConnMaxLifetime(time.Hour)         // 连接最长存活 1 小时后回收

	// 验证连接是否真的通了
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	return db, nil
}
