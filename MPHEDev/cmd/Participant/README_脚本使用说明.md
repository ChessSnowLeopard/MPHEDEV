# 参与方程序自动化脚本使用说明

本目录包含了四个自动化脚本，用于简化参与方程序的编译、部署和启动过程。

## 脚本文件说明

### 1. `build_and_deploy.bat` - 完整构建和部署脚本
**功能：** 编译项目并自动部署到三个参与方项目
**使用场景：** 当你修改了源代码后，需要重新编译并部署

**使用方法：**
```bash
# 在 MPHEDev\cmd\Participant 目录下运行
build_and_deploy.bat
```

**执行步骤：**
1. 检查当前目录是否有 `main.go`
2. 使用 `go build` 编译生成 `Participant.exe`
3. 自动复制到三个参与方项目：
   - `../../../Participants/project-1/cmd/Participant/Participant.exe`
   - `../../../Participants/project-2/cmd/Participant/Participant.exe`
   - `../../../Participants/project-3/cmd/Participant/Participant.exe`

### 2. `copy_participants.bat` - 仅复制脚本
**功能：** 只复制已编译好的 `Participant.exe` 到三个参与方项目
**使用场景：** 当你已经编译好了程序，只需要部署时

**使用方法：**
```bash
# 在 MPHEDev\cmd\Participant 目录下运行
copy_participants.bat
```

**执行步骤：**
1. 检查当前目录是否有 `Participant.exe`
2. 直接复制到三个参与方项目（不重新编译）

### 3. `start_all_participants.bat` - 快速启动所有参与方脚本
**功能：** 一键启动所有三个参与方程序
**使用场景：** 当你想快速启动所有参与方进行测试时

**使用方法：**
```bash
# 在 MPHEDev\cmd\Participant 目录下运行
start_all_participants.bat
```

**执行步骤：**
1. 检查三个参与方程序是否存在
2. 依次启动三个参与方窗口（从各自的工作目录启动）
3. 提供操作指导

**重要特性：**
- 每个参与方从自己的目录启动，确保正确读取数据分片
- 自动切换工作目录，解决数据路径问题

### 4. `start_participant.bat` - 启动单个参与方脚本
**功能：** 启动指定的单个参与方程序
**使用场景：** 当你想单独启动某个参与方时

**使用方法：**
```bash
# 在 MPHEDev\cmd\Participant 目录下运行
start_participant.bat [参与方ID]

# 示例：
start_participant.bat 1  # 启动参与方 1
start_participant.bat 2  # 启动参与方 2
start_participant.bat 3  # 启动参与方 3
```

**执行步骤：**
1. 检查指定参与方程序是否存在
2. 从参与方自己的工作目录启动程序
3. 提供操作指导

## 推荐工作流程

### 开发阶段
1. 修改源代码
2. 运行 `build_and_deploy.bat` 编译并部署
3. 运行 `start_all_participants.bat` 启动所有参与方
4. 进行测试

### 快速测试阶段
1. 如果只是小修改，运行 `copy_participants.bat` 快速部署
2. 运行 `start_all_participants.bat` 启动所有参与方
3. 进行测试

### 单独调试阶段
1. 运行 `start_participant.bat 1` 启动参与方1
2. 运行 `start_participant.bat 2` 启动参与方2
3. 运行 `start_participant.bat 3` 启动参与方3

## 数据目录问题解决方案

### 问题描述
之前参与方程序启动时会从 `MPHEDev\data` 目录读取数据分片，而不是从各自项目目录下的 `data` 目录读取。

### 解决方案
所有启动脚本现在都会：
1. 切换到参与方自己的工作目录
2. 从正确的工作目录启动程序
3. 确保相对路径 `../../data/horizontal` 指向正确的数据目录

### 目录结构
```
MPHEDevCombine/
├── MPHEDev/
│   ├── cmd/
│   │   └── Participant/
│   │       ├── main.go
│   │       ├── build_and_deploy.bat
│   │       ├── copy_participants.bat
│   │       ├── start_all_participants.bat
│   │       └── start_participant.bat
│   └── data/
│       ├── horizontal/
│       └── vertical/
└── Participants/
    ├── project-1/
    │   ├── cmd/
    │   │   └── Participant/
    │   │       └── Participant.exe
    │   └── data/
    │       ├── horizontal/
    │       └── vertical/
    ├── project-2/
    │   ├── cmd/
    │   │   └── Participant/
    │   │       └── Participant.exe
    │   └── data/
    │       ├── horizontal/
    │       └── vertical/
    └── project-3/
        ├── cmd/
        │   └── Participant/
        │       └── Participant.exe
        └── data/
            ├── horizontal/
            └── vertical/
```

## 注意事项

1. **目录结构要求：**
   - 确保 `Participants` 文件夹在正确位置
   - 每个参与方项目都有自己的 `data` 目录

2. **前置条件：**
   - 确保 Go 环境已正确配置
   - 确保协调器已经启动并运行在 `http://localhost:8080`
   - 确保每个参与方项目都有正确的数据分片文件

3. **错误处理：**
   - 脚本会检查必要的文件和目录是否存在
   - 如果编译失败，会显示错误信息并暂停
   - 如果复制失败，会显示具体哪个项目复制失败

4. **工作目录：**
   - 启动脚本会自动切换到正确的工作目录
   - 确保参与方程序能正确读取自己的数据分片

## 故障排除

### 编译失败
- 检查 Go 环境是否正确安装
- 检查代码是否有语法错误
- 检查依赖包是否正确安装

### 复制失败
- 检查 `Participants` 文件夹路径是否正确
- 检查目标目录是否存在
- 检查文件权限

### 启动失败
- 确保先运行了部署脚本
- 检查参与方程序是否存在
- 确保协调器已启动

### 数据读取错误
- 确保每个参与方项目都有自己的 `data` 目录
- 确保数据分片文件存在且格式正确
- 使用启动脚本而不是手动启动程序

## 自定义配置

如果需要修改目标路径，可以编辑脚本中的以下变量：
```batch
set "TARGET_DIR=../../../Participants"
```

根据你的实际目录结构调整路径。 