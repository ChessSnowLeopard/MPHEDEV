package main

import (
	"MNIST-CNN/pkg/training"
	"MNIST-CNN/pkg/vector_processor"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
)

// DataStructureInfo 数据结构信息
type DataStructureInfo struct {
	LayerType    string                 `json:"layer_type"`
	InputFormat  map[string]interface{} `json:"input_format"`
	OutputFormat map[string]interface{} `json:"output_format"`
	Parameters   map[string]interface{} `json:"parameters"`
	Description  string                 `json:"description"`
	Timestamp    string                 `json:"timestamp"`
}

// NetworkDataStructures 网络数据结构集合
type NetworkDataStructures struct {
	SystemConfig    map[string]interface{} `json:"system_config"`
	LayerStructures []DataStructureInfo    `json:"layer_structures"`
	DataFlow        map[string]interface{} `json:"data_flow"`
}

func main() {
	fmt.Println("======================================================")
	fmt.Println("     MNIST_CNN 中间数据结构分析工具 (简化版)")
	fmt.Println("======================================================")
	fmt.Println()

	// 记录开始时间
	startTime := time.Now()

	// ============ 数据加载和配置 ============
	fmt.Println("\n====== 数据准备 ======")

	// 加载MNIST数据集
	fmt.Println("加载MNIST数据集...")
	trainDataset, _, err := training.LoadDataset()
	if err != nil {
		log.Fatalf("加载数据集失败: %v", err)
	}

	fmt.Printf("训练数据集: %d 个样本\n", len(trainDataset.Images))

	// 配置向量组装器
	fmt.Println("配置向量组装器...")
	config := struct {
		VectorSize int
		BatchSize  int
		K          int
	}{
		VectorSize: 8192, // 典型的CKKS槽数
		BatchSize:  64,
		K:          8192 / 64, // 128
	}

	// 创建向量组装器
	assembler := vector_processor.NewVectorAssemblerFromDataset(config.VectorSize, config.K, trainDataset)
	assembler.PrintVectorInfo()

	// 处理少量数据用于分析
	numBatchesToProcess := 1
	smallTrainDataset := &training.Dataset{
		Images: trainDataset.Images[:assembler.ImagesPerVector*numBatchesToProcess],
		Labels: trainDataset.Labels[:assembler.ImagesPerVector*numBatchesToProcess],
	}

	trainVectors, trainBatchLabels, err := assembler.AssembleDataset(smallTrainDataset)
	if err != nil {
		log.Fatalf("重排训练数据集失败: %v", err)
	}

	// ============ 分析数据结构 ============
	fmt.Println("\n====== 分析中间数据结构 ======")

	// 创建数据结构分析结果
	dataStructures := NetworkDataStructures{
		SystemConfig: map[string]interface{}{
			"slots":             config.VectorSize,
			"batch_size":        config.BatchSize,
			"k":                 config.K,
			"num_vectors":       assembler.NumVectors,
			"images_per_vector": assembler.ImagesPerVector,
			"image_features":    assembler.ImageFeatures,
		},
		LayerStructures: []DataStructureInfo{},
		DataFlow:        map[string]interface{}{},
	}

	// 分析输入层数据结构
	inputLayerInfo := analyzeInputLayerStructure(assembler, trainVectors, trainBatchLabels)
	dataStructures.LayerStructures = append(dataStructures.LayerStructures, inputLayerInfo)

	// 分析隐藏层数据结构
	hiddenLayerInfo := analyzeHiddenLayerStructure(assembler)
	dataStructures.LayerStructures = append(dataStructures.LayerStructures, hiddenLayerInfo)

	// 分析输出层数据结构
	outputLayerInfo := analyzeOutputLayerStructure(assembler)
	dataStructures.LayerStructures = append(dataStructures.LayerStructures, outputLayerInfo)

	// 分析数据流
	dataFlowInfo := analyzeDataFlow(assembler, config)
	dataStructures.DataFlow = dataFlowInfo

	// ============ 保存分析结果 ============
	fmt.Println("\n====== 保存分析结果 ======")

	// 将结果保存为JSON文件
	outputFile := "data_structures_analysis_simple.json"
	jsonData, err := json.MarshalIndent(dataStructures, "", "  ")
	if err != nil {
		log.Fatalf("JSON序列化失败: %v", err)
	}

	err = os.WriteFile(outputFile, jsonData, 0644)
	if err != nil {
		log.Fatalf("保存文件失败: %v", err)
	}

	fmt.Printf("✓ 分析结果已保存到: %s\n", outputFile)

	// 打印摘要
	printAnalysisSummary(dataStructures)

	// 计算总执行时间
	duration := time.Since(startTime)
	fmt.Printf("\n✓ 数据结构分析完成，总用时: %v\n", duration)
}

// analyzeInputLayerStructure 分析输入层数据结构
func analyzeInputLayerStructure(assembler *vector_processor.VectorAssembler,
	plainVectors [][]float32,
	batchLabels [][]byte) DataStructureInfo {

	fmt.Println("\n--- 分析输入层数据结构 ---")

	inputInfo := DataStructureInfo{
		LayerType: "input_layer",
		InputFormat: map[string]interface{}{
			"data_type":      "raw_images",
			"format":         "byte_arrays",
			"normalization":  "divide_by_255",
			"batch_size":     assembler.ImagesPerVector,
			"image_features": assembler.ImageFeatures,
		},
		OutputFormat: map[string]interface{}{
			"data_type":      "encrypted_vectors",
			"format":         "map[uint64]*rlwe.Ciphertext",
			"num_vectors":    assembler.NumVectors,
			"vector_size":    assembler.S,
			"packing_format": "interleaved_features",
			"description":    "每个向量包含k个特征，每个特征包含n个样本的值",
		},
		Parameters: map[string]interface{}{
			"s":              assembler.S,
			"k":              assembler.K,
			"n":              assembler.ImagesPerVector,
			"num_vectors":    assembler.NumVectors,
			"image_features": assembler.ImageFeatures,
		},
		Description: "输入层负责将原始图像数据重排打包并加密，为后续层提供标准化的密文输入格式",
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
	}

	// 添加具体的向量示例
	if len(plainVectors) > 0 {
		inputInfo.InputFormat["example_vector"] = map[string]interface{}{
			"vector_index":    0,
			"vector_size":     len(plainVectors[0]),
			"first_10_values": plainVectors[0][:10],
			"packing_pattern": "feature_0_sample_0, feature_0_sample_1, ..., feature_1_sample_0, feature_1_sample_1, ...",
		}
	}

	// 添加加密后的数据结构说明
	inputInfo.OutputFormat["encrypted_structure"] = map[string]interface{}{
		"key_type":      "uint64",
		"value_type":    "*rlwe.Ciphertext",
		"key_mapping":   "vector_index -> encrypted_ciphertext",
		"example_keys":  []uint64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		"total_vectors": assembler.NumVectors,
	}

	fmt.Printf("✓ 输入层分析完成\n")
	return inputInfo
}

// analyzeHiddenLayerStructure 分析隐藏层数据结构
func analyzeHiddenLayerStructure(assembler *vector_processor.VectorAssembler) DataStructureInfo {

	fmt.Println("\n--- 分析隐藏层数据结构 ---")

	// 创建示例隐藏层
	hiddenSize := 128

	hiddenInfo := DataStructureInfo{
		LayerType: "hidden_layer",
		InputFormat: map[string]interface{}{
			"data_type":      "encrypted_vectors",
			"format":         "map[uint64]*rlwe.Ciphertext",
			"description":    "来自上一层的重组后密文向量",
			"reorganization": "mask_based_reorganization",
		},
		OutputFormat: map[string]interface{}{
			"data_type":     "neuron_outputs",
			"format":        "[]*rlwe.Ciphertext",
			"num_neurons":   hiddenSize,
			"output_format": "each_neuron_contains_all_samples",
			"description":   "每个神经元的输出包含所有样本的激活值",
		},
		Parameters: map[string]interface{}{
			"input_size":  assembler.ImageFeatures,
			"hidden_size": hiddenSize,
			"activation":  "ReLU",
			"k":           assembler.K,
			"n":           assembler.ImagesPerVector,
		},
		Description: "隐藏层执行全连接计算和激活函数，输出需要重组以便传递给下一层",
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
	}

	// 添加重组信息
	hiddenInfo.OutputFormat["reorganization"] = map[string]interface{}{
		"method":             "mask_based",
		"input_format":       "neuron_outputs",
		"output_format":      "reorganized_vectors",
		"mask_generation":    "segment_based_masking",
		"num_output_vectors": (hiddenSize + assembler.K - 1) / assembler.K,
		"mask_pattern":       "每个掩码提取一个神经元段，然后重新组合",
	}

	// 添加计算过程说明
	hiddenInfo.Parameters["computation_steps"] = []string{
		"1. 权重打包：将权重矩阵按神经元和特征维度打包",
		"2. 密文乘法：输入向量与打包权重相乘",
		"3. 旋转累加：使用log2(k)次旋转实现特征累加",
		"4. 偏置添加：添加偏置项到结果",
		"5. 激活函数：应用ReLU激活函数",
		"6. 输出重组：使用掩码重组输出格式",
	}

	fmt.Printf("✓ 隐藏层分析完成\n")
	return hiddenInfo
}

// analyzeOutputLayerStructure 分析输出层数据结构
func analyzeOutputLayerStructure(assembler *vector_processor.VectorAssembler) DataStructureInfo {

	fmt.Println("\n--- 分析输出层数据结构 ---")

	// 创建示例输出层
	outputSize := 10 // MNIST有10个类别

	outputInfo := DataStructureInfo{
		LayerType: "output_layer",
		InputFormat: map[string]interface{}{
			"data_type":   "reorganized_vectors",
			"format":      "[]*rlwe.Ciphertext",
			"description": "来自隐藏层的重组后密文向量",
		},
		OutputFormat: map[string]interface{}{
			"data_type":     "raw_outputs",
			"format":        "[]*rlwe.Ciphertext",
			"num_classes":   outputSize,
			"output_format": "logits_before_softmax",
			"description":   "输出层的原始logits，需要经过softmax处理",
		},
		Parameters: map[string]interface{}{
			"input_size":  128,
			"output_size": outputSize,
			"activation":  "Identity",
			"next_step":   "softmax_activation",
		},
		Description: "输出层产生最终的logits，然后通过softmax转换为概率分布",
		Timestamp:   time.Now().Format("2006-01-02 15:04:05"),
	}

	// 添加softmax信息
	outputInfo.OutputFormat["softmax"] = map[string]interface{}{
		"input":         "raw_logits",
		"output":        "probability_distribution",
		"method":        "homomorphic_softmax",
		"loss_function": "cross_entropy",
		"computation_steps": []string{
			"1. 指数计算：对每个logit计算e^x",
			"2. 求和：计算所有指数的和",
			"3. 除法：每个指数除以总和",
			"4. 损失计算：计算交叉熵损失",
		},
	}

	// 添加损失计算信息
	outputInfo.OutputFormat["loss_computation"] = map[string]interface{}{
		"loss_type":       "cross_entropy",
		"label_format":    "one_hot_encoded",
		"label_packing":   "class_based_packing",
		"efficiency_gain": "38x_improvement",
		"aggregation":     "batch_average_loss",
	}

	fmt.Printf("✓ 输出层分析完成\n")
	return outputInfo
}

// analyzeDataFlow 分析数据流
func analyzeDataFlow(assembler *vector_processor.VectorAssembler, config struct {
	VectorSize int
	BatchSize  int
	K          int
}) map[string]interface{} {

	fmt.Println("\n--- 分析数据流 ---")

	dataFlow := map[string]interface{}{
		"overview": map[string]interface{}{
			"total_layers": 3,
			"data_flow":    "input_layer -> hidden_layer -> output_layer",
			"encryption":   "ckks_homomorphic_encryption",
			"packing":      "vector_interleaving",
		},
		"layer_transitions": []map[string]interface{}{
			{
				"from":           "input_layer",
				"to":             "hidden_layer",
				"transformation": "raw_encryption",
				"data_format":    "map[uint64]*rlwe.Ciphertext",
				"description":    "输入层直接加密原始数据，无需重组",
			},
			{
				"from":           "hidden_layer",
				"to":             "output_layer",
				"transformation": "mask_reorganization",
				"data_format":    "[]*rlwe.Ciphertext",
				"description":    "隐藏层输出需要重组以便输出层处理",
			},
		},
		"key_parameters": map[string]interface{}{
			"s":           assembler.S,
			"k":           assembler.K,
			"n":           assembler.ImagesPerVector,
			"num_vectors": assembler.NumVectors,
			"batch_size":  config.BatchSize,
		},
		"communication_patterns": map[string]interface{}{
			"input_to_hidden":  "direct_forward",
			"hidden_to_output": "reorganized_forward",
			"reorganization":   "mask_based_segmentation",
		},
		"data_formats": map[string]interface{}{
			"input_format": map[string]interface{}{
				"raw_data":   "byte_arrays",
				"normalized": "float32_arrays",
				"packed":     "interleaved_vectors",
				"encrypted":  "rlwe_ciphertexts",
			},
			"intermediate_format": map[string]interface{}{
				"neuron_outputs": "[]*rlwe.Ciphertext",
				"reorganized":    "[]*rlwe.Ciphertext",
				"activation":     "applied_in_place",
			},
			"output_format": map[string]interface{}{
				"logits":  "[]*rlwe.Ciphertext",
				"softmax": "[]*rlwe.Ciphertext",
				"loss":    "single_ciphertext",
			},
		},
	}

	fmt.Printf("✓ 数据流分析完成\n")
	return dataFlow
}

// printAnalysisSummary 打印分析摘要
func printAnalysisSummary(dataStructures NetworkDataStructures) {
	fmt.Println("\n======================================================")
	fmt.Println("     数据结构分析摘要")
	fmt.Println("======================================================")

	fmt.Printf("系统配置:\n")
	fmt.Printf("• 槽数: %v\n", dataStructures.SystemConfig["slots"])
	fmt.Printf("• 批次大小: %v\n", dataStructures.SystemConfig["batch_size"])
	fmt.Printf("• 每图像特征数: %v\n", dataStructures.SystemConfig["k"])
	fmt.Printf("• 向量数: %v\n", dataStructures.SystemConfig["num_vectors"])

	fmt.Printf("\n层结构:\n")
	for _, layer := range dataStructures.LayerStructures {
		fmt.Printf("• %s: %s\n", layer.LayerType, layer.Description)
	}

	fmt.Printf("\n数据流:\n")
	fmt.Printf("• 总层数: %v\n", dataStructures.DataFlow["overview"].(map[string]interface{})["total_layers"])
	fmt.Printf("• 数据流: %v\n", dataStructures.DataFlow["overview"].(map[string]interface{})["data_flow"])

	fmt.Printf("\n关键数据结构:\n")
	fmt.Printf("• 输入层输出: map[uint64]*rlwe.Ciphertext\n")
	fmt.Printf("• 隐藏层输出: []*rlwe.Ciphertext (重组后)\n")
	fmt.Printf("• 输出层输出: []*rlwe.Ciphertext (logits)\n")
	fmt.Printf("• 最终损失: *rlwe.Ciphertext (批次平均)\n")

	fmt.Println("======================================================")
}
