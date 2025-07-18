package services

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

// LoadDataset 载入本地数据集
func (p *Participant) LoadDataset() error {
	dataDir := fmt.Sprintf("../../data/%s", p.DataManager.GetDataSplit())
	splitID := fmt.Sprintf("train_split_%03d", p.ID-1)
	imagesPath := fmt.Sprintf("%s/%s_images.csv", dataDir, splitID)
	labelsPath := fmt.Sprintf("%s/%s_labels.csv", dataDir, splitID)

	if _, err := os.Stat(imagesPath); os.IsNotExist(err) {
		return fmt.Errorf("图像文件不存在: %s", imagesPath)
	}
	if _, err := os.Stat(labelsPath); os.IsNotExist(err) {
		return fmt.Errorf("标签文件不存在: %s", labelsPath)
	}

	images, err := p.loadImagesCSV(imagesPath)
	if err != nil {
		return fmt.Errorf("载入图像数据失败: %v", err)
	}
	labels, err := p.loadLabelsCSV(labelsPath)
	if err != nil {
		return fmt.Errorf("载入标签数据失败: %v", err)
	}

	// 限制载入的数据量（测试模式）
	const batchSize = 128
	const numBatches = 1 // 只处理1个完整批次，减少内存开销
	maxSamples := batchSize * numBatches

	if len(images) > maxSamples {
		images = images[:maxSamples]
		p.LogInfo(fmt.Sprintf("测试模式：载入前 %d 个样本（%d个完整批次）", maxSamples, numBatches))
	}
	if len(labels) > maxSamples {
		labels = labels[:maxSamples]
		p.LogInfo(fmt.Sprintf("测试模式：载入前 %d 个标签（%d个完整批次）", maxSamples, numBatches))
	}

	p.DataManager.SetImages(images)
	p.DataManager.SetLabels(labels)

	p.LogInfo("数据集载入完成:")
	p.LogInfo(fmt.Sprintf("  • 图像数量: %d", len(images)))
	p.LogInfo(fmt.Sprintf("  • 标签数量: %d", len(labels)))
	p.LogInfo(fmt.Sprintf("  • 数据划分方式: %s", p.DataManager.GetDataSplit()))

	return nil
}

// loadImagesCSV 载入CSV格式的图像数据
func (p *Participant) loadImagesCSV(filepath string) ([][]float64, error) {
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

// loadLabelsCSV 载入CSV格式的标签数据
func (p *Participant) loadLabelsCSV(filepath string) ([]int, error) {
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
