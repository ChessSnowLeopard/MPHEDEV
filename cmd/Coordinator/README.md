# Coordinator 模块化重构架构

## 概述

Coordinator模块已完成模块化重构，将原有大结构体和路由处理器拆分为多个职责明确的子模块，提升了可维护性和可扩展性。

## 目录结构

```
cmd/Coordinator/
├── main.go                  # 主程序入口
├── services/                # 主协调器结构体
│   └── coordinator.go
├── participants/            # 参与方管理模块
│   └── manager.go
├── parameters/              # 参数管理模块
│   └── manager.go
├── keys/                    # 密钥管理与聚合模块
│   ├── manager.go
│   ├── aggregator.go
│   └── testing.go
├── server/                  # HTTP服务器模块
│   └── http_server.go
├── utils/                   # 工具与类型定义
│   └── types.go
```

## 各模块职责

- **main.go**：程序入口，读取参与方数量，启动Coordinator。
- **services/coordinator.go**：主协调器结构体，统一调度各子模块，注册所有HTTP路由。
- **participants/manager.go**：负责参与方注册、URL管理、心跳与在线状态管理。
- **parameters/manager.go**：负责CKKS参数、CRP、伽罗瓦元素等参数的生成与分发。
- **keys/manager.go**：负责密钥份额的存储、全局密钥的管理。
- **keys/aggregator.go**：负责公钥、私钥、伽罗瓦密钥、重线性化密钥的聚合。
- **keys/testing.go**：负责密钥测试功能。
- **server/http_server.go**：Gin HTTP服务器的统一启动与管理。
- **utils/types.go**：常用数据结构定义。

## 使用方法

### 编译
```bash
cd cmd/Coordinator
go build -o Coordinator.exe .
```

### 运行
```bash
./Coordinator.exe
```

程序会提示输入参与方数量，随后启动HTTP服务，等待参与方注册。

## 重构优势
- **职责分离**：每个模块只负责单一功能，便于维护和扩展。
- **接口统一**：所有HTTP路由由主结构体注册，便于管理。
- **易于扩展**：新增功能只需扩展对应模块。
- **无冗余代码**：已清理所有依赖旧结构体的无用文件和方法。

## 迁移说明
- 旧的 services/coordinator.go、key_aggregation.go、handlers/ 目录等已全部删除。
- 所有功能已迁移到新结构体和新模块。
- 仅保留一份主协调器结构体和各独立子模块。

---
如需进一步扩展（如增加日志、配置、监控等），可直接在对应模块实现，无需大幅改动主结构。

## 密钥测试功能

新增的密钥测试功能可以验证生成的密钥是否正确工作：

### 测试类型
1. **公钥测试**: 验证加密和解密功能
2. **重线性化密钥测试**: 验证密文乘法运算
3. **伽罗瓦密钥测试**: 验证旋转和置换操作
4. **全密钥测试**: 一次性测试所有密钥

### 测试方法

#### 1. 自动测试
- **自动聚合**: 当所有参与方的份额收集完成时，系统会自动触发密钥聚合
- **自动测试**: 密钥聚合完成后会自动进行测试，确保密钥质量
- **最终测试**: 所有密钥生成完成后会进行最终的综合测试

#### 2. HTTP API测试
```bash
# 测试所有密钥
curl -X POST http://localhost:8080/test/all

# 仅测试公钥
curl -X POST http://localhost:8080/test/public

# 仅测试重线性化密钥
curl -X POST http://localhost:8080/test/relin

# 仅测试伽罗瓦密钥
curl -X POST http://localhost:8080/test/galois
```

#### 3. 程序化测试
```go
// 测试所有密钥
err := coordinator.TestAllKeys()

// 仅测试公钥
err := coordinator.TestPublicKeyOnly()

// 仅测试重线性化密钥
err := coordinator.TestRelinearizationKeyOnly()

// 仅测试伽罗瓦密钥
err := coordinator.TestGaloisKeysOnly()
```

### 输出优化

系统已优化输出信息，减少不必要的详细输出：

- **进度显示**: 只在关键节点显示收集进度（每5个或第1个）
- **完成提示**: 当所有份额收集完成时显示汇总信息
- **聚合状态**: 清晰显示聚合过程状态
- **测试结果**: 突出显示测试结果和错误信息

### 流程自动化

1. **份额收集**: 参与方提交密钥份额
2. **自动聚合**: 份额收集完成后自动触发聚合
3. **自动测试**: 聚合完成后自动进行密钥测试
4. **状态检查**: 持续检查所有密钥完成状态
5. **最终验证**: 所有密钥完成后进行最终综合测试

## 使用方法

### 编译
```bash
go build -o Coordinator.exe .
```

### 运行
```bash
./Coordinator.exe
```

### 配置
默认配置：
- 监听端口: 8080
- 最小参与方数量: 3
- 预期参与方数量: 5

## API接口

### 参与方管理
- `POST /register` - 注册参与方
- `GET /participants` - 获取参与方列表
- `POST /participants/url` - 报告参与方URL
- `GET /participants/list` - 获取参与方URL列表

### 参数管理
- `GET /params/ckks` - 获取CKKS参数

### 密钥管理
- `POST /keys/public` - 提交公钥份额
- `POST /keys/secret` - 提交私钥份额
- `POST /keys/galois` - 提交伽罗瓦密钥份额
- `POST /keys/relin` - 提交重线性化密钥份额
- `GET /keys/relin/round1` - 获取第一轮聚合结果

### 状态查询
- `GET /setup/status` - 获取设置状态
- `GET /participants/online` - 获取在线参与方
- `GET /status/online` - 获取在线状态

### 密钥测试
- `POST /test/all` - 测试所有密钥
- `POST /test/public` - 测试公钥
- `POST /test/relin` - 测试重线性化密钥
- `POST /test/galois` - 测试伽罗瓦密钥

## 重构优势
- **模块化设计**：功能分离，职责明确
- **易于维护**：每个模块独立，修改影响范围小
- **可扩展性**：新增功能只需添加相应模块
- **测试友好**：支持密钥功能验证
- **代码复用**：模块间松耦合，便于复用

## 迁移指南

### 从旧版本迁移
1. 停止旧版本服务
2. 备份配置文件（如有）
3. 部署新版本
4. 启动服务
5. 验证功能正常

### 配置变更
- 新增测试相关配置
- 保持API兼容性
- 支持渐进式迁移

## 注意事项

1. 确保所有参与方都已注册后再进行密钥聚合
2. 密钥测试需要完整的密钥集，请确保所有密钥都已聚合完成
3. 测试失败可能表明密钥生成有问题，需要检查参与方状态
4. 建议在生产环境中定期进行密钥测试 