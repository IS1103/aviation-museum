package etcd

import (
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Client etcd 客戶端封裝
type Client struct {
	Cli *clientv3.Client
}

var global *Client

// Init 初始化 etcd 客戶端
func Init(endpoints []string, dialTimeout time.Duration) error {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: dialTimeout,
	})
	if err != nil {
		return err
	}
	global = &Client{Cli: cli}
	return nil
}

// Default 獲取全局 etcd 客戶端實例
func Default() *Client {
	return global
}

// Close 關閉 etcd 客戶端
func (c *Client) Close() error {
	if c.Cli != nil {
		return c.Cli.Close()
	}
	return nil
}
