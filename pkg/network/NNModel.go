package network

import (
	"fmt"
)

/*
该文件包含整个神经网络的初始化方法
*/

type NeuronNetwork struct {
	Layers []*Layer
}

func NewNeuronNetwork(layerSize []int) *NeuronNetwork {
	//存储网络中每层的切片
	layers := make([]*Layer, len(layerSize)-1)
	for i, _ := range layers {
		fmt.Println(len(layerSize)-1, i)
		if i == len(layers)-1 {
			layers[i] = NewLayer(layerSize[i], layerSize[i+1], Softmax, nil)
		} else {
			layers[i] = NewLayer(layerSize[i], layerSize[i+1], ReLU, ReLUDerivative)
			//layers[i] = NewLayer(layerSize[i], layerSize[i+1], Sigmoid, SigmoidDerivative)
		}
	}
	for _, layer := range layers {
		fmt.Println(layer)
	}

	return &NeuronNetwork{layers}
}
