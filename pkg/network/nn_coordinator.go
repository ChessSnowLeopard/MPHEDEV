package network

import (
	"MPHEDev/pkg/participant"
	"fmt"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// NNCoordinator 神经网络协调器，负责初始化和管理分布式神经网络
type NNCoordinator struct {
	Params     ckks.Parameters        // 同态加密参数
	PublicKey  *rlwe.PublicKey        // 多方公钥
	SecretKey  *rlwe.SecretKey        // 聚合私钥（仅用于验证）
	NNParties  []*participant.NNParty // 神经网络参与方列表
	User       *participant.User      // 数据用户
	InputSize  int                    // 输入层维度
	OutputSize int                    // 输出层维度（类别数）
	HiddenSize int                    // 隐藏层节点数
	NumParties int                    // 参与方数量
}

// LayerConfig 层配置结构体
type LayerConfig struct {
	InputSize  int
	OutputSize int
	Activation participant.NNActivationFunc
	InitMethod participant.WeightInitMethod
}

// NetworkConfig 网络配置结构体
type NetworkConfig struct {
	InputSize   int   // 输入层维度
	OutputSize  int   // 输出层维度（分类数）
	HiddenSize  int   // 隐藏层节点数
	NumParties  int   // 参与方数量
	RandomSeed  int64 // 随机种子
	EnableDebug bool  // 是否启用调试输出
}

// NewNNCoordinator 创建新的神经网络协调器
func NewNNCoordinator(
	params ckks.Parameters,
	pk *rlwe.PublicKey,
	sk *rlwe.SecretKey,
	parties []*participant.Party,
	config NetworkConfig,
) *NNCoordinator {

	coordinator := &NNCoordinator{
		Params:     params,
		PublicKey:  pk,
		SecretKey:  sk,
		InputSize:  config.InputSize,
		OutputSize: config.OutputSize,
		HiddenSize: config.HiddenSize,
		NumParties: config.NumParties,
	}

	if config.EnableDebug {
		fmt.Println("\n======================================================")
		fmt.Println("     开始初始化多方神经网络组件")
		fmt.Println("======================================================")

		fmt.Printf("网络结构: 输入层维度(%d) -> %d个隐藏层(每层%d个节点) -> 输出层维度(%d)\n",
			config.InputSize, config.NumParties-1, config.HiddenSize, config.OutputSize)
	}

	// 初始化数据用户
	coordinator.initializeUser(config.EnableDebug)

	// 初始化神经网络参与方
	coordinator.initializeNNParties(parties, config)

	// 建立参与方连接
	coordinator.establishConnections(config.EnableDebug)

	if config.EnableDebug {
		coordinator.printNetworkStructure()
	}

	return coordinator
}

// initializeUser 初始化数据用户
func (coord *NNCoordinator) initializeUser(enableDebug bool) {
	if enableDebug {
		fmt.Println("\n初始化数据用户...")
	}

	coord.User = participant.NewUser(coord.Params, coord.PublicKey, coord.SecretKey)

	if enableDebug {
		fmt.Printf("用户初始化完成，支持输入维度: %d\n", coord.InputSize)
	}
}

// generateLayerConfigs 动态生成层配置
func (coord *NNCoordinator) generateLayerConfigs() []LayerConfig {
	layerConfigs := make([]LayerConfig, coord.NumParties)

	for i := 0; i < coord.NumParties; i++ {
		if i == 0 {
			// 第一层：输入层 -> 第一个隐藏层
			layerConfigs[i] = LayerConfig{
				InputSize:  coord.InputSize,
				OutputSize: coord.HiddenSize,
				Activation: participant.ActivationReLU,
				InitMethod: participant.InitHe,
			}
		} else if i == coord.NumParties-1 {
			// 最后一层：最后隐藏层 -> 输出层
			layerConfigs[i] = LayerConfig{
				InputSize:  coord.HiddenSize,
				OutputSize: coord.OutputSize,
				Activation: participant.ActivationSigmoid,
				InitMethod: participant.InitXavier,
			}
		} else {
			// 中间层：隐藏层 -> 隐藏层
			layerConfigs[i] = LayerConfig{
				InputSize:  coord.HiddenSize,
				OutputSize: coord.HiddenSize,
				Activation: participant.ActivationReLU,
				InitMethod: participant.InitHe,
			}
		}
	}

	return layerConfigs
}

// initializeNNParties 初始化神经网络参与方
func (coord *NNCoordinator) initializeNNParties(parties []*participant.Party, config NetworkConfig) {
	// 生成层配置
	layerConfigs := coord.generateLayerConfigs()

	if config.EnableDebug {
		fmt.Println("\n预期神经网络层配置:")
		for i, layerConfig := range layerConfigs {
			fmt.Printf("第%d层: %dx%d, 激活函数: %v, 初始化方法: %v\n",
				i+1, layerConfig.InputSize, layerConfig.OutputSize,
				layerConfig.Activation, layerConfig.InitMethod)
		}

		fmt.Println("\n初始化神经网络参与方...")
	}

	// 初始化NNParty对象
	coord.NNParties = make([]*participant.NNParty, coord.NumParties)
	for i := 0; i < coord.NumParties; i++ {
		layerConfig := layerConfigs[i]
		coord.NNParties[i] = participant.NewNNParty(parties[i])
		coord.NNParties[i].InitializeLayer(
			layerConfig.InputSize,
			layerConfig.OutputSize,
			layerConfig.Activation,
			layerConfig.InitMethod,
		)
		coord.NNParties[i].SetLayerIndex(i)
	}
}

// establishConnections 建立参与方之间的连接
func (coord *NNCoordinator) establishConnections(enableDebug bool) {
	if enableDebug {
		fmt.Println("\n建立参与方之间的连接...")
	}

	for i := 0; i < len(coord.NNParties); i++ {
		// 设置前向连接
		if i < len(coord.NNParties)-1 {
			coord.NNParties[i].SetNextParty(coord.NNParties[i+1])
		}

		// 设置后向连接
		if i > 0 {
			coord.NNParties[i].SetPrevParty(coord.NNParties[i-1])
		}
	}
}

// printNetworkStructure 打印网络结构信息
func (coord *NNCoordinator) printNetworkStructure() {
	fmt.Println("\n======================================================")
	fmt.Println("分布式神经网络结构:")
	fmt.Println("======================================================")

	// 打印用户信息
	fmt.Printf("用户: 数据输入端，输入维度 = %d\n", coord.InputSize)
	fmt.Println("  ↓ (加密数据传输)")

	// 打印各层信息
	for i, nnParty := range coord.NNParties {
		fmt.Printf("%s\n", nnParty.GetLayerInfo())

		// 打印连接信息
		connections := []string{}
		if nnParty.PrevParty != nil {
			connections = append(connections, fmt.Sprintf("前驱: 参与方%d", nnParty.PrevParty.ID))
		} else {
			connections = append(connections, "前驱: 用户输入")
		}

		if nnParty.NextParty != nil {
			connections = append(connections, fmt.Sprintf("后继: 参与方%d", nnParty.NextParty.ID))
		} else {
			connections = append(connections, "后继: 输出结果")
		}

		fmt.Printf("  连接: %s\n", fmt.Sprintf("%s, %s", connections[0], connections[1]))

		if i < len(coord.NNParties)-1 {
			fmt.Println("  ↓ (同态加密计算)")
		}
	}

	fmt.Println("  ↓ (多方解密)")
	fmt.Printf("输出: %d个类别的预测概率\n", coord.OutputSize)

	fmt.Println("\n======================================================")
	fmt.Printf("网络初始化完成！参与方数量: %d, 总层数: %d\n", coord.NumParties, coord.NumParties)
	fmt.Println("======================================================")
}

// GetNNParties 获取神经网络参与方列表
func (coord *NNCoordinator) GetNNParties() []*participant.NNParty {
	return coord.NNParties
}

// GetUser 获取数据用户
func (coord *NNCoordinator) GetUser() *participant.User {
	return coord.User
}

// GetNetworkInfo 获取网络基本信息
func (coord *NNCoordinator) GetNetworkInfo() (int, int, int, int) {
	return coord.InputSize, coord.OutputSize, coord.HiddenSize, coord.NumParties
}

// ValidateNetwork 验证网络结构的正确性
func (coord *NNCoordinator) ValidateNetwork() error {
	if len(coord.NNParties) != coord.NumParties {
		return fmt.Errorf("参与方数量不匹配: 期望 %d, 实际 %d", coord.NumParties, len(coord.NNParties))
	}

	// 验证连接的完整性
	for i, party := range coord.NNParties {
		if i > 0 && party.PrevParty == nil {
			return fmt.Errorf("参与方 %d 缺少前驱连接", party.ID)
		}
		if i < len(coord.NNParties)-1 && party.NextParty == nil {
			return fmt.Errorf("参与方 %d 缺少后继连接", party.ID)
		}
	}

	return nil
}
