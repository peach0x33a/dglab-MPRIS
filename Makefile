# Makefile for dglab-MPRIS
# 编译产物统一输出到 bin/ 目录

.PHONY: all build clean run install

# 输出目录
BIN_DIR := bin

# 编译产物名称
BIN_NAME := dglab-mpris

# Go 相关
GOCMD := go
GOBUILD := $(GOCMD) build
GOMOD := $(GOCMD) mod

# 目标平台（可选）
# GOOS := linux
# GOARCH := amd64

all: build

# 默认构建到 bin/ 目录
build:
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(BIN_NAME) .

# 直接运行（用于开发调试）
run:
	$(GOCMD) run .

# 清理构建产物
clean:
	rm -rf $(BIN_DIR)

# 安装依赖
deps:
	$(GOMOD) tidy
