package network

import (
	"gonum.org/v1/gonum/mat"
	"math"
	"math/rand"
)

/*
该文件包含神经网络层的封装和该层的前向传播
*/

// 定义一个Layer层，注意这样的话我们就不需要再去单独封装神经元
// 此外如果这样定义的话我们不需要单独封装输入层，直接将数据输入到第一个隐藏层
type Layer struct {
	InputSize            int
	OutputSize           int
	Weights              *mat.Dense    //该层的权重矩阵的大小就为OutputSize*InputSiZe
	Biases               *mat.VecDense //偏置向量
	Activation           func(*mat.VecDense) *mat.VecDense
	ActivationDerivative func(*mat.VecDense) *mat.VecDense
}

func NewLayer(inputSize int, outputSize int, activation func(*mat.VecDense) *mat.VecDense, activationDeriv func(*mat.VecDense) *mat.VecDense) *Layer {
	weights := mat.NewDense(outputSize, inputSize, nil)
	scale := math.Sqrt(2.0 / float64(inputSize)) //ReLu激活函数利用HE初始化
	//scale := math.Sqrt(2.0 / float64(inputSize+outputSize)) //Sigmoid函数利用Xavier初始化
	for i := 0; i < outputSize; i++ {
		for j := 0; j < inputSize; j++ {
			weights.Set(i, j, rand.NormFloat64()*scale)
		}
	}
	biases := mat.NewVecDense(outputSize, nil)
	return &Layer{
		InputSize:            inputSize,
		OutputSize:           outputSize,
		Weights:              weights,
		Biases:               biases,
		Activation:           activation,
		ActivationDerivative: activationDeriv,
	}
}

func (l *Layer) Forward(x *mat.VecDense) *mat.VecDense {
	var z mat.VecDense
	z.MulVec(l.Weights, x)
	z.AddVec(&z, l.Biases)
	return l.Activation(&z)
}
