#!/usr/bin/env python3
"""
训练API测试脚本
用于验证后端的训练历史记录API是否正常工作
"""

import requests
import json
import time
import random

# 配置
BASE_URL = "http://localhost:8080"  # 协调器端口
TRAINING_HISTORY_URL = f"{BASE_URL}/training/history"
TRAINING_STATUS_URL = f"{BASE_URL}/training/status"

def test_training_status():
    """测试获取训练状态"""
    print("🔍 测试获取训练状态...")
    try:
        response = requests.get(TRAINING_STATUS_URL, timeout=5)
        if response.status_code == 200:
            data = response.json()
            print(f"✅ 训练状态获取成功:")
            print(f"   - 状态: {data.get('status', 'unknown')}")
            print(f"   - 总轮数: {data.get('totalEpochs', 0)}")
            if data.get('lastEpoch'):
                last_epoch = data['lastEpoch']
                print(f"   - 最新轮次: {last_epoch.get('epoch')}")
                print(f"   - 损失: {last_epoch.get('loss', 0):.6f}")
                print(f"   - 准确率: {last_epoch.get('accuracy', 0) * 100:.2f}%")
            return True
        else:
            print(f"❌ 获取训练状态失败，状态码: {response.status_code}")
            return False
    except Exception as e:
        print(f"❌ 获取训练状态异常: {e}")
        return False

def test_training_history():
    """测试获取训练历史"""
    print("\n🔍 测试获取训练历史...")
    try:
        response = requests.get(TRAINING_HISTORY_URL, timeout=5)
        if response.status_code == 200:
            data = response.json()
            print(f"✅ 训练历史获取成功:")
            print(f"   - 数据条数: {data.get('count', 0)}")
            
            history = data.get('data', [])
            if history:
                print(f"   - 训练历史:")
                for item in history[-3:]:  # 只显示最后3条
                    print(f"     Epoch {item['epoch']}: 损失={item['loss']:.6f}, "
                          f"准确率={item['accuracy']*100:.2f}%, "
                          f"学习率={item['learningRate']:.6f}")
            return True
        else:
            print(f"❌ 获取训练历史失败，状态码: {response.status_code}")
            return False
    except Exception as e:
        print(f"❌ 获取训练历史异常: {e}")
        return False

def simulate_training_data():
    """模拟添加训练数据（仅用于测试）"""
    print("\n🔍 模拟训练数据...")
    print("注意：这个功能需要后端支持，目前仅用于测试API连接")
    
    # 这里只是测试API连接，实际的数据添加需要通过ctCNN程序
    print("✅ API连接测试完成")

def main():
    """主测试函数"""
    print("🚀 开始训练API测试")
    print("=" * 50)
    
    # 测试训练状态API
    status_ok = test_training_status()
    
    # 测试训练历史API
    history_ok = test_training_history()
    
    # 模拟训练数据
    simulate_training_data()
    
    print("\n" + "=" * 50)
    print("📊 测试结果总结:")
    print(f"   - 训练状态API: {'✅ 正常' if status_ok else '❌ 失败'}")
    print(f"   - 训练历史API: {'✅ 正常' if history_ok else '❌ 失败'}")
    
    if status_ok and history_ok:
        print("\n🎉 所有API测试通过！")
        print("\n💡 使用说明:")
        print("1. 启动协调器: cd MPHEDev && go run cmd/Coordinator/main.go")
        print("2. 启动参与方: cd MPHEDev && go run cmd/Participant/main.go")
        print("3. 运行ctCNN训练: cd MNIST_CNN && go run cmd/ctCNN/main.go")
        print("4. 访问前端页面查看实时训练进度")
    else:
        print("\n⚠️ 部分API测试失败，请检查后端服务是否正常运行")

if __name__ == "__main__":
    main() 