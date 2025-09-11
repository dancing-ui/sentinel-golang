#!/bin/zsh

mkdir -p ./out

# 编译go代码
go build -o out/llm_token_ratelimit_benchmark main.go
# 复制 sentinel 配置文件到输出目录
cp sentinel.yml out/