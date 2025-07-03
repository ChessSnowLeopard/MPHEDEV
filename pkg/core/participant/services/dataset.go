package services

import (
	"MPHEDev/pkg/core/participant/utils"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// LoadDataset 载入本地数据集
func (p *Participant) LoadDataset() error {
	dataDir := fmt.Sprintf("../../data/%s", p.DataSplit)
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

	p.Images = images
	p.Labels = labels

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

// EncryptDataset 加密本地数据集
func (p *Participant) EncryptDataset() error {
	if !p.KeyManager.IsReady() {
		return fmt.Errorf("密钥未准备就绪，无法加密数据")
	}

	if len(p.Images) == 0 {
		return fmt.Errorf("数据集未载入，请先调用LoadDataset")
	}

	// 获取加密所需的组件
	params := p.KeyManager.GetParams()
	pubKey := p.KeyManager.GetPublicKey()
	encoder := ckks.NewEncoder(params)
	encryptor := ckks.NewEncryptor(params, pubKey)

	// 加密每个样本
	encryptedImages := make([]*rlwe.Ciphertext, len(p.Images))
	for i, image := range p.Images {
		// 将图像数据转换为复数向量
		values := make([]complex128, len(image))
		for j, pixel := range image {
			// 归一化像素值到[0,1]范围
			values[j] = complex(pixel/255.0, 0)
		}

		// 编码
		pt := ckks.NewPlaintext(params, params.MaxLevel())
		if err := encoder.Encode(values, pt); err != nil {
			return fmt.Errorf("编码失败: %v", err)
		}

		// 加密
		ct, err := encryptor.EncryptNew(pt)
		if err != nil {
			return fmt.Errorf("加密失败: %v", err)
		}

		encryptedImages[i] = ct

		if (i+1)%100 == 0 {

		}
	}

	return nil
}

// EncryptAndDistributeDataset 加密并分发数据集
func (p *Participant) EncryptAndDistributeDataset() error {
	if !p.KeyManager.IsReady() {
		return fmt.Errorf("密钥未准备就绪，无法加密数据")
	}

	if len(p.Images) == 0 {
		return fmt.Errorf("数据集未载入，请先调用LoadDataset")
	}

	// 获取加密所需的组件
	params := p.KeyManager.GetParams()
	pubKey := p.KeyManager.GetPublicKey()
	encoder := ckks.NewEncoder(params)
	encryptor := ckks.NewEncryptor(params, pubKey)

	// 获取在线参与方列表，确定输入层和输出层ID
	onlinePeers := p.PeerManager.GetPeers()

	// 将自己也加入在线参与方列表
	allParticipants := make(map[int]string)
	for id, url := range onlinePeers {
		allParticipants[id] = url
	}
	// 添加自己（虽然URL可能为空，但ID是有效的）
	allParticipants[p.ID] = ""

	if len(allParticipants) == 0 {
		return fmt.Errorf("没有在线参与方，无法分发数据")
	}

	// 找到输入层(ID=1)和输出层(ID=最大)的参与方
	inputLayerID := 1
	outputLayerID := 1
	for peerID := range allParticipants {
		if peerID == 1 {
			inputLayerID = 1
		}
		if peerID > outputLayerID {
			outputLayerID = peerID
		}
	}

	// 根据参与方角色处理数据
	if p.ID == inputLayerID && p.ID == outputLayerID {
		// 只有一个参与方时：既是输入层又是输出层，需要发送数据给自己

		return p.encryptAndSendData(encoder, encryptor, inputLayerID, outputLayerID)
	} else if p.ID == inputLayerID {
		// 输入层参与方：只接收特征数据，不发送

		return nil
	} else if p.ID == outputLayerID {
		// 输出层参与方：只接收标签数据，不发送

		return nil
	} else {
		// 中间层参与方：加密并发送特征和标签数据
		return p.encryptAndSendData(encoder, encryptor, inputLayerID, outputLayerID)
	}
}

// encryptAndSendData 加密并发送数据
func (p *Participant) encryptAndSendData(encoder *ckks.Encoder, encryptor *rlwe.Encryptor, inputLayerID, outputLayerID int) error {

	// 获取CKKS参数用于确定槽数
	params := p.KeyManager.GetParams()
	slots := params.N() / 2 // CKKS的槽数是N/2

	// 按批次加密和发送特征数据
	if err := p.encryptAndSendFeatures(encoder, encryptor, inputLayerID, slots); err != nil {
		return fmt.Errorf("发送特征数据失败: %v", err)
	}

	// 按批次加密和发送标签数据
	if err := p.encryptAndSendLabels(encoder, encryptor, outputLayerID, slots); err != nil {
		return fmt.Errorf("发送标签数据失败: %v", err)
	}

	return nil
}

// encryptAndSendFeatures 加密并发送特征数据
func (p *Participant) encryptAndSendFeatures(encoder *ckks.Encoder, encryptor *rlwe.Encryptor, targetID int, slots int) error {

	totalSamples := len(p.Images)
	totalFeatures := 156 // MNIST数据集的特征数

	// 计算需要多少个批次来处理所有特征
	// 每个槽存储一个特征值，所以每个批次可以处理slots个特征
	totalFeaturesToProcess := totalSamples * totalFeatures
	batchCount := (totalFeaturesToProcess + slots - 1) / slots // 向上取整

	// 流式发送：每50个批次发送一次
	batchSize := 50
	totalBatches := (batchCount + batchSize - 1) / batchSize

	// 当前批次的密文列表
	var currentBatchCiphertexts []string

	// 按批次处理所有特征
	for batchIndex := 0; batchIndex < batchCount; batchIndex++ {
		startFeatureIndex := batchIndex * slots
		endFeatureIndex := startFeatureIndex + slots
		if endFeatureIndex > totalFeaturesToProcess {
			endFeatureIndex = totalFeaturesToProcess
		}

		// 准备当前批次的数据
		batchData := make([]complex128, slots)
		featuresInThisBatch := 0

		for i := 0; i < slots; i++ {
			globalFeatureIndex := startFeatureIndex + i
			if globalFeatureIndex < totalFeaturesToProcess {
				// 计算样本索引和特征索引
				sampleIndex := globalFeatureIndex / totalFeatures
				featureIndex := globalFeatureIndex % totalFeatures

				if sampleIndex < len(p.Images) && featureIndex < len(p.Images[sampleIndex]) {
					// 归一化特征值到[0,1]范围
					batchData[i] = complex(p.Images[sampleIndex][featureIndex]/255.0, 0)
					featuresInThisBatch++
				} else {
					batchData[i] = complex(0, 0)
				}
			} else {
				batchData[i] = complex(0, 0) // 填充0
			}
		}

		// 编码
		pt := ckks.NewPlaintext(p.KeyManager.GetParams(), p.KeyManager.GetParams().MaxLevel())
		if err := encoder.Encode(batchData, pt); err != nil {
			return fmt.Errorf("编码特征数据批次 %d 失败: %v", batchIndex, err)
		}

		// 加密
		ct, err := encryptor.EncryptNew(pt)
		if err != nil {
			return fmt.Errorf("加密特征数据批次 %d 失败: %v", batchIndex, err)
		}

		// 序列化密文
		ctBytes, err := utils.EncodeShare(ct)
		if err != nil {
			return fmt.Errorf("序列化特征密文批次 %d 失败: %v", batchIndex, err)
		}

		// 添加到当前发送批次
		currentBatchCiphertexts = append(currentBatchCiphertexts, utils.EncodeToBase64(ctBytes))

		// 检查是否需要发送当前批次
		if len(currentBatchCiphertexts) >= batchSize || batchIndex == batchCount-1 {
			sendBatchIndex := (batchIndex / batchSize) + 1
			fmt.Printf("发送特征数据批次 %d/%d (包含 %d 个密文)...\n", sendBatchIndex, totalBatches, len(currentBatchCiphertexts))

			// 构造消息
			message := DataMessage{
				Type:      "feature_batch",
				From:      p.ID,
				Data:      utils.EncodeToBase64([]byte(fmt.Sprintf("%d,%d", sendBatchIndex, totalBatches))), // 发送批次信息
				BatchData: currentBatchCiphertexts,                                                          // 当前批次的密文数据
			}

			// 发送消息
			messageJSON, err := json.Marshal(message)
			if err != nil {
				return fmt.Errorf("序列化特征消息批次 %d 失败: %v", sendBatchIndex, err)
			}

			if err := p.SendMessageToParticipant(targetID, string(messageJSON)); err != nil {
				return fmt.Errorf("发送特征数据批次 %d 到参与方 %d 失败: %v", sendBatchIndex, targetID, err)
			}

			fmt.Printf("特征数据批次 %d/%d 发送完成\n", sendBatchIndex, totalBatches)

			// 清空当前批次
			currentBatchCiphertexts = nil
		}
	}

	fmt.Printf("所有特征数据发送完成 (总样本数: %d, 总特征数: %d)\n", totalSamples, totalFeaturesToProcess)
	return nil
}

// encryptAndSendLabels 加密并发送标签数据
func (p *Participant) encryptAndSendLabels(encoder *ckks.Encoder, encryptor *rlwe.Encryptor, targetID int, slots int) error {
	fmt.Printf("向参与方 %d 发送标签数据...\n", targetID)

	totalSamples := len(p.Labels)

	fmt.Printf("开始加密 %d 个样本的标签数据\n", totalSamples)

	// 计算需要多少个批次来处理所有标签
	// 每个槽存储一个标签值
	batchCount := (totalSamples + slots - 1) / slots // 向上取整

	fmt.Printf("需要 %d 个批次来处理所有标签数据 (每批次 %d 个槽)\n", batchCount, slots)

	// 流式发送：每20个批次发送一次
	batchSize := 20
	totalBatches := (batchCount + batchSize - 1) / batchSize

	fmt.Printf("将分 %d 次发送，每次发送 %d 个批次\n", totalBatches, batchSize)

	// 当前批次的密文列表
	var currentBatchCiphertexts []string

	// 按批次处理所有标签
	for batchIndex := 0; batchIndex < batchCount; batchIndex++ {
		startSampleIndex := batchIndex * slots
		endSampleIndex := startSampleIndex + slots
		if endSampleIndex > totalSamples {
			endSampleIndex = totalSamples
		}

		// 准备当前批次的数据
		batchData := make([]complex128, slots)
		labelsInThisBatch := 0

		for i := 0; i < slots; i++ {
			sampleIndex := startSampleIndex + i
			if sampleIndex < totalSamples {
				// 将标签转换为复数
				label := p.Labels[sampleIndex]
				batchData[i] = complex(float64(label), 0)
				labelsInThisBatch++
			} else {
				batchData[i] = complex(0, 0) // 填充0
			}
		}

		// 编码
		pt := ckks.NewPlaintext(p.KeyManager.GetParams(), p.KeyManager.GetParams().MaxLevel())
		if err := encoder.Encode(batchData, pt); err != nil {
			return fmt.Errorf("编码标签数据批次 %d 失败: %v", batchIndex, err)
		}

		// 加密
		ct, err := encryptor.EncryptNew(pt)
		if err != nil {
			return fmt.Errorf("加密标签数据批次 %d 失败: %v", batchIndex, err)
		}

		// 序列化密文
		ctBytes, err := utils.EncodeShare(ct)
		if err != nil {
			return fmt.Errorf("序列化标签密文批次 %d 失败: %v", batchIndex, err)
		}

		// 添加到当前发送批次
		currentBatchCiphertexts = append(currentBatchCiphertexts, utils.EncodeToBase64(ctBytes))

		// 每10个批次输出一次进度，减少日志输出
		if (batchIndex+1)%10 == 0 || batchIndex == batchCount-1 {
			fmt.Printf("标签数据加密进度: %d/%d 批次完成 (%.1f%%)\n",
				batchIndex+1, batchCount, float64(batchIndex+1)/float64(batchCount)*100)
		}

		// 检查是否需要发送当前批次
		if len(currentBatchCiphertexts) >= batchSize || batchIndex == batchCount-1 {
			sendBatchIndex := (batchIndex / batchSize) + 1
			fmt.Printf("发送标签数据批次 %d/%d (包含 %d 个密文)...\n", sendBatchIndex, totalBatches, len(currentBatchCiphertexts))

			// 构造消息
			message := DataMessage{
				Type:      "label_batch",
				From:      p.ID,
				Data:      utils.EncodeToBase64([]byte(fmt.Sprintf("%d,%d", sendBatchIndex, totalBatches))), // 发送批次信息
				BatchData: currentBatchCiphertexts,                                                          // 当前批次的密文数据
			}

			// 发送消息
			messageJSON, err := json.Marshal(message)
			if err != nil {
				return fmt.Errorf("序列化标签消息批次 %d 失败: %v", sendBatchIndex, err)
			}

			if err := p.SendMessageToParticipant(targetID, string(messageJSON)); err != nil {
				return fmt.Errorf("发送标签数据批次 %d 到参与方 %d 失败: %v", sendBatchIndex, targetID, err)
			}

			fmt.Printf("标签数据批次 %d/%d 发送完成\n", sendBatchIndex, totalBatches)

			// 清空当前批次
			currentBatchCiphertexts = nil
		}
	}

	fmt.Printf("所有标签数据发送完成 (总样本数: %d)\n", totalSamples)
	return nil
}
