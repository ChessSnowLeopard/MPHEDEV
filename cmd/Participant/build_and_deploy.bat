@echo off
chcp 65001 >nul
echo ========================================
echo 参与方程序构建和部署脚本
echo ========================================

:: 检查是否在正确的目录
if not exist "main.go" (
    echo [错误] 当前目录下没有找到main.go
    echo 请确保在MPHEDev\cmd\Participant目录下运行此脚本
    pause
    exit /b 1
)

echo [步骤1] 开始编译Participant.exe...
echo.

:: 编译项目
go build -o Participant.exe main.go

if %errorlevel% neq 0 (
    echo [错误] 编译失败！
    echo 请检查代码是否有语法错误
    pause
    exit /b 1
)

echo [成功] 编译完成，生成Participant.exe
echo.

:: 检查编译结果
if not exist "Participant.exe" (
    echo [错误] 编译后没有找到Participant.exe
    pause
    exit /b 1
)

echo [步骤2] 开始部署到三个参与方项目...
echo.

:: 定义目标路径
set "TARGET_DIR=../../../Participants"
set "SOURCE_EXE=Participant.exe"

:: 检查目标目录是否存在
if not exist "%TARGET_DIR%" (
    echo [错误] 目标目录 %TARGET_DIR% 不存在
    echo 请确保Participants文件夹在正确位置
    pause
    exit /b 1
)

:: 复制到project-1
if exist "%TARGET_DIR%\project-1\cmd\Participant\" (
    copy /Y "%SOURCE_EXE%" "%TARGET_DIR%\project-1\cmd\Participant\Participant.exe" >nul
    if %errorlevel% equ 0 (
        echo [成功] 已复制到 project-1
    ) else (
        echo [错误] 复制到 project-1 失败
    )
) else (
    echo [警告] project-1 目标目录不存在，跳过
)

:: 复制到project-2
if exist "%TARGET_DIR%\project-2\cmd\Participant\" (
    copy /Y "%SOURCE_EXE%" "%TARGET_DIR%\project-2\cmd\Participant\Participant.exe" >nul
    if %errorlevel% equ 0 (
        echo [成功] 已复制到 project-2
    ) else (
        echo [错误] 复制到 project-2 失败
    )
) else (
    echo [警告] project-2 目标目录不存在，跳过
)

:: 复制到project-3
if exist "%TARGET_DIR%\project-3\cmd\Participant\" (
    copy /Y "%SOURCE_EXE%" "%TARGET_DIR%\project-3\cmd\Participant\Participant.exe" >nul
    if %errorlevel% equ 0 (
        echo [成功] 已复制到 project-3
    ) else (
        echo [错误] 复制到 project-3 失败
    )
) else (
    echo [警告] project-3 目标目录不存在，跳过
)

echo.
echo ========================================
echo 构建和部署完成！
echo ========================================
echo.
echo 现在你可以启动三个参与方进行测试了：
echo 1. 进入 Participants\project-1\cmd\Participant\ 并运行 Participant.exe
echo 2. 进入 Participants\project-2\cmd\Participant\ 并运行 Participant.exe  
echo 3. 进入 Participants\project-3\cmd\Participant\ 并运行 Participant.exe
echo.
echo 提示：你也可以直接运行 copy_participants.bat 来只复制文件（不重新编译）
echo.
pause 