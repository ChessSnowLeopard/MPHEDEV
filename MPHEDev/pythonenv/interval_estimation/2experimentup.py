import numpy as np
import torch
import torch.nn as nn
import copy
import torch.nn.functional as F

# 导入其他文件里的包示例：
# from pack import add
# b = 

# 计算函数输出
def evaluate_function(func, data):
    """ 给定函数 func 和数据 data，返回 func(data) 的输出 """
    return func(data)

# 计算矩
def compute_moments(data, order=4):
    """ 计算数据的矩：包括均值、方差、以及更高阶的矩 """
    moments = [np.mean(data ** i) for i in range(1, order + 1)]
    return moments


# 生成伪输入分布 D，使得其矩与 E[f(xi)] 相同
def generate_fake_distribution(moments, num_samples=100, target_range=(0, 1), target_mean=None):
    mean = target_mean if target_mean is not None else moments[0]  
    var = moments[1]  # 使用当前计算的方差
    fake_samples = np.random.normal(loc=mean, scale=np.sqrt(var), size=num_samples)

    # 将生成的样本归一化到目标范围 (0, 1)
    fake_samples = np.clip(fake_samples, target_range[0], target_range[1])
    return fake_samples


# 调整g区间的逼近方式：使用切比雪夫不等式计算区间约束，还没写马可夫
# def chebyshev_inequality(moments, target_mean, target_std):

#     k = moments[1]  # 方差
#     return (target_mean - 2 * np.sqrt(k), target_mean + 2 * np.sqrt(k))


# 近似函数：多项式拟合
def approximate_function(activation, input_min, input_max, num_points=100, degree=5):
    """ 用多项式逼近激活函数（需传入输入范围的最小值和最大值）"""
    # 生成区间内的连续点（例如从 input_min 到 input_max 生成 100 个点）
    input_points = torch.linspace(input_min, input_max, num_points).unsqueeze(1)  # 形状 (100, 1)
    activation_values = activation(input_points)  # 计算激活函数输出

    # 多项式拟合（需将输入输出展平为一维数组）
    poly_coefs = np.polyfit(
        input_points.detach().numpy().flatten(), 
        activation_values.detach().numpy().flatten(), 
        degree
    )
    poly_approx = np.poly1d(poly_coefs)
    return poly_approx

    # 假设拟合切比雪夫多项式，这里使用一个简单的多项式拟合（好像暂时用不到，先注释掉）
    # 用切比雪夫多项式来拟合目标激活函数，这里先使用简单的多项式因为我真的写晕了（可以替换为切比雪夫多项式拟合方法）
    # poly_coefs = np.polyfit(input_range, activation(input_range), degree)
    # poly_approx = np.poly1d(poly_coefs)
    # return poly_approx


# 定义神经网络结构
class ApproxNN(nn.Module):
    def __init__(self, input_size, hidden_sizes, output_size):
        super(ApproxNN, self).__init__()
        self.layers = nn.ModuleList()
        self.activations = []
        
        prev_size = input_size
        for hidden_size in hidden_sizes:
            self.layers.append(nn.Linear(prev_size, hidden_size))
            self.activations.append(torch.relu)  # 初始激活函数是 ReLU
            prev_size = hidden_size
        
        self.layers.append(nn.Linear(prev_size, output_size))
    
    def forward(self, x):
        for i, layer in enumerate(self.layers[:-1]):
            x = layer(x)
            x = self.activations[i](x)
        return self.layers[-1](x)

# 创建神经网络模型
input_size = 1
hidden_sizes = [10, 10, 10]  # 3层隐藏层
output_size = 1
model = ApproxNN(input_size, hidden_sizes, output_size)
criterion = nn.MSELoss()  # 定义均方误差损失
optimizer = torch.optim.Adam(model.parameters(), lr=0.01)  # 定义优化器


#实现训练过程
#model(...)
#dataloader = ...
#optimizer = ...
#criterion = ...



# 假设输入数据服从标准正态分布
np.random.seed(42)
torch.manual_seed(42)
X = np.random.normal(0, 1, (1000, 1))  # 输入数据

# 假设的非多项式函数 g：g(x) = sin(x)
def non_polynomial_function(x):
    return np.sin(x)

#【已解决】f function initial value
# 计算初始的 f、期望、方差
def compute_initial_f(X):
    
    E_Xi = np.mean(X, axis=0)  # 均值 
    Var_Xi = np.var(X, axis=0)  # 方差
    
    print("初始 f 计算完成：")
    print(f"  - E[Xi] (期望): {E_Xi}")
    print(f"  - Var[Xi] (方差): {Var_Xi}")
    
    return E_Xi, Var_Xi

def process_g_with_approximation(X, max_iterations=5):

    # 先计算初始 f
    E_Xi, Var_Xi = compute_initial_f(X)

    # 将原始的二维输入保留
    current_input_g = torch.tensor(X, dtype=torch.float32)
    current_input_g1 = torch.tensor(X, dtype=torch.float32).unsqueeze(1)  # 确保输入形状为 (1000, 1)
    
    # 交替进行f和g的循环
    for i in range(max_iterations):
        # g 循环：g, g1, g2, g3...
        print(f"----- g{i+1} -----")
        
        # 使用上一次的伪输入分布 D_f
        D_g = X  # 假设 D_g 是之前的输入分布 
        # 采样并确保形状是 (100, 1)
        D_samples_g = np.random.choice(D_g.flatten(), 100).reshape(-1, 1)  
        print(f"g{i+1} 采样结果（原始）：{D_samples_g[:10]}")

        # -------------------- 对采样的 x 进行选择性缩放 --------------------
        #【方便缩放回原来状态】加入一步：记录缩放因子
        scaling_factor = np.ones_like(D_samples_g) 

        if np.all(D_samples_g >= 0) and np.all(D_samples_g <= 1):
            print(f"g{i+1} 采样值在[0, 1]区间，无需缩放")
            D_samples_g_scaled = D_samples_g  # 无需缩放
        else:
            print(f"g{i+1} 采样值不在[0, 1]区间，需要缩放")
            if np.max(D_samples_g) > 1:
                D_samples_g_scaled = D_samples_g / 100
            elif np.min(D_samples_g) < 0:
                D_samples_g_scaled = D_samples_g * 100

        original_shape = D_samples_g.shape  # 记录原始的形状

        D_samples_g_scaled = D_samples_g * scaling_factor  # 应用缩放因子
        print(f"g{i+1} 采样结果（缩放后）：{D_samples_g_scaled[:10]}")

        # 计算 g(d) 输入，得到一组输出 y
        current_output_g = evaluate_function(non_polynomial_function, D_samples_g_scaled)
        print(f"g{i+1} 输出：{current_output_g[:10]}")

        # 根据 g 的输出 y 来生成新的伪输入分布 D'
        moments_g = compute_moments(current_output_g)
        
        # 生成伪输入分布，调用 generate_fake_distribution，确保均值匹配 E[f(xi)]
        D_prime = generate_fake_distribution(moments_g, num_samples=100, target_mean=E_Xi)
        print(f"伪输入分布 D_prime：{D_prime[:10]}")

        # 恢复 x 到原始尺度
        D_samples_g_recovered = D_samples_g_scaled / scaling_factor
        print(f"g{i+1} 采样结果（恢复后）：{D_samples_g_recovered[:10]}") 

        # [采样的]将一维数组恢复为原始的二维形状
        D_samples_g_recovered_reshaped = D_samples_g_recovered.reshape(original_shape)
        print(f"g{i+1} 采样结果（恢复为二维）：{D_samples_g_recovered_reshaped[:10]}")
        
        
        # 【输出的】将一维数组恢复为二维列向量
        # 可以找一个更好的方式还原成二维，或者直接保持原维度的状态
        current_output_reshaped = current_output_g.reshape(-1, 1)  # reshape 为列向量
        
        # 打印恢复后的前10个元素
        print(f"g{i+1} 采样结果（恢复为二维）：{current_output_reshaped[:10]}")

        # 将原始二维数组 (1000, 1) 直接用于计算
        current_input_g1 = torch.tensor(D_prime, dtype=torch.float32).unsqueeze(1)  # 保持原始的二维形状
         
        # 【切比雪夫】重新估算区间，暂时用不到所以注释了
        # target_mean = np.mean(current_output_g)
        # target_std = np.std(current_output_g)
        # estimated_range = chebyshev_inequality(moments_g, target_mean, target_std)
        # print(f"使用切比雪夫不等式估算的区间：{estimated_range}")

        # ----------------------- 创建和替换神经网络的激活函数 ------------------------
        model_p = copy.deepcopy(model)  # 改造版的模型

# (1,10)
# (10,10)
# (10,10)
# (10,1)

# in (100,1)

# in*(1,10) = (100,10)
# # in*(10,10)
# in*(1,10)*(10,10)
        # temp = [p1, p2, p3]


        for j, layer in enumerate(model_p.layers[:-1]):  # 遍历所有隐藏层
            # current_output_reshaped_ = torch.tensor(current_output_reshaped, dtype=torch.float32).unsqueeze(2)
            # current_output_reshaped_10x10 = current_output_reshaped_.reshape(100, )
            # current_output = layer(current_output_reshaped_10x10) # 使用原始二维输入
            # current_output_np = current_output.detach().numpy()
            current_output_reshaped = layer(torch.tensor(current_output_reshaped, dtype=torch.float32))
            current_output_np = current_output_reshaped.detach().numpy()
            # temp.append(current_output_np.copy()) # deepcopy

            # ********原代码（错误）：
# input_range = (np.min(current_output_np), np.max(current_output_np))

# 修改为：
            input_min = np.min(current_output_np)  # 定义最小值
            input_max = np.max(current_output_np)  # 定义最大值

            # 拟合 ReLU，得到多项式近似
            #poly_approx = approximate_function(torch.relu, input_range, degree=5)
            poly_approx = approximate_function(torch.relu, input_min, input_max, degree=5)  # 传入标量范围
            
            # 生成输入点张量（形状 (100, 1)）
            input_points = torch.linspace(input_min, input_max, 100).unsqueeze(1)
            out = model(input_points) 
            #*****************
            poly_values = torch.tensor(poly_approx(input_points.detach().numpy().flatten()), dtype=torch.float32).unsqueeze(1)
            loss = criterion(out, poly_values)
            optimizer.zero_grad()
            loss.backward()
            optimizer.step()

            # 用多项式替换 ReLU
            model_p.activations[j] = lambda x, poly=poly_approx: torch.tensor(poly(x.detach().numpy()), dtype=torch.float32)

        # 更新输入数据为当前的输出，进入下一次迭代
        current_input_g = torch.tensor(D_prime, dtype=torch.float32)

        # f 循环：f1, f2, ...
        print(f"----- f{i+1} -----")
        #current_output_f = evaluate_function(poly_function, current_input_f.numpy())  # 计算 f
        current_output_f = evaluate_function(poly_approx, current_input_f.numpy())
        moments_f = compute_moments(current_output_f)
        print(f"f{i+1} 输出的矩: {moments_f}")

        D_f = generate_fake_distribution(moments_f, num_samples=100)
        distributions_f.append(D_f)  # 存储伪输入分布
        print(f"伪输入分布 D_f：{D_f[:10]}")  # 打印前10个样本
        current_input_f = torch.tensor(current_output_f, dtype=torch.float32)

    return distributions_f, distributions_g
    
# 执行 f 和 g 的交替循环处理
distributions_f, distributions_g = process_g_with_approximation(X, max_iterations=5)

...


#目前存在的问题是：采样需要把二维数组展开成一维，但是展开之后再输出，输出的还是一维数组的形式，没办法和第二个里面的相乘，怎么转回二维数组？    

# 调用函数
process_g_with_approximation(X)