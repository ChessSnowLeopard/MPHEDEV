import numpy as np
import torch
import matplotlib.pyplot as plt

# 定义Sigmoid函数
def sigmoid(x):
    return 1 / (1 + np.exp(-x))

# 最小二乘多项式逼近函数
def least_squares_approximation(func, interval=(-8, 8), degree=3, num_points=1000):
    # 生成标准化的输入点（x/8缩放）
    x_scaled = np.linspace(-1, 1, num_points)  # 先将区间映射到[-1,1]
    x = x_scaled * (interval[1] - interval[0])/2 + np.mean(interval)  # 反标准化到目标区间
    
    # 计算目标函数值
    y = func(x)
    
    # 构建多项式特征矩阵（包含x/8的奇次幂）
    X_poly = np.zeros((num_points, (degree+1)//2))
    for i in range(0, degree+1, 2):
        X_poly[:, i//2] = (x/interval[1])**i  # 论文中的奇次幂结构
    
    # 添加常数项
    X_poly = np.hstack([np.ones((num_points, 1)), X_poly])
    
    # 求解最小二乘系数
    coeffs = np.linalg.lstsq(X_poly, y, rcond=None)[0]
    
    # 构建多项式函数
    def poly_func(x_input):
        x_norm = (x_input - np.mean(interval)) / (interval[1] - interval[0])*2  # 标准化到[-1,1]
        X_poly_input = np.zeros((x_input.shape[0], (degree+1)//2))
        for i in range(0, degree+1, 2):
            X_poly_input[:, i//2] = x_norm**i
        X_poly_input = np.hstack([np.ones((x_input.shape[0], 1)), X_poly_input])
        return X_poly_input.dot(coeffs)
    
    return poly_func, coeffs

# 测试用例
if __name__ == "__main__":
    # 生成逼近函数（3次和7次多项式）
    poly3, coeffs3 = least_squares_approximation(sigmoid, degree=3)
    poly7, coeffs7 = least_squares_approximation(sigmoid, degree=7)
    
    # 打印系数
    print("3次多项式系数：", coeffs3)
    print("7次多项式系数：", coeffs7)
    
    # 可视化对比
    x_test = np.linspace(-10, 10, 1000)
    plt.figure(figsize=(10,6))
    
    plt.plot(x_test, sigmoid(x_test), label='Original Sigmoid')
    plt.plot(x_test, poly3(x_test), '--', label='Degree 3 Approximation')
    plt.plot(x_test, poly7(x_test), '--', label='Degree 7 Approximation')
    
    plt.xlim(-10, 10)
    plt.ylim(-0.1, 1.1)
    plt.axvline(x=-8, color='gray', linestyle='--')
    plt.axvline(x=8, color='gray', linestyle='--')
    plt.legend()
    plt.title("Least Squares Polynomial Approximation of Sigmoid")
    plt.show()