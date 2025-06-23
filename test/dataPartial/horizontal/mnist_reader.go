package main

import (
    "encoding/binary"
    "encoding/csv"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "os"
    "strconv"
)

type SplitInfo struct {
    SplitID     string `json:"split_id"`
    NumSamples  int    `json:"num_samples"`
    NumFeatures int    `json:"num_features"`
    ImageShape  []int  `json:"image_shape"`
}

type Metadata struct {
    NumSplits   int         `json:"num_splits"`
    Format      string      `json:"format"`
    SplitsInfo  []SplitInfo `json:"splits_info"`
}

// 读取元数据
func LoadMetadata(filepath string) (*Metadata, error) {
    data, err := ioutil.ReadFile(filepath)
    if err != nil {
        return nil, err
    }
    
    var metadata Metadata
    err = json.Unmarshal(data, &metadata)
    return &metadata, err
}

// 读取CSV格式的图像数据
func LoadImagesCSV(filepath string) ([][]float64, error) {
    file, err := os.Open(filepath)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {
        return nil, err
    }
    
    images := make([][]float64, len(records))
    for i, record := range records {
        images[i] = make([]float64, len(record))
        for j, val := range record {
            images[i][j], err = strconv.ParseFloat(val, 64)
            if err != nil {
                return nil, err
            }
        }
    }
    
    return images, nil
}

// 读取CSV格式的标签数据
func LoadLabelsCSV(filepath string) ([]int, error) {
    file, err := os.Open(filepath)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    reader := csv.NewReader(file)
    records, err := reader.ReadAll()
    if err != nil {
        return nil, err
    }
    
    labels := make([]int, len(records))
    for i, record := range records {
        labels[i], err = strconv.Atoi(record[0])
        if err != nil {
            return nil, err
        }
    }
    
    return labels, nil
}

// 读取二进制格式的图像数据
func LoadImagesBinary(filepath string) ([][]uint8, []int, error) {
    file, err := os.Open(filepath)
    if err != nil {
        return nil, nil, err
    }
    defer file.Close()
    
    // 读取维度数量
    var numDims uint32
    err = binary.Read(file, binary.LittleEndian, &numDims)
    if err != nil {
        return nil, nil, err
    }
    
    // 读取各维度大小
    dims := make([]int, numDims)
    for i := range dims {
        var dim uint32
        err = binary.Read(file, binary.LittleEndian, &dim)
        if err != nil {
            return nil, nil, err
        }
        dims[i] = int(dim)
    }
    
    // 计算总大小
    totalSize := 1
    for _, dim := range dims {
        totalSize *= dim
    }
    
    // 读取数据
    data := make([]uint8, totalSize)
    err = binary.Read(file, binary.LittleEndian, &data)
    if err != nil {
        return nil, nil, err
    }
    
    // 重塑为2D数组
    images := make([][]uint8, dims[0])
    featuresPerImage := totalSize / dims[0]
    for i := range images {
        start := i * featuresPerImage
        end := start + featuresPerImage
        images[i] = data[start:end]
    }
    
    return images, dims, nil
}

// 示例使用
func main() {
    // 加载元数据
    metadata, err := LoadMetadata("train_metadata.json")
    if err != nil {
        fmt.Printf("Error loading metadata: %v\n", err)
        return
    }
    
    fmt.Printf("数据集包含 %d 个分片\n", metadata.NumSplits)
    
    // 加载第一个分片作为示例
    if len(metadata.SplitsInfo) > 0 {
        split := metadata.SplitsInfo[0]
        fmt.Printf("加载分片: %s\n", split.SplitID)
        
        if metadata.Format == "csv" {
            images, err := LoadImagesCSV(split.SplitID + "_images.csv")
            if err != nil {
                fmt.Printf("Error loading images: %v\n", err)
                return
            }
            
            labels, err := LoadLabelsCSV(split.SplitID + "_labels.csv")
            if err != nil {
                fmt.Printf("Error loading labels: %v\n", err)
                return
            }
            
            fmt.Printf("加载了 %d 个样本, %d 个特征\n", len(images), len(images[0]))
            fmt.Printf("前5个标签: %v\n", labels[:5])
        }
    }
}
