# 数据集目录

本目录包含项目所需的所有数据集文件。

## 目录结构

```
data/
├── horizontal/         # 水平划分数据集
│   ├── train_split_000_images.csv
│   ├── train_split_000_labels.csv
│   ├── train_split_001_images.csv
│   ├── train_split_001_labels.csv
│   └── ...
├── vertical/           # 垂直划分数据集
│   ├── train_split_000_images.csv
│   ├── train_split_000_labels.csv
│   └── ...
├── raw/                # 原始MNIST数据集
│   ├── train-images-idx3-ubyte.gz
│   ├── train-labels-idx1-ubyte.gz
│   ├── t10k-images-idx3-ubyte.gz
│   └── t10k-labels-idx1-ubyte.gz
└── README.md           # 本文件
```

## 数据集说明

### 水平划分数据集 (horizontal/)

- **用途**：按样本划分，每个参与方持有部分样本的完整特征
- **格式**：CSV文件，每行一个样本
- **文件命名**：`train_split_XXX_images.csv` 和 `train_split_XXX_labels.csv`
- **XXX**：3位数字，从000开始

### 垂直划分数据集 (vertical/)

- **用途**：按特征划分，每个参与方持有所有样本的部分特征
- **格式**：CSV文件，每行一个样本
- **文件命名**：`train_split_XXX_images.csv` 和 `train_split_XXX_labels.csv`
- **XXX**：3位数字，从000开始

### 原始数据集 (raw/)

- **用途**：原始MNIST数据集，用于生成划分后的数据集
- **格式**：IDX格式的压缩文件
- **来源**：MNIST官方数据集

## 生成数据集

### 使用Python脚本生成

```bash
cd test/dataPartial
python mnist_splitter.py --num_splits 5 --format csv
```

### 手动生成

1. 将原始MNIST文件放入 `data/raw/` 目录
2. 运行分割脚本
3. 将生成的文件移动到对应的 `horizontal/` 或 `vertical/` 目录

## 数据格式

### 图像数据 (XXX_images.csv)

- 每行代表一个样本
- 每列代表一个像素值
- 像素值范围：0-255
- 图像尺寸：28x28 = 784个像素

### 标签数据 (XXX_labels.csv)

- 每行代表一个样本的标签
- 标签值范围：0-9（对应数字0-9）
- 与图像数据一一对应

## 参与方配置

每个参与方需要配置对应的数据分片：

```go
config := &dataset.Config{
    DataDir:       "data",
    SplitType:     "horizontal",  // 或 "vertical"
    SplitID:       "train_split_000",
    ParticipantID: 0,
}
```

## 注意事项

1. 确保数据文件格式正确
2. 图像和标签文件必须成对存在
3. 参与方ID与数据分片ID的对应关系由协调器管理
4. 数据文件较大，建议使用版本控制忽略（.gitignore）

## 版本控制

建议在 `.gitignore` 中添加：

```
# 忽略大型数据文件
data/horizontal/*.csv
data/vertical/*.csv
data/raw/*.gz
```

只保留元数据文件：

```
# 保留元数据
!data/horizontal/train_metadata.json
!data/vertical/train_metadata.json
``` 