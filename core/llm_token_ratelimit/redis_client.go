// Copyright 1999-2020 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package llmtokenratelimit

import (
	"fmt"
	"sync"
	"time"

	redis "github.com/go-redis/redis/v7"
)

var (
	redisClientMu      sync.RWMutex
	redisClusterClient *redis.ClusterClient
)

func initRedisClusterClient() error {
	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	serviceName := cfg.Redis.ServiceName
	servicePort := cfg.Redis.ServicePort
	timeout := time.Duration(cfg.Redis.Timeout) * time.Millisecond
	poolSize := cfg.Redis.PoolSize
	minIdleConns := cfg.Redis.MinIdleConns
	maxRetries := cfg.Redis.MaxRetries

	if len(serviceName) == 0 {
		serviceName = DefaultRedisServiceName
	}
	if servicePort == 0 {
		servicePort = DefaultRedisServicePort
	}
	if timeout == 0 {
		timeout = time.Duration(DefaultRedisTimeout) * time.Millisecond
	}
	if poolSize == 0 {
		poolSize = DefaultRedisPoolSize
	}
	if minIdleConns == 0 {
		minIdleConns = DefaultRedisMinIdleConns
	}
	if maxRetries == 0 {
		maxRetries = DefaultRedisMaxRetries
	}

	addr := fmt.Sprintf("%s:%d", serviceName, servicePort)

	newClient := redis.NewClusterClient(
		&redis.ClusterOptions{
			Addrs: []string{addr},

			Username: cfg.Redis.Username,
			Password: cfg.Redis.Password,

			DialTimeout:  timeout,
			ReadTimeout:  timeout,
			WriteTimeout: timeout,
			PoolTimeout:  timeout,

			PoolSize:     int(poolSize),
			MinIdleConns: int(minIdleConns),
			MaxRetries:   int(maxRetries),
		},
	)

	if _, err := newClient.Ping().Result(); err != nil {
		return fmt.Errorf("failed to connect to redis cluster: %v", err)
	}
	// Perform lock replacement only after the new client successfully connects;
	// otherwise, a deadlock will occur if the connection fails
	redisClientMu.Lock()
	defer redisClientMu.Unlock()

	if redisClusterClient != nil {
		redisClusterClient.Close()
	}

	redisClusterClient = newClient
	return nil
}

func getRedisClient() *redis.ClusterClient {
	redisClientMu.RLock()
	defer redisClientMu.RUnlock()
	return redisClusterClient
}
