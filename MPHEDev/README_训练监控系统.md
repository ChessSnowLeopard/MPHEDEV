# 实时训练监控系统使用说明

## 概述

本系统实现了基于同态加密的MNIST神经网络训练的实时监控功能，包括：

- 实时训练进度图表显示
- 训练历史记录存储和查询
- 自动数据刷新和状态更新
- 美观的前端界面

## 系统架构

```
前端 (Vue.js) ←→ 协调器 (Go) ←→ ctCNN训练程序 (Go)
     ↓              ↓                    ↓
  图表显示      训练历史API          训练数据解析
  状态监控      状态管理            实时输出解析
```

## 功能特性

### 1. 实时训练监控
- 自动检测训练开始/结束状态
- 实时更新训练进度图表
- 支持手动刷新和自动刷新模式

### 2. 训练数据可视化
- 损失值变化曲线
- 准确率变化曲线
- 学习率变化曲线
- 训练时间统计

### 3. 训练状态管理
- 训练状态：空闲/训练中/已完成
- 总训练轮数统计
- 最新轮次详细信息

## 安装和配置

### 1. 环境要求
- Go 1.19+
- Node.js 16+
- Python 3.8+ (用于测试)

### 2. 依赖安装

#### 后端依赖
```bash
cd MPHEDev
go mod tidy
```

#### 前端依赖
```bash
cd MPHEDev/MPHEDevFrontEnd/ParticipantFrontEnd/encryption
npm install
```

## 使用步骤

### 1. 启动协调器
```bash
cd MPHEDev
go run cmd/Coordinator/main.go
```
协调器将在 `http://localhost:8080` 启动

### 2. 启动参与方
```bash
cd MPHEDev
go run cmd/Participant/main.go
```
参与方将在 `http://localhost:8061` 启动

### 3. 启动前端
```bash
cd MPHEDev/MPHEDevFrontEnd/ParticipantFrontEnd/encryption
npm run serve
```
前端将在 `http://localhost:8030` 启动

### 4. 运行训练程序
```bash
cd MNIST_CNN
go run cmd/ctCNN/main.go
```

### 5. 访问训练监控页面
在浏览器中访问：`http://localhost:8030/#/training-chart`

## API接口

### 1. 获取训练历史
```
GET /training/history
```

响应格式：
```json
{
  "success": true,
  "data": [
    {
      "epoch": 1,
      "loss": 2.302585,
      "accuracy": 0.1000,
      "learningRate": 0.100000,
      "epochTime": "7.3s",
      "timestamp": 1640995200
    }
  ],
  "count": 1
}
```

### 2. 获取训练状态
```
GET /training/status
```

响应格式：
```json
{
  "success": true,
  "status": "training",
  "totalEpochs": 5,
  "lastEpoch": {
    "epoch": 5,
    "loss": 1.234567,
    "accuracy": 0.8500,
    "learningRate": 0.081000,
    "epochTime": "6.8s",
    "timestamp": 1640995260
  }
}
```

## 前端功能

### 1. 图表控制
- **刷新数据**: 手动获取最新训练数据
- **自动刷新**: 开启/关闭自动刷新模式（默认3秒间隔）

### 2. 状态显示
- 当前训练状态（空闲/训练中/已完成）
- 总训练轮数
- 最新轮次的详细信息

### 3. 数据表格
- 显示所有训练轮次的详细数据
- 包括轮次、损失值、准确率、学习率、用时

## 测试

### 1. API测试
```bash
cd MPHEDev
python test_training_api.py
```

### 2. 前端测试
```bash
cd MPHEDev/MPHEDevFrontEnd/ParticipantFrontEnd/encryption
npm run test:unit
```

## 故障排除

### 1. 前端无法连接后端
- 检查后端服务是否正常运行
- 确认端口配置是否正确
- 检查防火墙设置

### 2. 训练数据不更新
- 确认ctCNN程序正在运行
- 检查训练输出格式是否正确
- 查看后端日志确认数据解析是否成功

### 3. 图表显示异常
- 检查浏览器控制台是否有错误
- 确认Chart.js库是否正确加载
- 验证数据格式是否符合预期

## 开发说明

### 1. 后端开发
- 训练数据解析逻辑在 `coordinator_handlers.go` 中
- 训练历史存储使用内存存储，重启后会丢失
- 可以扩展为数据库存储以持久化数据

### 2. 前端开发
- 图表组件使用Chart.js库
- 自动刷新逻辑在 `TrainingChart.vue` 中
- API调用封装在 `training.js` 中

### 3. 数据格式
训练输出格式必须包含：
```
✓ Epoch 1: 损失=2.302585, 准确率=10.00%, 学习率=0.100000, 用时=7.3s
```

## 扩展功能

### 1. 数据持久化
- 添加数据库支持（如SQLite、PostgreSQL）
- 实现训练历史数据的持久化存储

### 2. 多用户支持
- 添加用户认证和授权
- 支持多用户同时监控不同训练任务

### 3. 高级图表
- 添加更多图表类型（如散点图、热力图）
- 支持图表交互和数据钻取

### 4. 告警功能
- 训练异常告警
- 性能指标监控
- 邮件/短信通知

## 许可证

本项目采用MIT许可证，详见LICENSE文件。 