package training

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// Dataset MNIST数据集结构
type Dataset struct {
	Images [][]byte
	Labels []byte
}

// LoadImages 从IDX文件加载图像数据
func LoadImages(filename string) ([][]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("无法打开图像文件: %v", err)
	}
	defer file.Close()

	// 解压缩文件
	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("无法解压缩文件: %v", err)
	}
	defer reader.Close()

	// 读取IDX头信息
	var magicNumber, numImages, numRows, numCols int32
	err = binary.Read(reader, binary.BigEndian, &magicNumber)
	if err != nil {
		return nil, fmt.Errorf("读取魔数失败: %v", err)
	}
	if magicNumber != 2051 {
		return nil, fmt.Errorf("文件格式不正确（魔数不匹配）")
	}
	err = binary.Read(reader, binary.BigEndian, &numImages)
	if err != nil {
		return nil, fmt.Errorf("读取图像数量失败: %v", err)
	}
	err = binary.Read(reader, binary.BigEndian, &numRows)
	if err != nil {
		return nil, fmt.Errorf("读取行数失败: %v", err)
	}
	err = binary.Read(reader, binary.BigEndian, &numCols)
	if err != nil {
		return nil, fmt.Errorf("读取列数失败: %v", err)
	}

	// 读取图像数据
	images := make([][]byte, numImages)
	for i := 0; i < int(numImages); i++ {
		img := make([]byte, numRows*numCols)
		_, err := io.ReadFull(reader, img)
		if err != nil {
			return nil, fmt.Errorf("读取图像数据失败: %v", err)
		}
		images[i] = img
	}

	return images, nil
}

// LoadLabels 从IDX文件加载标签数据
func LoadLabels(filename string) ([]byte, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("无法打开标签文件: %v", err)
	}
	defer file.Close()

	// 解压缩文件
	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("无法解压缩文件: %v", err)
	}
	defer reader.Close()

	// 读取IDX头信息
	var magicNumber, numItems int32
	err = binary.Read(reader, binary.BigEndian, &magicNumber)
	if err != nil {
		return nil, fmt.Errorf("读取魔数失败: %v", err)
	}
	if magicNumber != 2049 {
		return nil, fmt.Errorf("文件格式不正确（魔数不匹配）")
	}
	err = binary.Read(reader, binary.BigEndian, &numItems)
	if err != nil {
		return nil, fmt.Errorf("读取标签数量失败: %v", err)
	}

	// 读取标签数据
	labels := make([]byte, numItems)
	_, err = io.ReadFull(reader, labels)
	if err != nil {
		return nil, fmt.Errorf("读取标签数据失败: %v", err)
	}

	return labels, nil
}

// LoadDataset 加载训练和测试数据集
func LoadDataset() (*Dataset, *Dataset, error) {
	// 加载训练数据
	trainImages, err := LoadImages("pkg/training/data/train-images-idx3-ubyte.gz")
	if err != nil {
		return nil, nil, fmt.Errorf("加载训练图像数据失败: %v", err)
	}
	trainLabels, err := LoadLabels("pkg/training/data/train-labels-idx1-ubyte.gz")
	if err != nil {
		return nil, nil, fmt.Errorf("加载训练标签数据失败: %v", err)
	}
	trainDataset := &Dataset{
		Images: trainImages,
		Labels: trainLabels,
	}

	// 加载测试数据
	testImages, err := LoadImages("pkg/training/data/t10k-images-idx3-ubyte.gz")
	if err != nil {
		return nil, nil, fmt.Errorf("加载测试图像数据失败: %v", err)
	}
	testLabels, err := LoadLabels("pkg/training/data/t10k-labels-idx1-ubyte.gz")
	if err != nil {
		return nil, nil, fmt.Errorf("加载测试标签数据失败: %v", err)
	}
	testDataset := &Dataset{
		Images: testImages,
		Labels: testLabels,
	}

	return trainDataset, testDataset, nil
}
