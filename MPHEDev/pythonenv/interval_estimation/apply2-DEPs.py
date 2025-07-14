import numpy as np
import torch
import torch.nn as nn
import copy

# 定义DEP生成函数 (三次多项式)
def generate_dep(scale_factor=1.0):
    return np.poly1d([-4/(27*scale_factor**2), 0, 1, 0])  # -ax³ + x

# 改进的多项式逼近函数（支持DEP扩展）
def dep_approximate(func, init_range, num_extensions=3, degree=3, num_points=100):
    current_range = init_range
    composite_poly = np.poly1d([0])  # 初始化为0多项式
    
    # 生成初始区间逼近
    x = np.linspace(current_range[0], current_range[1], num_points)
    y = func(x)
    base_poly = np.polyfit(x, y, deg=degree)
    composite_poly = np.poly1d(base_poly)
    
    # DEP迭代扩展
    for _ in range(num_extensions):
        # 计算当前缩放因子
        scale_factor = (current_range[1] - current_range[0])/2
        
        # 生成DEP并复合
        dep_poly = generate_dep(scale_factor)
        composite_poly = np.poly1d(np.polymul(composite_poly.coeffs, dep_poly.coeffs))
        
        # 扩展输入区间
        current_range = (current_range[0]*2, current_range[1]*2)
        
        # 在扩展区间上重新拟合
        x = np.linspace(current_range[0], current_range[1], num_points)
        y = func(x) - composite_poly(x)  # 拟合残差
        residual_poly = np.polyfit(x, y, deg=degree)
        
        # 叠加残差多项式
        composite_poly = np.poly1d(np.polyadd(composite_poly, residual_poly))
    
    return composite_poly

# 修改后的近似函数
def approximate_function(activation, input_min, input_max, num_points=100, degree=3):
    """DEP增强型多项式逼近"""
    # 使用DEP方法进行逼近
    dep_poly = dep_approximate(
        lambda x: activation(torch.tensor(x)).numpy(),
        init_range=(input_min, input_max),
        num_extensions=2,
        degree=degree
    )
    return dep_poly

# 修改神经网络激活函数替换过程
def replace_activations_with_dep(model, current_output_np):
    #用DEP多项式替换ReLU激活函数
    model_p = copy.deepcopy(model)
    
    input_min = np.min(current_output_np)
    input_max = np.max(current_output_np)
    
    # 使用DEP方法进行多项式逼近
    poly_approx = approximate_function(
        torch.relu,
        input_min,
        input_max,
        degree=3
    )
    
    # 定义多项式激活层
    def poly_activation(x):
        x_np = x.detach().numpy()
        return torch.tensor(poly_approx(x_np), dtype=torch.float32)
    
    # 替换所有隐藏层激活函数
    for j in range(len(model_p.activations)):
        model_p.activations[j] = poly_activation
        
    return model_p

# process_g_with_approximation函数替换部分
# model_p.activations[j] = lambda x, poly=poly_approx: torch.tensor(poly(x.detach().numpy()), dtype=torch.float32)

#model_p = replace_activations_with_dep(model, current_output_np)

# test
if __name__ == "__main__":
    # test by same function
    # sin(x)
    test_func = lambda x: np.sin(x)
    
    # initial range
    init_range = (-np.pi/2, np.pi/2)
    
    # dep
    final_poly = dep_approximate(test_func, init_range, num_extensions=3)
    
    # 评估效果
    test_x = np.linspace(-4*np.pi, 4*np.pi, 1000)
    true_y = test_func(test_x)
    approx_y = final_poly(test_x)
    
    # 计算最大误差
    max_error = np.max(np.abs(true_y - approx_y))
    print(f"最大逼近误差: {max_error:.4f}")
    
    
    
    # [new add]可视化结果
    import matplotlib.pyplot as plt
    plt.figure(figsize=(10, 6))
    plt.plot(test_x, true_y, label="True Function")
    plt.plot(test_x, approx_y, label="DEP Approximation")
    plt.fill_between(test_x, true_y, approx_y, alpha=0.3)
    plt.title("DEP Polynomial Approximation")
    plt.legend()
    plt.show()