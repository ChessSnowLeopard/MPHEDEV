import numpy as np
import torch
from scipy.optimize import minimize

# 定义逆平方根函数
def inverse_sqrt(x):
    return 1 / np.sqrt(x)

# Remez算法简化实现（目标是最小化最大误差）
def remez_rational(x, y_true, num_degree=3, den_degree=2):
    """ 优化分子和分母多项式系数 """
    def error_func(coeffs, x, y_true):
        num = np.poly1d(coeffs[:num_degree+1])
        den = np.poly1d(coeffs[num_degree+1:])
        y_pred = num(x) / den(x)
        return np.max(np.abs(y_pred - y_true))
    
    # 初始猜测（分子为泰勒展开，分母为1）
    initial_coeffs = np.concatenate([
        np.polyfit(x, y_true, num_degree),
        [1] + [0]*den_degree
    ])
    
    # 最小化最大误差
    result = minimize(error_func, initial_coeffs, args=(x, y_true), method='Nelder-Mead')
    return (
        np.poly1d(result.x[:num_degree+1]),
        np.poly1d(result.x[num_degree+1:])
    )

# 生成测试数据
X = np.linspace(0.1, 5, 1000)
y_true = inverse_sqrt(X)

# 计算有理函数逼近
num_poly, den_poly = remez_rational(X, y_true, num_degree=3, den_degree=1)
y_rational = num_poly(X) / den_poly(X)

# 评估误差
mse = np.mean((y_true - y_rational)**2)
print(f"有理函数逼近 MSE: {mse:.6f}")

# 可视化结果
import matplotlib.pyplot as plt
plt.figure()
plt.plot(X, y_true, label='True')
plt.plot(X, y_rational, '--', label='Rational')
plt.legend()
plt.title("Rational Approximation of 1/√x")
plt.show()