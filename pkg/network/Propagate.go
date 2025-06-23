package network

import (
	"fmt"
	"gonum.org/v1/gonum/mat"
	"math"
	"math/rand"
)

/*
该文件包含了网络的前向传播和后向传播
此外还有一些辅助函数，例如准确度计算，预测，损失计算等
*/
// 整个网络的前向传播
func (nn *NeuronNetwork) FeedForward(input *mat.VecDense) *mat.VecDense {
	a := input
	for _, layer := range nn.Layers {
		a = layer.Forward(a)
	}
	return a
}

// 保存梯度信息的结构体
type Gradients struct {
	// 每一层的权重梯度
	WeightGrads []*mat.Dense
	// 每一层的偏置梯度
	BiasGrads []*mat.VecDense
}

// 创建新的梯度结构体
func NewGradients(nn *NeuronNetwork) *Gradients {
	weightGrads := make([]*mat.Dense, len(nn.Layers))
	biasGrads := make([]*mat.VecDense, len(nn.Layers))
	//fmt.Println(len(nn.Layers))
	for i, layer := range nn.Layers {
		weightGrads[i] = mat.NewDense(layer.OutputSize, layer.InputSize, nil)
		biasGrads[i] = mat.NewVecDense(layer.OutputSize, nil)
	}

	return &Gradients{
		WeightGrads: weightGrads,
		BiasGrads:   biasGrads,
	}
}
func (g *Gradients) String() string {
	var s string
	s += "权重梯度:\n"
	for i, wg := range g.WeightGrads {
		s += fmt.Sprintf("第 %d 层:\n%v\n", i, mat.Formatted(wg, mat.Prefix("  "), mat.Squeeze()))
	}
	s += "偏置梯度:\n"
	for i, bg := range g.BiasGrads {
		s += fmt.Sprintf("第 %d 层:\n%v\n", i, mat.Formatted(bg, mat.Prefix("  "), mat.Squeeze()))
	}
	return s
}

// 计算单个样本的梯度（但不更新参数）
func (nn *NeuronNetwork) CalculateGradients(x *mat.VecDense, y *mat.VecDense) *Gradients {
	// 创建梯度结构体
	grads := NewGradients(nn)
	activations := make([]*mat.VecDense, len(nn.Layers)+1)
	preActivations := make([]*mat.VecDense, len(nn.Layers))
	// 初始输入是第一个激活值
	activations[0] = x
	// 前向传播，保存中间结果
	for i, layer := range nn.Layers {
		// 计算前激活值 z = Wx + b
		z := mat.NewVecDense(layer.OutputSize, nil)
		z.MulVec(layer.Weights, activations[i])
		z.AddVec(z, layer.Biases)
		preActivations[i] = z

		// 计算激活值 a = activation(z)
		activations[i+1] = layer.Activation(z)
	}

	// 计算输出层的误差（这里假设使用交叉熵损失函数）
	// delta = output - target
	delta := mat.NewVecDense(activations[len(activations)-1].Len(), nil)
	delta.SubVec(activations[len(activations)-1], y)

	// 从后向前传播误差
	for i := len(nn.Layers) - 1; i >= 0; i-- {
		layer := nn.Layers[i]

		// 计算权重梯度 dW = delta * a^T
		dW := grads.WeightGrads[i]
		for j := 0; j < layer.OutputSize; j++ {
			for k := 0; k < layer.InputSize; k++ {
				dW.Set(j, k, delta.AtVec(j)*activations[i].AtVec(k))
			}
		}

		// 计算偏置梯度 db = delta
		db := grads.BiasGrads[i]
		for j := 0; j < layer.OutputSize; j++ {
			db.SetVec(j, delta.AtVec(j))
		}

		// 如果不是第一层，则计算前一层的delta
		if i > 0 {
			// 计算前一层的delta：delta = (W^T * delta) ⊙ σ'(z)
			prevDelta := mat.NewVecDense(layer.InputSize, nil)

			// W^T * delta
			for j := 0; j < layer.InputSize; j++ {
				sum := 0.0
				for k := 0; k < layer.OutputSize; k++ {
					sum += layer.Weights.At(k, j) * delta.AtVec(k)
				}
				prevDelta.SetVec(j, sum)
			}

			// 应用激活函数的导数
			if i > 0 && nn.Layers[i-1].ActivationDerivative != nil {
				// 获取前一层输出的激活函数导数
				activationDeriv := nn.Layers[i-1].ActivationDerivative(activations[i])
				// 应用链式法则：prevDelta = prevDelta ⊙ σ'(z)(这个表示激活函数的导数)
				for j := 0; j < prevDelta.Len(); j++ {
					prevDelta.SetVec(j, prevDelta.AtVec(j)*activationDeriv.AtVec(j))
				}
			}

			// 更新delta为前一层的delta
			delta = prevDelta
		}
	}
	//fmt.Println("单个样本的梯度", grads)
	return grads
}

// 累加梯度
func AddGradients(accumGrads *Gradients, grads *Gradients) {
	for i := 0; i < len(accumGrads.WeightGrads); i++ {
		// 累加权重梯度
		for j := 0; j < accumGrads.WeightGrads[i].RawMatrix().Rows; j++ {
			for k := 0; k < accumGrads.WeightGrads[i].RawMatrix().Cols; k++ {
				accumGrads.WeightGrads[i].Set(j, k,
					accumGrads.WeightGrads[i].At(j, k)+grads.WeightGrads[i].At(j, k))
			}
		}

		// 累加偏置梯度
		for j := 0; j < accumGrads.BiasGrads[i].Len(); j++ {
			accumGrads.BiasGrads[i].SetVec(j,
				accumGrads.BiasGrads[i].AtVec(j)+grads.BiasGrads[i].AtVec(j))
		}
	}
}

// 计算一个batch的累积梯度
func (nn *NeuronNetwork) CalculateBatchGradients(inputs []*mat.VecDense, targets []*mat.VecDense) *Gradients {
	// 创建累积梯度结构体
	accumGrads := NewGradients(nn)

	// 对批次中的每个样本计算梯度并累加
	for i := 0; i < len(inputs); i++ {
		sampleGrads := nn.CalculateGradients(inputs[i], targets[i])
		AddGradients(accumGrads, sampleGrads)
	}

	return accumGrads
}

// 使用累积梯度更新参数
func (nn *NeuronNetwork) UpdateParameters(grads *Gradients, learningRate float64, batchSize int) {
	for i, layer := range nn.Layers {
		// 更新权重
		for j := 0; j < layer.OutputSize; j++ {
			for k := 0; k < layer.InputSize; k++ {
				// 计算平均梯度
				avgGrad := grads.WeightGrads[i].At(j, k) / float64(batchSize)
				// 更新权重
				currentWeight := layer.Weights.At(j, k)
				layer.Weights.Set(j, k, currentWeight-learningRate*avgGrad)
			}
		}

		// 更新偏置
		for j := 0; j < layer.OutputSize; j++ {
			// 计算平均梯度
			avgGrad := grads.BiasGrads[i].AtVec(j) / float64(batchSize)
			// 更新偏置
			currentBias := layer.Biases.AtVec(j)
			layer.Biases.SetVec(j, currentBias-learningRate*avgGrad)
		}
	}
}

// CalculateBatchLoss 计算一个批次的平均交叉熵损失
func (nn *NeuronNetwork) CalculateBatchLoss(inputs []*mat.VecDense, targets []*mat.VecDense) float64 {
	totalLoss := 0.0
	for i := 0; i < len(inputs); i++ {
		output := nn.FeedForward(inputs[i])
		target := targets[i]
		// 交叉熵损失
		loss := 0.0
		for j := 0; j < output.Len(); j++ {
			// 防止log(0)
			outputVal := output.AtVec(j)
			if outputVal < 1e-10 {
				outputVal = 1e-10
			}
			loss -= target.AtVec(j) * math.Log(outputVal)
		}

		totalLoss += loss
	}

	return totalLoss / float64(len(inputs))
}

// 计算整个数据集的交叉熵损失，直接将整个数据集输入
func (nn *NeuronNetwork) CalculateLoss(inputs []*mat.VecDense, targets []*mat.VecDense) float64 {
	return nn.CalculateBatchLoss(inputs, targets)
}

// 预测样本的类别
func (nn *NeuronNetwork) Predict(input *mat.VecDense) int {
	output := nn.FeedForward(input)

	// 找出概率最大的类别
	maxProb := output.AtVec(0)
	maxIdx := 0

	for i := 1; i < output.Len(); i++ {
		if output.AtVec(i) > maxProb {
			maxProb = output.AtVec(i)
			maxIdx = i
		}
	}

	return maxIdx
}

// 评估模型在测试集上的准确率
func (nn *NeuronNetwork) Evaluate(inputs []*mat.VecDense, targets []*mat.VecDense) float64 {
	correct := 0

	for i := 0; i < len(inputs); i++ {
		pred := nn.Predict(inputs[i])

		// 找出目标one-hot向量中的1所在位置
		actual := 0
		for j := 0; j < targets[i].Len(); j++ {
			if targets[i].AtVec(j) > 0.5 {
				actual = j
				break
			}
		}

		if pred == actual {
			correct++
		}
	}

	return float64(correct) / float64(len(inputs))
}

// 使用 Mini-batch SGD训练神经网络
func (nn *NeuronNetwork) Train(inputs []*mat.VecDense, targets []*mat.VecDense, batchSize int, learningRate float64, epochs int) {
	numSamples := len(inputs) //训练样本数

	for epoch := 0; epoch < epochs; epoch++ {
		totalLoss := 0.0
		// 遍历每个mini-batch
		for i := 0; i < numSamples; i += batchSize {
			end := i + batchSize
			if end > numSamples {
				end = numSamples
			}

			// 获取当前批次的输入和目标
			batchInputs := inputs[i:end]
			batchTargets := targets[i:end]

			// 计算这个批次的累积梯度
			batchGradients := nn.CalculateBatchGradients(batchInputs, batchTargets)

			// 使用累积梯度一次性更新模型参数
			nn.UpdateParameters(batchGradients, learningRate, end-i)

			// 计算当前批次的损失（用于监控训练进度）
			batchLoss := nn.CalculateBatchLoss(batchInputs, batchTargets)
			totalLoss += batchLoss * float64(end-i)
		}

		// 计算平均损失
		avgLoss := totalLoss / float64(numSamples)

		fmt.Printf("第 %d 轮训练 - 平均损失: %.4f\n", epoch+1, avgLoss)

	}
}

// TrainWithDP 使用差分隐私SGD训练神经网络
func (nn *NeuronNetwork) TrainWithDP(inputs []*mat.VecDense, targets []*mat.VecDense, dpConfig *DPSGDConfig, epochs int) []float64 {
	numSamples := len(inputs) // 训练样本数

	//// 定义RDP阶数，用于隐私分析
	//orders := make([]float64, 0)
	//for i := 1; i < 100; i++ {
	//	orders = append(orders, 1.0+float64(i)/10.0)
	//}
	//for i := 11; i <= 64; i++ {
	//	orders = append(orders, float64(i))
	//}
	//orders = append(orders, 128.0, 256.0, 512.0)

	//// 计算采样率
	//samplingRate := float64(dpConfig.BatchSize) / float64(numSamples)

	lossHistory := make([]float64, epochs)

	// 开始训练
	totalSteps := 0
	for epoch := 0; epoch < epochs; epoch++ {
		totalLoss := 0.0

		// 随机打乱数据
		shuffledIndices := shuffleIndices(numSamples)

		// 遍历每个mini-batch
		for i := 0; i < numSamples; i += dpConfig.BatchSize {
			end := i + dpConfig.BatchSize
			if end > numSamples {
				end = numSamples
			}

			// 获取当前批次的索引
			batchIndices := shuffledIndices[i:end]
			batchSize := len(batchIndices)

			// 收集当前批次的输入和目标
			batchInputs := make([]*mat.VecDense, batchSize)
			batchTargets := make([]*mat.VecDense, batchSize)
			for j, idx := range batchIndices {
				batchInputs[j] = inputs[idx]
				batchTargets[j] = targets[idx]
			}

			// 为每个样本计算梯度并裁剪
			accumulatedGrads := NewGradients(nn)
			for j := 0; j < batchSize; j++ {
				// 计算单个样本的梯度
				sampleGrads := nn.CalculateGradients(batchInputs[j], batchTargets[j])
				// 裁剪梯度
				ClipGradientByL2Norm(sampleGrads, dpConfig.L2NormClip)
				// 累加梯度
				AddGradients(accumulatedGrads, sampleGrads)
				//fmt.Println(accumulatedGrads)
			}

			//fmt.Println("添加噪声前的累积梯度", accumulatedGrads.BiasGrads[2])

			// 添加噪声到累加的梯度
			AddGaussianNoise(accumulatedGrads, dpConfig.NoiseMultiplier, dpConfig.L2NormClip, dpConfig.Seed)

			//fmt.Println("添加噪声后的累积梯度", accumulatedGrads.BiasGrads[2])

			// 使用带噪声的梯度更新参数
			nn.UpdateParameters(accumulatedGrads, dpConfig.LearningRate, batchSize)

			// 计算当前批次的损失
			batchLoss := nn.CalculateBatchLoss(batchInputs, batchTargets)
			totalLoss += batchLoss * float64(batchSize)

			totalSteps++
		}

		// 计算平均损失
		avgLoss := totalLoss / float64(numSamples)
		lossHistory[epoch] = avgLoss

		//// 计算当前的隐私预算
		//rdpValues := ComputeRDP(samplingRate, dpConfig.NoiseMultiplier, totalSteps, orders)
		//eps, optOrder := ConvertRDPtoDP(orders, rdpValues, dpConfig.Delta)

		fmt.Printf("第 %d 轮训练 - 平均损失: %.4f\n",
			epoch+1, avgLoss)
		//fmt.Printf("第 %d 轮训练", epoch+1)
	}

	//训练结束，计算最终隐私预算
	//rdpValues := ComputeRDP(samplingRate, dpConfig.NoiseMultiplier, totalSteps, orders)
	//eps, optOrder := ConvertRDPtoDP(orders, rdpValues, dpConfig.Delta)
	//fmt.Printf("训练完成 - 总步数: %d, 最终隐私预算: ε=%.4f (α=%.1f, δ=%.0e)\n",
	//	totalSteps, eps, optOrder, dpConfig.Delta)

	return lossHistory
}

// 辅助函数：打乱索引顺序
func shuffleIndices(length int) []int {
	indices := make([]int, length)
	for i := 0; i < length; i++ {
		indices[i] = i
	}

	// Fisher-Yates 洗牌算法
	for i := length - 1; i > 0; i-- {
		j := int(math.Floor(rand.Float64() * float64(i+1)))
		indices[i], indices[j] = indices[j], indices[i]
	}

	return indices
}
