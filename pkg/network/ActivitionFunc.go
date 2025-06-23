package network

import (
	"gonum.org/v1/gonum/mat"
	"math"
)

// sigmoid激活函数（对整个向量的操作）
func Sigmoid(z *mat.VecDense) *mat.VecDense {
	out := mat.NewVecDense(z.Len(), nil)
	for i := 0; i < z.Len(); i++ {
		out.SetVec(i, 1/(1+math.Exp(-z.AtVec(i))))
	}
	return out
}

// sigmoid的导数(标量函数对向量求导)
func SigmoidDerivative(a *mat.VecDense) *mat.VecDense {
	out := mat.NewVecDense(a.Len(), nil)
	for i := 0; i < a.Len(); i++ {
		v := a.AtVec(i)
		out.SetVec(i, v*(1-v))
	}
	return out
}

// ReLU 激活函数
func ReLU(z *mat.VecDense) *mat.VecDense {
	out := mat.NewVecDense(z.Len(), nil)
	for i := 0; i < z.Len(); i++ {
		val := z.AtVec(i)
		if val > 0 {
			out.SetVec(i, val)
		} else {
			out.SetVec(i, 0)
		}
	}
	return out
}

// ReLU 的导数函数
func ReLUDerivative(a *mat.VecDense) *mat.VecDense {
	out := mat.NewVecDense(a.Len(), nil)
	for i := 0; i < a.Len(); i++ {
		if a.AtVec(i) > 0 {
			out.SetVec(i, 1)
		} else {
			out.SetVec(i, 0)
		}
	}
	return out
}

// softmax函数
func Softmax(z *mat.VecDense) *mat.VecDense {
	sum := 0.0
	for i := 0; i < z.Len(); i++ {
		sum += math.Exp(z.AtVec(i))
	}
	out := mat.NewVecDense(z.Len(), nil)
	for i := 0; i < z.Len(); i++ {
		out.SetVec(i, math.Exp(z.AtVec(i))/sum)
	}
	return out
}
