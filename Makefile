.PHONY: swag-init help

# 获取 GOPATH 和 GOBIN
GOPATH := $(shell go env GOPATH)
GOBIN := $(shell go env GOBIN)
# 如果 GOBIN 设置了，使用 GOBIN，否则使用 GOPATH/bin
SWAG_BIN_DIR := $(if $(GOBIN),$(GOBIN),$(GOPATH)/bin)
SWAG_PATH := $(SWAG_BIN_DIR)/swag

# 默认目标
help:
	@echo "可用的命令:"
	@echo "  make swag-init  - 检查并安装 swag，然后生成 Swagger 文档"

# Swagger 初始化
swag-init:
	@echo "检查 swag 是否已安装..."
	@$(SWAG_PATH) --version > /dev/null 2>&1 || (echo "swag 未安装，正在安装..." && go install github.com/swaggo/swag/cmd/swag@latest)
	@echo "swag 已就绪，开始生成 Swagger 文档..."
	@PATH="$(SWAG_BIN_DIR):$$PATH" $(SWAG_PATH) init
	@echo "Swagger 文档生成完成！"
