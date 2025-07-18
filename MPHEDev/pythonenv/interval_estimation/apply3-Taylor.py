import numpy as np
import torch
import scipy
# 逆平方根func
def inverse_sqrt(x):
    return 1 / np.sqrt(x)
def numerical_derivative(func, x, h=1e-6):
    return (func(x + h) - func(x - h)) / (2 * h)

# taylor_approximation
def taylor_approximation(func, center=1.0, degree=3):
    coefficients = []
    for i in range(degree+1):
        coeff = numerical_derivative(func, center, h=1e-6)
        coefficients.append(coeff)
    return np.poly1d(coefficients[::-1])  # 降幂

# test
X = np.linspace(0.1, 5, 1000)  # 避免0附近不收敛
y_true = inverse_sqrt(X)

# 计算逼近
taylor_poly = taylor_approximation(inverse_sqrt, center=2.0, degree=3)
y_taylor = taylor_poly(X)

# 误差
mse = np.mean((y_true - y_taylor)**2)
print(f"泰勒展开逼近 MSE: {mse:.6f}")

# 可视化
import matplotlib.pyplot as plt
plt.figure()
plt.plot(X, y_true, label='True')
plt.plot(X, y_taylor, '--', label='Taylor')
plt.legend()
plt.title("Taylor Approximation of 1/√x")
plt.show()