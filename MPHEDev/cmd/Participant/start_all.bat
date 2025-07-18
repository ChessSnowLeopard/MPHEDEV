@echo off
chcp 65001 >nul
echo ========================================
echo 参与方程序一键启动脚本
echo ========================================

:: 获取当前脚本的绝对路径
set "SCRIPT_DIR=%~dp0"
echo [调试] 脚本目录: %SCRIPT_DIR%

:: 切换到脚本所在目录
cd /d "%SCRIPT_DIR%"
echo [调试] 切换到脚本目录: %CD%

:: 定义参与方路径（使用绝对路径）
set "TARGET_DIR=%CD%\..\..\..\Participants"
echo [调试] 目标目录: %TARGET_DIR%

:: 检查目标目录是否存在
if not exist "%TARGET_DIR%" (
    echo [错误] 目标目录 %TARGET_DIR% 不存在
    echo 请确保Participants文件夹在正确位置
    pause
    exit /b 1
)

echo [步骤1] 检查参与方程序是否存在...
echo.

:: 检查project-1
if not exist "%TARGET_DIR%\project-1\cmd\Participant\Participant.exe" (
    echo [错误] project-1 的 Participant.exe 不存在
    echo 请先运行 build_and_deploy.bat 编译并部署程序
    pause
    exit /b 1
)

:: 检查project-2
if not exist "%TARGET_DIR%\project-2\cmd\Participant\Participant.exe" (
    echo [错误] project-2 的 Participant.exe 不存在
    echo 请先运行 build_and_deploy.bat 编译并部署程序
    pause
    exit /b 1
)

:: 检查project-3
if not exist "%TARGET_DIR%\project-3\cmd\Participant\Participant.exe" (
    echo [错误] project-3 的 Participant.exe 不存在
    echo 请先运行 build_and_deploy.bat 编译并部署程序
    pause
    exit /b 1
)

echo [成功] 所有参与方程序检查通过
echo.

echo [步骤2] 开始启动三个参与方...
echo.

:: 启动project-1
echo [启动] 启动参与方 1 (project-1)...
echo [调试] 当前目录: %CD%
set "PROJECT1_DIR=%TARGET_DIR%\project-1\cmd\Participant"
echo [调试] 切换到: %PROJECT1_DIR%
cd /d "%PROJECT1_DIR%"
echo [调试] 切换后目录: %CD%
start /d "%CD%" "参与方 1" "Participant.exe"
echo [调试] 已启动参与方 1

:: 等待2秒
timeout /t 2 /nobreak >nul

:: 启动project-2
echo [启动] 启动参与方 2 (project-2)...
echo [调试] 当前目录: %CD%
set "PROJECT2_DIR=%TARGET_DIR%\project-2\cmd\Participant"
echo [调试] 切换到: %PROJECT2_DIR%
cd /d "%PROJECT2_DIR%"
echo [调试] 切换后目录: %CD%
start /d "%CD%" "参与方 2" "Participant.exe"
echo [调试] 已启动参与方 2

:: 等待2秒
timeout /t 2 /nobreak >nul

:: 启动project-3
echo [启动] 启动参与方 3 (project-3)...
echo [调试] 当前目录: %CD%
set "PROJECT3_DIR=%TARGET_DIR%\project-3\cmd\Participant"
echo [调试] 切换到: %PROJECT3_DIR%
cd /d "%PROJECT3_DIR%"
echo [调试] 切换后目录: %CD%
start /d "%CD%" "参与方 3" "Participant.exe"
echo [调试] 已启动参与方 3

:: 回到原始目录
cd /d "%SCRIPT_DIR%"

echo.
echo ========================================
echo 启动完成！
echo ========================================
echo.
echo 三个参与方窗口已打开，请按以下顺序操作：
echo 1. 在参与方 1 窗口中输入参与方ID: 1
echo 2. 在参与方 2 窗口中输入参与方ID: 2  
echo 3. 在参与方 3 窗口中输入参与方ID: 3
echo.
echo 注意：确保协调器已经启动并运行在 http://localhost:8080
echo.
pause 