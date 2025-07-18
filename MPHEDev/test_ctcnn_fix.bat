@echo off
echo ========================================
echo 测试ctCNN数据集路径修复
echo ========================================

echo.
echo 当前工作目录:
cd
echo.

echo 切换到MPHEDev根目录...
cd /d "%~dp0"

echo 当前工作目录:
cd
echo.

echo 检查数据集文件是否存在:
if exist "pkg\training\data\train-images-idx3-ubyte.gz" (
    echo ✓ 训练图像文件存在
) else (
    echo ❌ 训练图像文件不存在
)

if exist "pkg\training\data\train-labels-idx1-ubyte.gz" (
    echo ✓ 训练标签文件存在
) else (
    echo ❌ 训练标签文件不存在
)

if exist "pkg\training\data\t10k-images-idx3-ubyte.gz" (
    echo ✓ 测试图像文件存在
) else (
    echo ❌ 测试图像文件不存在
)

if exist "pkg\training\data\t10k-labels-idx1-ubyte.gz" (
    echo ✓ 测试标签文件存在
) else (
    echo ❌ 测试标签文件不存在
)

echo.
echo 检查ctCNN.exe是否存在:
if exist "cmd\ctCNN\ctCNN.exe" (
    echo ✓ ctCNN.exe存在
) else (
    echo ❌ ctCNN.exe不存在
)

echo.
echo ========================================
echo 测试完成
echo ========================================
pause 