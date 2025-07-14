import torch
import torch.nn as nn
import numpy as np
import copy
# import matplotlib.pyplot as plt
from numpy.polynomial.chebyshev import Chebyshev

# 定义神经网络结构
class ApproxNN(nn.Module):
    def __init__(self, input_size, hidden_sizes, output_size):
        super(ApproxNN, self).__init__()
        self.layers = nn.ModuleList()
        self.activations = []
        
        prev_size = input_size
        for hidden_size in hidden_sizes:
            self.layers.append(nn.Linear(prev_size, hidden_size))
            self.activations.append(torch.sigmoid)  # 设定非线性激活函数 # torch.relu
            prev_size = hidden_size
        
        self.layers.append(nn.Linear(prev_size, output_size))
    
    def forward(self, x):
        for i, layer in enumerate(self.layers[:-1]):
            x = layer(x)
            x = self.activations[i](x)
        return self.layers[-1](x)

# 计算某一层输出的矩估计
def compute_moments(data, order=4):
    moments = [np.mean(data ** i) for i in range(1, order + 1)]
    return moments

# 估计输入区间
def estimate_input_range(data, confidence=0.95):
    mean = np.mean(data,axis=0)
    std = np.std(data,axis=0)
    lower_bound = mean - 3 * std  # 近似95%置信区间 (may be we can do better here by using more moments instead of only mean and std.)
    upper_bound = mean + 3 * std
    return (lower_bound, upper_bound)

# 逼近非多项式函数 g 使用切比雪夫多项式
def approximate_function(g, input_range, degree=5):
    x = torch.tensor(np.linspace(input_range[0], input_range[1], 100))
    y = g(x).numpy()
    coefs = Chebyshev.fit(x, y, degree).coef
    return Chebyshev(coefs)


# Approximation configuration
degree = 5

# 生成数据集 X
np.random.seed(42)
torch.manual_seed(42)
X = np.random.normal(0, 1, (1000, 1))  # 假设输入数据服从标准正态分布

# 训练神经网络
input_size = 1
hidden_sizes = [10, 10, 10]  # 3层隐藏层
output_size = 1
model = ApproxNN(input_size, hidden_sizes, output_size)

# 近似非多项式激活函数

# Create two set of identical input, one for approximate computation, one for real/exact computation.
current_input = torch.tensor(X, dtype=torch.float32)
current_input_real = current_input.clone()

# Store polynomials for subsequent usage.
poly_approximations = []

for i, (layer, activation) in enumerate(zip(model.layers[:-1], model.activations)):

    # Compute "f" on approximate input:
    current_output = layer(current_input)
    current_output_np = current_output.detach().numpy()

    # Compute "f" on real input:
    current_output_real = layer(current_input_real)
    current_output_real_np = current_output_real.detach().numpy()
    

    # 计算通过矩得到的输入区间
    input_range = estimate_input_range(current_output_np)
    input_range = np.vstack(input_range)
    input_range_real = estimate_input_range(current_output_real_np)
    input_range_real = np.vstack(input_range_real)
    # Check the outliers:
    '''
    for j in range(current_output_np.shape[0]):
        for k in range(current_output_np.shape[1]):
            if current_output_np[j][k] > input_range[1] or current_output_np[j][k] < input_range[0]:
                print("outliar at ("+str(j)+","+str(k)+"):"+str(current_output_np[j][k]))
    '''
    
    # 计算通过 min/max 直接得到的输入区间
    input_range_minmax = np.vstack((np.min(current_output_np,axis=0), np.max(current_output_np,axis=0)))
    input_range_minmax_real = np.vstack((np.min(current_output_real_np,axis=0),np.max(current_output_real_np,axis=0)))

    print("input_range_moments:")
    print(input_range)
    print("input_range_minmax:")
    print(input_range_minmax)

    print("input_range_moments_real:")
    print(input_range_real)
    print("input_range_minmax_real:")
    print(input_range_minmax_real)
    
    
    # Construct approximate poly for "g"
    
    # Store one layer of "p_g"s:
    P_gs = []

    for j in range(input_range[0].shape[0]):
        poly_approx = approximate_function(activation, (input_range[0][j],input_range[1][j]),degree=degree)
        P_gs.append(poly_approx)

    poly_approximations.append(P_gs)

    # Compute approximated "g" on approximate output
    y_approx = copy.deepcopy(current_output_np)
    for j,poly_approx in enumerate(P_gs):
        y_approx[:,j] = poly_approx(y_approx[:,j])
    y_approx = torch.tensor(y_approx)

    
    
    # Compute real "g" on real output
    y_real = activation(current_output_real)

    y_approx_range_minmax = np.vstack((np.min(y_approx.detach().numpy(),axis=0),np.max(y_approx.detach().numpy(),axis=0)))
    y_real_range_minmax = np.vstack((np.min(y_real.detach().numpy(),axis=0),np.max(y_real.detach().numpy(),axis=0)))

    print("y_approx:")
    print(y_approx)
    print("y_real:")
    print(y_real)

    print("y_approx_range_minmax:")
    print(y_approx_range_minmax)
    print("y_real_range_minmax:")
    print(y_real_range_minmax)

    # Let input be output for next iteration. 
    current_input = y_approx
    current_input_real = y_real

