package activation

import (
	"MPHEDev/pkg/core/participant/crypto"
	"MPHEDev/pkg/core/participant/interfaces"
	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// ActivationManager 激活函数管理器
type ActivationManager struct {
	// 角色类型
	Role string // "input", "hidden", "output"

	// 激活函数（根据角色不同而不同）
	ActivationFunction interfaces.ActivationFunction // 隐藏层激活函数
	SoftmaxProcessor   interfaces.SoftmaxProcessor   // 输出层Softmax处理器
	LossFunction       interfaces.LossFunction       // 输出层损失函数

	// 配置参数
	NumClasses int // 输出层类别数
	BatchSize  int // 批大小
	Slots      int // 密文槽数
	K          int // 每密文特征数

	// 刷新服务支持
	RefreshService *crypto.RefreshService
	OnlinePeers    map[int]string
	MyID           int
	CommonCRSSeed  []byte
}

// NewActivationManager 创建激活函数管理器
func NewActivationManager(role string, numClasses, batchSize, slots, k int) *ActivationManager {
	manager := &ActivationManager{
		Role:       role,
		NumClasses: numClasses,
		BatchSize:  batchSize,
		Slots:      slots,
		K:          k,
	}

	// 根据角色初始化相应的激活函数
	manager.initializeByRole()

	return manager
}

// initializeByRole 根据角色初始化激活函数
func (am *ActivationManager) initializeByRole() {
	switch am.Role {
	case "input":
		// 输入层：不需要激活函数
		am.ActivationFunction = GetActivation("identity")
		am.SoftmaxProcessor = nil
		am.LossFunction = nil

	case "hidden":
		// 隐藏层：使用ReLU或Sigmoid激活函数
		am.ActivationFunction = GetActivation("relu") // 默认使用ReLU
		am.SoftmaxProcessor = nil
		am.LossFunction = nil

	case "output":
		// 输出层：使用Softmax和交叉熵损失
		am.ActivationFunction = GetActivation("identity") // 输出层通常不使用激活函数
		am.SoftmaxProcessor = NewSoftmaxProcessor()
		am.LossFunction = NewCrossEntropyLoss(am.NumClasses, am.BatchSize, am.Slots, am.K)

	default:
		// 默认：使用恒等激活函数
		am.ActivationFunction = GetActivation("identity")
		am.SoftmaxProcessor = nil
		am.LossFunction = nil
	}
}

// SetActivationFunction 设置激活函数
func (am *ActivationManager) SetActivationFunction(activationName string) {
	am.ActivationFunction = GetActivation(activationName)
}

// SetRole 设置角色
func (am *ActivationManager) SetRole(role string) {
	am.Role = role
	am.initializeByRole()
}

// SetSoftmaxProcessor 设置Softmax处理器
func (am *ActivationManager) SetSoftmaxProcessor(processor interfaces.SoftmaxProcessor) {
	am.SoftmaxProcessor = processor
}

// SetLossFunction 设置损失函数
func (am *ActivationManager) SetLossFunction(lossFunction interfaces.LossFunction) {
	am.LossFunction = lossFunction
}

// SetRefreshService 设置刷新服务
func (am *ActivationManager) SetRefreshService(refreshService interface{}) {
	if rs, ok := refreshService.(*crypto.RefreshService); ok {
		am.RefreshService = rs
	}
}

// SetOnlinePeers 设置在线参与方信息
func (am *ActivationManager) SetOnlinePeers(onlinePeers map[int]string, myID int) {
	am.OnlinePeers = onlinePeers
	am.MyID = myID
}

// SetCommonCRSSeed 设置统一的CRS种子
func (am *ActivationManager) SetCommonCRSSeed(seed []byte) {
	am.CommonCRSSeed = seed
	if am.RefreshService != nil {
		am.RefreshService.SetCommonCRSSeed(seed)
	}
}

// checkAndRefresh 检查密文层级并在需要时触发刷新
func (am *ActivationManager) checkAndRefresh(
	ciphertexts []*rlwe.Ciphertext,
	params ckks.Parameters,
	stepName string,
) ([]*rlwe.Ciphertext, error) {
	if am.RefreshService == nil || len(am.OnlinePeers) == 0 {
		// 如果没有刷新服务或在线参与方，直接返回原密文
		return ciphertexts, nil
	}

	// 设置刷新阈值
	bootstrapThreshold := params.MaxLevel() / 3

	// 检查最低层级
	minLevel := params.MaxLevel()
	for _, ct := range ciphertexts {
		if ct.Level() < minLevel {
			minLevel = ct.Level()
		}
	}

	fmt.Printf("\n🔍 %s层级检查: 最低层级=%d/%d, 阈值=%d\n",
		stepName, minLevel, params.MaxLevel(), bootstrapThreshold)

	if minLevel < bootstrapThreshold {
		fmt.Printf("🔄 触发协同刷新: 层级过低（%d < %d）\n", minLevel, bootstrapThreshold)

		// 使用同步刷新接口
		refreshedCiphertexts, err := am.RefreshService.RefreshCiphertextsSync(
			ciphertexts,
			am.OnlinePeers,
			am.MyID,
			fmt.Sprintf("refresh_%s", stepName),
		)
		if err != nil {
			return nil, fmt.Errorf("协同刷新失败: %v", err)
		}

		fmt.Printf("✅ 协同刷新完成，共刷新 %d 个密文\n", len(refreshedCiphertexts))
		return refreshedCiphertexts, nil
	} else {
		fmt.Printf("✅ 层级充足，无需刷新\n")
		return ciphertexts, nil
	}
}

// ApplySoftmaxWithRefresh 带刷新功能的Softmax计算
func (am *ActivationManager) ApplySoftmaxWithRefresh(
	neuronOutputs []*rlwe.Ciphertext,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
) ([]*rlwe.Ciphertext, error) {
	if am.SoftmaxProcessor == nil {
		return nil, fmt.Errorf("Softmax处理器未初始化")
	}

	// 检查输入层级
	refreshedInputs, err := am.checkAndRefresh(neuronOutputs, params, "Softmax输入")
	if err != nil {
		return nil, err
	}

	// 执行Softmax计算
	softmaxOutputs, err := am.SoftmaxProcessor.ApplySoftmax(refreshedInputs, params, evaluator, encoder)
	if err != nil {
		return nil, err
	}

	// 检查输出层级
	refreshedOutputs, err := am.checkAndRefresh(softmaxOutputs, params, "Softmax输出")
	if err != nil {
		return nil, err
	}

	return refreshedOutputs, nil
}

// ComputeLossWithRefresh 带刷新功能的损失计算
func (am *ActivationManager) ComputeLossWithRefresh(
	softmaxOutputs []*rlwe.Ciphertext,
	labels []int,
	params ckks.Parameters,
	evaluator *ckks.Evaluator,
	encoder *ckks.Encoder,
) (*rlwe.Ciphertext, error) {
	if am.LossFunction == nil {
		return nil, fmt.Errorf("损失函数未初始化")
	}

	// 检查输入层级
	refreshedInputs, err := am.checkAndRefresh(softmaxOutputs, params, "损失函数输入")
	if err != nil {
		return nil, err
	}

	// 执行损失计算
	loss, err := am.LossFunction.ComputeLoss(refreshedInputs, labels, params, evaluator, encoder)
	if err != nil {
		return nil, err
	}

	// 检查输出层级
	refreshedLoss, err := am.checkAndRefresh([]*rlwe.Ciphertext{loss}, params, "损失函数输出")
	if err != nil {
		return nil, err
	}

	return refreshedLoss[0], nil
}

// GetActivationFunction 获取激活函数
func (am *ActivationManager) GetActivationFunction() interfaces.ActivationFunction {
	return am.ActivationFunction
}

// GetSoftmaxProcessor 获取Softmax处理器
func (am *ActivationManager) GetSoftmaxProcessor() interfaces.SoftmaxProcessor {
	return am.SoftmaxProcessor
}

// GetLossFunction 获取损失函数
func (am *ActivationManager) GetLossFunction() interfaces.LossFunction {
	return am.LossFunction
}

// PrintConfiguration 打印配置信息
func (am *ActivationManager) PrintConfiguration() {
	fmt.Printf("\n=== 激活函数管理器配置 ===\n")
	fmt.Printf("角色: %s\n", am.Role)
	fmt.Printf("激活函数: %s\n", am.ActivationFunction.GetName())

	if am.SoftmaxProcessor != nil {
		fmt.Printf("Softmax处理器: %s\n", am.SoftmaxProcessor.GetName())
	} else {
		fmt.Printf("Softmax处理器: 无\n")
	}

	if am.LossFunction != nil {
		fmt.Printf("损失函数: %s\n", am.LossFunction.GetName())
	} else {
		fmt.Printf("损失函数: 无\n")
	}

	fmt.Printf("类别数: %d\n", am.NumClasses)
	fmt.Printf("批大小: %d\n", am.BatchSize)
	fmt.Printf("密文槽数: %d\n", am.Slots)
	fmt.Printf("每密文特征数: %d\n", am.K)

	// 刷新服务状态
	if am.RefreshService != nil {
		fmt.Printf("刷新服务: 已配置 ✅\n")
		if len(am.OnlinePeers) > 0 {
			fmt.Printf("在线参与方: %d个\n", len(am.OnlinePeers))
			fmt.Printf("本机ID: %d\n", am.MyID)
		} else {
			fmt.Printf("在线参与方: 未配置\n")
		}
	} else {
		fmt.Printf("刷新服务: 未配置 ❌\n")
	}

	fmt.Println("==========================")
}

// IsInputLayer 判断是否为输入层
func (am *ActivationManager) IsInputLayer() bool {
	return am.Role == "input"
}

// IsHiddenLayer 判断是否为隐藏层
func (am *ActivationManager) IsHiddenLayer() bool {
	return am.Role == "hidden"
}

// IsOutputLayer 判断是否为输出层
func (am *ActivationManager) IsOutputLayer() bool {
	return am.Role == "output"
}

// HasSoftmax 判断是否有Softmax处理器
func (am *ActivationManager) HasSoftmax() bool {
	return am.SoftmaxProcessor != nil
}

// HasLossFunction 判断是否有损失函数
func (am *ActivationManager) HasLossFunction() bool {
	return am.LossFunction != nil
}
