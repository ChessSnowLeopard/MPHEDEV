#!/usr/bin/env python3
"""
MNIST数据集分割脚本
支持按样本(水平)或特征(竖直)拆分MNIST数据集，便于分布式训练使用
数据路径: ./raw/
输出路径: ./horizontal/ 和 ./vertical/
"""

import argparse
import gzip
import struct
import numpy as np
import os
import json
import csv
import shutil
from pathlib import Path
from typing import Tuple, List, Dict, Any

class MNISTSplitter:
    def __init__(self):
        self.train_images = None
        self.train_labels = None
        self.test_images = None
        self.test_labels = None
        # 固定数据路径
        self.data_dir = Path("./raw")
        
    def load_mnist_images(self, filepath: Path) -> np.ndarray:
        """加载MNIST图像数据"""
        with gzip.open(filepath, 'rb') as f:
            # 读取文件头
            magic, num_images, rows, cols = struct.unpack('>IIII', f.read(16))
            if magic != 2051:
                raise ValueError(f'Invalid magic number {magic} in {filepath}')
            
            # 读取图像数据
            images = np.frombuffer(f.read(), dtype=np.uint8)
            images = images.reshape(num_images, rows, cols)
            return images
    
    def load_mnist_labels(self, filepath: Path) -> np.ndarray:
        """加载MNIST标签数据"""
        with gzip.open(filepath, 'rb') as f:
            # 读取文件头
            magic, num_labels = struct.unpack('>II', f.read(8))
            if magic != 2049:
                raise ValueError(f'Invalid magic number {magic} in {filepath}')
            
            # 读取标签数据
            labels = np.frombuffer(f.read(), dtype=np.uint8)
            return labels
    
    def load_data(self):
        """加载所有MNIST数据"""
        # 检查数据目录是否存在
        if not self.data_dir.exists():
            raise FileNotFoundError(f"数据目录不存在: {self.data_dir}")
        
        # 定义文件路径
        train_images_path = self.data_dir / 'train-images-idx3-ubyte.gz'
        train_labels_path = self.data_dir / 'train-labels-idx1-ubyte.gz'
        test_images_path = self.data_dir / 't10k-images-idx3-ubyte.gz'
        test_labels_path = self.data_dir / 't10k-labels-idx1-ubyte.gz'
        
        # 检查文件是否存在
        required_files = [train_images_path, train_labels_path, test_images_path, test_labels_path]
        for file_path in required_files:
            if not file_path.exists():
                raise FileNotFoundError(f"MNIST文件不存在: {file_path}")
        
        print("加载MNIST数据...")
        print(f"数据目录: {self.data_dir.absolute()}")
        
        self.train_images = self.load_mnist_images(train_images_path)
        self.train_labels = self.load_mnist_labels(train_labels_path)
        self.test_images = self.load_mnist_images(test_images_path)
        self.test_labels = self.load_mnist_labels(test_labels_path)
        
        print(f"训练集: {self.train_images.shape[0]} 样本")
        print(f"测试集: {self.test_images.shape[0]} 样本")
        print(f"图像尺寸: {self.train_images.shape[1]}x{self.train_images.shape[2]}")
    
    def split_by_samples(self, images: np.ndarray, labels: np.ndarray, 
                        num_splits: int, ratios: List[float] = None) -> List[Tuple[np.ndarray, np.ndarray]]:
        """按样本水平拆分数据"""
        total_samples = len(images)
        
        if ratios is None:
            # 均匀拆分
            samples_per_split = total_samples // num_splits
            splits = []
            
            for i in range(num_splits):
                start_idx = i * samples_per_split
                if i == num_splits - 1:  # 最后一个分片包含剩余样本
                    end_idx = total_samples
                else:
                    end_idx = (i + 1) * samples_per_split
                
                split_images = images[start_idx:end_idx]
                split_labels = labels[start_idx:end_idx]
                splits.append((split_images, split_labels))
        else:
            # 按比例拆分
            if len(ratios) != num_splits:
                raise ValueError("比例数量必须等于拆分数量")
            if abs(sum(ratios) - 1.0) > 1e-6:
                raise ValueError("比例之和必须等于1.0")
            
            splits = []
            start_idx = 0
            
            for i, ratio in enumerate(ratios):
                if i == len(ratios) - 1:  # 最后一个分片
                    end_idx = total_samples
                else:
                    end_idx = start_idx + int(total_samples * ratio)
                
                split_images = images[start_idx:end_idx]
                split_labels = labels[start_idx:end_idx]
                splits.append((split_images, split_labels))
                start_idx = end_idx
        
        return splits
    
    def split_by_features(self, images: np.ndarray, labels: np.ndarray, 
                         num_splits: int, ratios: List[float] = None) -> List[Tuple[np.ndarray, np.ndarray]]:
        """按特征垂直拆分数据（拆分图像像素）"""
        # 将图像展平为特征向量
        flattened_images = images.reshape(images.shape[0], -1)
        total_features = flattened_images.shape[1]
        
        if ratios is None:
            # 均匀拆分特征
            features_per_split = total_features // num_splits
            splits = []
            
            for i in range(num_splits):
                start_idx = i * features_per_split
                if i == num_splits - 1:  # 最后一个分片包含剩余特征
                    end_idx = total_features
                else:
                    end_idx = (i + 1) * features_per_split
                
                split_images = flattened_images[:, start_idx:end_idx]
                # 每个特征分片都包含完整的标签
                splits.append((split_images, labels.copy()))
        else:
            # 按比例拆分特征
            if len(ratios) != num_splits:
                raise ValueError("比例数量必须等于拆分数量")
            if abs(sum(ratios) - 1.0) > 1e-6:
                raise ValueError("比例之和必须等于1.0")
            
            splits = []
            start_idx = 0
            
            for i, ratio in enumerate(ratios):
                if i == len(ratios) - 1:  # 最后一个分片
                    end_idx = total_features
                else:
                    end_idx = start_idx + int(total_features * ratio)
                
                split_images = flattened_images[:, start_idx:end_idx]
                splits.append((split_images, labels.copy()))
                start_idx = end_idx
        
        return splits
    
    def prepare_output_dir(self, output_dir: Path):
        """准备输出目录，删除已存在的文件"""
        if output_dir.exists():
            print(f"清空已存在的输出目录: {output_dir}")
            shutil.rmtree(output_dir)
        
        output_dir.mkdir(parents=True, exist_ok=True)
        print(f"创建输出目录: {output_dir}")
    
    def save_splits(self, splits: List[Tuple[np.ndarray, np.ndarray]], 
                   output_dir: Path, prefix: str, format_type: str = "csv"):
        """保存拆分后的数据"""
        # 准备输出目录（清空重建）
        self.prepare_output_dir(output_dir)
        
        # 保存元数据
        metadata = {
            "num_splits": len(splits),
            "format": format_type,
            "splits_info": []
        }
        
        for i, (split_images, split_labels) in enumerate(splits):
            split_id = f"{prefix}_split_{i:03d}"
            
            # 记录分片信息
            split_info = {
                "split_id": split_id,
                "num_samples": len(split_images),
                "num_features": split_images.shape[1] if len(split_images.shape) == 2 else split_images.shape[1] * split_images.shape[2],
                "image_shape": list(split_images.shape[1:])
            }
            metadata["splits_info"].append(split_info)
            
            if format_type == "csv":
                # 保存为CSV格式，便于Go读取
                images_file = output_dir / f"{split_id}_images.csv"
                labels_file = output_dir / f"{split_id}_labels.csv"
                
                # 展平图像数据
                if len(split_images.shape) > 2:
                    flattened_images = split_images.reshape(split_images.shape[0], -1)
                else:
                    flattened_images = split_images
                
                # 保存图像数据
                np.savetxt(images_file, flattened_images, delimiter=',', fmt='%d')
                
                # 保存标签数据
                np.savetxt(labels_file, split_labels, delimiter=',', fmt='%d')
                
            elif format_type == "npy":
                # 保存为numpy格式
                images_file = output_dir / f"{split_id}_images.npy"
                labels_file = output_dir / f"{split_id}_labels.npy"
                
                np.save(images_file, split_images)
                np.save(labels_file, split_labels)
                
            elif format_type == "binary":
                # 保存为二进制格式，便于Go读取
                images_file = output_dir / f"{split_id}_images.bin"
                labels_file = output_dir / f"{split_id}_labels.bin"
                
                # 保存图像数据（小端序）
                with open(images_file, 'wb') as f:
                    # 写入维度信息
                    f.write(struct.pack('<I', len(split_images.shape)))
                    for dim in split_images.shape:
                        f.write(struct.pack('<I', dim))
                    # 写入数据
                    f.write(split_images.astype(np.uint8).tobytes())
                
                # 保存标签数据
                with open(labels_file, 'wb') as f:
                    f.write(struct.pack('<I', len(split_labels)))
                    f.write(split_labels.astype(np.uint8).tobytes())
            
            print(f"保存分片 {i+1}/{len(splits)}: {split_id} "
                  f"({len(split_images)} 样本, {split_images.shape[1:]} 维度)")
        
        # 保存元数据
        metadata_file = output_dir / f"{prefix}_metadata.json"
        with open(metadata_file, 'w', encoding='utf-8') as f:
            json.dump(metadata, f, indent=2, ensure_ascii=False)
        
        print(f"元数据保存到: {metadata_file}")
        print(f"数据格式: {format_type}")
    
    def create_go_helper_code(self, output_dir: Path, prefix: str):
        """生成Go语言读取数据的辅助代码"""
        go_code = f'''package main

import (
    "encoding/binary"
    "encoding/csv"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "os"
    "strconv"
)

type SplitInfo struct {{
    SplitID     string `json:"split_id"`
    NumSamples  int    `json:"num_samples"`
    NumFeatures int    `json:"num_features"`
    ImageShape  []int  `json:"image_shape"`
}}

type Metadata struct {{
    NumSplits   int         `json:"num_splits"`
    Format      string      `json:"format"`
    SplitsInfo  []SplitInfo `json:"splits_info"`
}}

// 读取元数据
func LoadMetadata(filepath string) (*Metadata, error) {{
    data, err := ioutil.ReadFile(filepath)
    if err != nil {{
        return nil, err
    }}
    
    var metadata Metadata
    err = json.Unmarshal(data, &metadata)
    return &metadata, err
}}

// 读取CSV格式的图像数据
func LoadImagesCSV(filepath string) ([][]float64, error) {{
    file, err := os.Open(filepath)
    if err != nil {{
        return nil, err
    }}
    defer file.Close()
    
    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {{
        return nil, err
    }}
    
    images := make([][]float64, len(records))
    for i, record := range records {{
        images[i] = make([]float64, len(record))
        for j, val := range record {{
            images[i][j], err = strconv.ParseFloat(val, 64)
            if err != nil {{
                return nil, err
            }}
        }}
    }}
    
    return images, nil
}}

// 读取CSV格式的标签数据
func LoadLabelsCSV(filepath string) ([]int, error) {{
    file, err := os.Open(filepath)
    if err != nil {{
        return nil, err
    }}
    defer file.Close()
    
    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {{
        return nil, err
    }}
    
    labels := make([]int, len(records))
    for i, record := range records {{
        labels[i], err = strconv.Atoi(record[0])
        if err != nil {{
            return nil, err
        }}
    }}
    
    return labels, nil
}}

// 读取二进制格式的图像数据
func LoadImagesBinary(filepath string) ([][]uint8, []int, error) {{
    file, err := os.Open(filepath)
    if err != nil {{
        return nil, nil, err
    }}
    defer file.Close()
    
    // 读取维度数量
    var numDims uint32
    err = binary.Read(file, binary.LittleEndian, &numDims)
    if err != nil {{
        return nil, nil, err
    }}
    
    // 读取各维度大小
    dims := make([]int, numDims)
    for i := range dims {{
        var dim uint32
        err = binary.Read(file, binary.LittleEndian, &dim)
        if err != nil {{
            return nil, nil, err
        }}
        dims[i] = int(dim)
    }}
    
    // 计算总大小
    totalSize := 1
    for _, dim := range dims {{
        totalSize *= dim
    }}
    
    // 读取数据
    data := make([]uint8, totalSize)
    err = binary.Read(file, binary.LittleEndian, &data)
    if err != nil {{
        return nil, nil, err
    }}
    
    // 重塑为2D数组
    images := make([][]uint8, dims[0])
    featuresPerImage := totalSize / dims[0]
    for i := range images {{
        start := i * featuresPerImage
        end := start + featuresPerImage
        images[i] = data[start:end]
    }}
    
    return images, dims, nil
}}

// 示例使用
func main() {{
    // 加载元数据
    metadata, err := LoadMetadata("{prefix}_metadata.json")
    if err != nil {{
        fmt.Printf("Error loading metadata: %v\\n", err)
        return
    }}
    
    fmt.Printf("数据集包含 %d 个分片\\n", metadata.NumSplits)
    
    // 加载第一个分片作为示例
    if len(metadata.SplitsInfo) > 0 {{
        split := metadata.SplitsInfo[0]
        fmt.Printf("加载分片: %s\\n", split.SplitID)
        
        if metadata.Format == "csv" {{
            images, err := LoadImagesCSV(split.SplitID + "_images.csv")
            if err != nil {{
                fmt.Printf("Error loading images: %v\\n", err)
                return
            }}
            
            labels, err := LoadLabelsCSV(split.SplitID + "_labels.csv")
            if err != nil {{
                fmt.Printf("Error loading labels: %v\\n", err)
                return
            }}
            
            fmt.Printf("加载了 %d 个样本, %d 个特征\\n", len(images), len(images[0]))
            fmt.Printf("前5个标签: %v\\n", labels[:5])
        }}
    }}
}}
'''
        
        go_file = output_dir / "mnist_reader.go"
        with open(go_file, 'w', encoding='utf-8') as f:
            f.write(go_code)
        
        print(f"Go辅助代码保存到: {go_file}")

def parse_ratios(ratios_str: str) -> List[float]:
    """解析比例字符串"""
    if not ratios_str:
        return None
    
    try:
        ratios = [float(x.strip()) for x in ratios_str.split(',')]
        if abs(sum(ratios) - 1.0) > 1e-6:
            raise ValueError("比例之和必须等于1.0")
        return ratios
    except ValueError as e:
        raise argparse.ArgumentTypeError(f"无效的比例格式: {e}")

def main():
    parser = argparse.ArgumentParser(description='MNIST数据集分割工具')
    parser.add_argument('--num_splits', type=int, required=True,
                       help='拆分数量')
    parser.add_argument('--ratios', type=str, default=None,
                       help='各分片比例，用逗号分隔，如: 0.3,0.3,0.4 (默认均匀拆分)')
    parser.add_argument('--format', type=str, choices=['csv', 'npy', 'binary'], 
                       default='csv', help='输出格式 (默认: csv)')
    parser.add_argument('--use_train_only', action='store_true',
                       help='仅使用训练集数据')
    
    args = parser.parse_args()
    
    # 解析比例
    ratios = parse_ratios(args.ratios) if args.ratios else None
    if ratios and len(ratios) != args.num_splits:
        parser.error("比例数量必须等于拆分数量")
    
    # 创建分割器
    splitter = MNISTSplitter()
    
    try:
        # 加载数据
        splitter.load_data()
        
        # 定义输出目录
        horizontal_dir = Path("./horizontal")
        vertical_dir = Path("./vertical")
        
        print(f"\\n开始数据拆分...")
        print(f"拆分数量: {args.num_splits}")
        print(f"输出格式: {args.format}")
        print(f"比例设置: {'均匀拆分' if ratios is None else ratios}")
        
        if args.use_train_only:
            # 仅使用训练集
            print(f"\\n=== 仅处理训练集数据 ===")
            
            # 水平拆分（按样本）
            print(f"\\n>>> 水平拆分训练集（按样本）...")
            train_splits_horizontal = splitter.split_by_samples(
                splitter.train_images, splitter.train_labels, args.num_splits, ratios)
            splitter.save_splits(train_splits_horizontal, horizontal_dir, "train", args.format)
            splitter.create_go_helper_code(horizontal_dir, "train")
            
            # 垂直拆分（按特征）
            print(f"\\n>>> 垂直拆分训练集（按特征）...")
            train_splits_vertical = splitter.split_by_features(
                splitter.train_images, splitter.train_labels, args.num_splits, ratios)
            splitter.save_splits(train_splits_vertical, vertical_dir, "train", args.format)
            splitter.create_go_helper_code(vertical_dir, "train")
            
        else:
            # 处理训练集和测试集
            print(f"\\n=== 处理训练集和测试集 ===")
            
            # 水平拆分
            print(f"\\n>>> 水平拆分（按样本）...")
            print("处理训练集...")
            train_splits_horizontal = splitter.split_by_samples(
                splitter.train_images, splitter.train_labels, args.num_splits, ratios)
            
            print("处理测试集...")
            test_splits_horizontal = splitter.split_by_samples(
                splitter.test_images, splitter.test_labels, args.num_splits, ratios)
            
            # 保存水平拆分结果
            splitter.save_splits(train_splits_horizontal, horizontal_dir, "train", args.format)
            # 为了避免覆盖，测试集保存在同一目录但不同前缀
            for i, (split_images, split_labels) in enumerate(test_splits_horizontal):
                split_id = f"test_split_{i:03d}"
                
                if args.format == "csv":
                    images_file = horizontal_dir / f"{split_id}_images.csv"
                    labels_file = horizontal_dir / f"{split_id}_labels.csv"
                    
                    flattened_images = split_images.reshape(split_images.shape[0], -1) if len(split_images.shape) > 2 else split_images
                    np.savetxt(images_file, flattened_images, delimiter=',', fmt='%d')
                    np.savetxt(labels_file, split_labels, delimiter=',', fmt='%d')
                    
                elif args.format == "npy":
                    images_file = horizontal_dir / f"{split_id}_images.npy"
                    labels_file = horizontal_dir / f"{split_id}_labels.npy"
                    np.save(images_file, split_images)
                    np.save(labels_file, split_labels)
                    
                elif args.format == "binary":
                    images_file = horizontal_dir / f"{split_id}_images.bin"
                    labels_file = horizontal_dir / f"{split_id}_labels.bin"
                    
                    with open(images_file, 'wb') as f:
                        f.write(struct.pack('<I', len(split_images.shape)))
                        for dim in split_images.shape:
                            f.write(struct.pack('<I', dim))
                        f.write(split_images.astype(np.uint8).tobytes())
                    
                    with open(labels_file, 'wb') as f:
                        f.write(struct.pack('<I', len(split_labels)))
                        f.write(split_labels.astype(np.uint8).tobytes())
            
            splitter.create_go_helper_code(horizontal_dir, "train")
            
            # 垂直拆分
            print(f"\\n>>> 垂直拆分（按特征）...")
            print("处理训练集...")
            train_splits_vertical = splitter.split_by_features(
                splitter.train_images, splitter.train_labels, args.num_splits, ratios)
            
            print("处理测试集...")
            test_splits_vertical = splitter.split_by_features(
                splitter.test_images, splitter.test_labels, args.num_splits, ratios)
            
            # 保存垂直拆分结果
            splitter.save_splits(train_splits_vertical, vertical_dir, "train", args.format)
            # 保存测试集垂直拆分
            for i, (split_images, split_labels) in enumerate(test_splits_vertical):
                split_id = f"test_split_{i:03d}"
                
                if args.format == "csv":
                    images_file = vertical_dir / f"{split_id}_images.csv"
                    labels_file = vertical_dir / f"{split_id}_labels.csv"
                    np.savetxt(images_file, split_images, delimiter=',', fmt='%d')
                    np.savetxt(labels_file, split_labels, delimiter=',', fmt='%d')
                    
                elif args.format == "npy":
                    images_file = vertical_dir / f"{split_id}_images.npy"
                    labels_file = vertical_dir / f"{split_id}_labels.npy"
                    np.save(images_file, split_images)
                    np.save(labels_file, split_labels)
                    
                elif args.format == "binary":
                    images_file = vertical_dir / f"{split_id}_images.bin"
                    labels_file = vertical_dir / f"{split_id}_labels.bin"
                    
                    with open(images_file, 'wb') as f:
                        f.write(struct.pack('<I', len(split_images.shape)))
                        for dim in split_images.shape:
                            f.write(struct.pack('<I', dim))
                        f.write(split_images.astype(np.uint8).tobytes())
                    
                    with open(labels_file, 'wb') as f:
                        f.write(struct.pack('<I', len(split_labels)))
                        f.write(split_labels.astype(np.uint8).tobytes())
            
            splitter.create_go_helper_code(vertical_dir, "train")
        
        print(f"\\n=== 拆分完成! ===")
        print(f"水平拆分结果: ./horizontal/")
        print(f"垂直拆分结果: ./vertical/")
        print(f"拆分数量: {args.num_splits}")
        print(f"输出格式: {args.format}")
        
    except FileNotFoundError as e:
        print(f"错误: {e}")
        print("请确保MNIST数据集文件存在于 ./raw/ 目录中:")
        print("  - train-images-idx3-ubyte.gz")
        print("  - train-labels-idx1-ubyte.gz") 
        print("  - t10k-images-idx3-ubyte.gz")
        print("  - t10k-labels-idx1-ubyte.gz")
    except Exception as e:
        print(f"发生错误: {e}")

if __name__ == "__main__":
    main()