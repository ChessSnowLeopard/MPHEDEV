package homomorphic

import (
	"fmt"
	"math"
	"math/big"

	"github.com/tuneinsight/lattigo/v6/circuits/ckks/polynomial"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/bignum"
)

// ActivationFunction 激活函数接口
type ActivationFunction interface {
	Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error)
	ApplyDerivative(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error)
	GetName() string
}

// IdentityActivation 恒等激活函数（不做任何处理）
type IdentityActivation struct{}

func (ia *IdentityActivation) Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	return ct, nil
}

func (ia *IdentityActivation) ApplyDerivative(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	// 恒等函数的导数是1
	// 创建一个新的密文，内容全为1

	// 创建全1的明文
	slots := ct.Slots()
	ones := make([]complex128, slots)
	for i := range ones {
		ones[i] = complex(1.0, 0)
	}

	// 编码为明文
	encoder := ckks.NewEncoder(params)
	onesPt := ckks.NewPlaintext(params, ct.Level())
	if err := encoder.Encode(ones, onesPt); err != nil {
		return nil, fmt.Errorf("编码常数1失败: %v", err)
	}

	// 创建一个新的密文，内容为1
	// 我们需要创建一个密文，其解密结果是1
	result := ct.CopyNew()

	// 将密文归零（乘以0）
	if err := evaluator.MulRelin(result, 0.0, result); err != nil {
		return nil, fmt.Errorf("清零密文失败: %v", err)
	}

	// 加上常数1
	if err := evaluator.Add(result, onesPt, result); err != nil {
		return nil, fmt.Errorf("添加常数1失败: %v", err)
	}

	return result, nil
}

func (ia *IdentityActivation) GetName() string {
	return "Identity"
}

// SigmoidChebyshevActivation 使用Chebyshev多项式近似的Sigmoid激活函数
// 支持自定义区间K和多项式阶数degree
type SigmoidChebyshevActivation struct {
	K             float64
	Degree        int
	poly          bignum.Polynomial
	polyEval      *polynomial.Evaluator
	derivPoly     bignum.Polynomial
	derivPolyEval *polynomial.Evaluator
	init          bool
}

// NewSigmoidChebyshevActivation 创建Chebyshev多项式近似的Sigmoid激活
func NewSigmoidChebyshevActivation(K float64, degree int) *SigmoidChebyshevActivation {
	return &SigmoidChebyshevActivation{
		K:      K,
		Degree: degree,
		init:   false,
	}
}

// Initialize 初始化多项式和评估器
func (sca *SigmoidChebyshevActivation) Initialize(params ckks.Parameters, evaluator *ckks.Evaluator) {
	if sca.init {
		return
	}

	// 直接在[-K, K]区间拟合sigmoid(x)和其导数 - 按照demo的方式
	sigmoid := func(x float64) float64 {
		return 1 / (math.Exp(-x) + 1)
	}

	// sigmoid导数函数: σ'(x) = σ(x) * (1 - σ(x))
	sigmoidDerivative := func(x float64) float64 {
		sig := sigmoid(x)
		return sig * (1 - sig)
	}

	// 在[-K, K]区间上近似，不做归一化
	sca.poly = getChebyshevPoly(sca.K, sca.Degree, sigmoid)
	sca.derivPoly = getChebyshevPoly(sca.K, sca.Degree, sigmoidDerivative)
	sca.polyEval = polynomial.NewEvaluator(params, evaluator)
	sca.derivPolyEval = polynomial.NewEvaluator(params, evaluator)
	sca.init = true
}

// Apply 应用Chebyshev多项式近似Sigmoid
func (sca *SigmoidChebyshevActivation) Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	sca.Initialize(params, evaluator)

	ctResult := ct.CopyNew()
	targetScale := params.DefaultScale()

	// 按照demo方式：直接在[-K, K]区间评估，不做归一化
	poly := polynomial.NewPolynomial(sca.poly)
	scalar, constant := poly.ChangeOfBasis()

	// 变换基底
	if err := evaluator.MulRelin(ctResult, scalar, ctResult); err != nil {
		return nil, err
	}
	if err := evaluator.Add(ctResult, constant, ctResult); err != nil {
		return nil, err
	}
	if ctResult.Level() > 0 && ctResult.Scale.Cmp(targetScale) > 0 {
		if err := evaluator.Rescale(ctResult, ctResult); err != nil {
			return nil, err
		}
	}

	// 评估多项式
	result, err := sca.polyEval.Evaluate(ctResult, poly, targetScale)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ApplyDerivative 应用Chebyshev多项式近似Sigmoid导数
func (sca *SigmoidChebyshevActivation) ApplyDerivative(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	sca.Initialize(params, evaluator)

	ctResult := ct.CopyNew()
	targetScale := params.DefaultScale()

	// 按照demo方式：直接在[-K, K]区间评估导数，不做归一化
	derivPoly := polynomial.NewPolynomial(sca.derivPoly)
	scalar, constant := derivPoly.ChangeOfBasis()

	// 变换基底
	if err := evaluator.MulRelin(ctResult, scalar, ctResult); err != nil {
		return nil, fmt.Errorf("变换基底失败: %v", err)
	}
	if err := evaluator.Add(ctResult, constant, ctResult); err != nil {
		return nil, fmt.Errorf("添加常数失败: %v", err)
	}
	if ctResult.Level() > 0 && ctResult.Scale.Cmp(targetScale) > 0 {
		if err := evaluator.Rescale(ctResult, ctResult); err != nil {
			return nil, fmt.Errorf("重新缩放失败: %v", err)
		}
	}

	// 评估导数多项式
	result, err := sca.derivPolyEval.Evaluate(ctResult, derivPoly, targetScale)
	if err != nil {
		return nil, fmt.Errorf("评估导数多项式失败: %v", err)
	}

	return result, nil
}

func (sca *SigmoidChebyshevActivation) GetName() string {
	return fmt.Sprintf("SigmoidChebyshev(K=%.1f, degree=%d)", sca.K, sca.Degree)
}

// getChebyshevPoly 返回函数f在区间[-K, K]上的Chebyshev多项式近似
func getChebyshevPoly(K float64, degree int, f64 func(x float64) (y float64)) bignum.Polynomial {
	FBig := func(x *big.Float) (y *big.Float) {
		xF64, _ := x.Float64()
		return new(big.Float).SetPrec(x.Prec()).SetFloat64(f64(xF64))
	}
	var prec uint = 128
	interval := bignum.Interval{
		A:     *bignum.NewFloat(-K, prec),
		B:     *bignum.NewFloat(K, prec),
		Nodes: degree,
	}
	return bignum.ChebyshevApproximation(FBig, interval)
}

// GetActivation 根据名称获取激活函数
func GetActivation(name string) ActivationFunction {
	switch name {
	case "sigmoid_chebyshev":
		return NewSigmoidChebyshevActivation(6.0, 20)
	case "exp_chebyshev":
		return NewExpChebyshevActivation(10.0, 20)
	case "reciprocal_chebyshev":
		return NewReciprocalChebyshevActivation(100.0, 0.1, 20)
	case "identity", "":
		return &IdentityActivation{}
	default:
		return &IdentityActivation{}
	}
}

// ExpChebyshevActivation 使用Chebyshev多项式近似的指数激活函数
type ExpChebyshevActivation struct {
	K        float64
	Degree   int
	poly     bignum.Polynomial
	polyEval *polynomial.Evaluator
	init     bool
}

// NewExpChebyshevActivation 创建Chebyshev多项式近似的指数激活
func NewExpChebyshevActivation(K float64, degree int) *ExpChebyshevActivation {
	return &ExpChebyshevActivation{
		K:      K,
		Degree: degree,
		init:   false,
	}
}

// Initialize 初始化多项式和评估器
func (eca *ExpChebyshevActivation) Initialize(params ckks.Parameters, evaluator *ckks.Evaluator) {
	if eca.init {
		return
	}

	// 指数函数
	expFunc := func(x float64) float64 {
		return math.Exp(x)
	}

	eca.poly = getChebyshevPoly(eca.K, eca.Degree, expFunc)
	eca.polyEval = polynomial.NewEvaluator(params, evaluator)
	eca.init = true
}

// Apply 应用Chebyshev多项式近似指数函数
func (eca *ExpChebyshevActivation) Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	eca.Initialize(params, evaluator)

	ctResult := ct.CopyNew()
	targetScale := params.DefaultScale()

	// 变换基底
	poly := polynomial.NewPolynomial(eca.poly)
	scalar, constant := poly.ChangeOfBasis()
	if err := evaluator.MulRelin(
		ctResult, scalar, ctResult); err != nil {
		return nil, err
	}
	if err := evaluator.Add(ctResult, constant, ctResult); err != nil {
		return nil, err
	}
	if ctResult.Level() > 0 && ctResult.Scale.Cmp(targetScale) > 0 {
		if err := evaluator.Rescale(ctResult, ctResult); err != nil {
			return nil, err
		}
	}

	// 评估多项式
	result, err := eca.polyEval.Evaluate(ctResult, poly, targetScale)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ApplyDerivative 应用指数函数的导数（即自身）
func (eca *ExpChebyshevActivation) ApplyDerivative(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	// 指数函数的导数就是自身：d/dx[e^x] = e^x
	return eca.Apply(ct, evaluator, params)
}

func (eca *ExpChebyshevActivation) GetName() string {
	return fmt.Sprintf("ExpChebyshev(K=%.1f, degree=%d)", eca.K, eca.Degree)
}

// ReciprocalChebyshevActivation 使用Chebyshev多项式近似的倒数函数
type ReciprocalChebyshevActivation struct {
	K             float64
	Epsilon       float64
	Degree        int
	poly          bignum.Polynomial
	polyEval      *polynomial.Evaluator
	derivPoly     bignum.Polynomial
	derivPolyEval *polynomial.Evaluator
	init          bool
}

// NewReciprocalChebyshevActivation 创建Chebyshev多项式近似的倒数函数
func NewReciprocalChebyshevActivation(K, epsilon float64, degree int) *ReciprocalChebyshevActivation {
	return &ReciprocalChebyshevActivation{
		K:       K,
		Epsilon: epsilon,
		Degree:  degree,
		init:    false,
	}
}

// Initialize 初始化多项式和评估器
func (rca *ReciprocalChebyshevActivation) Initialize(params ckks.Parameters, evaluator *ckks.Evaluator) {
	if rca.init {
		return
	}

	// 倒数函数，避免0附近
	reciprocalFunc := func(x float64) float64 {
		if math.Abs(x) < rca.Epsilon {
			if x >= 0 {
				return 1 / rca.Epsilon
			}
			return -1 / rca.Epsilon
		}
		return 1 / x
	}

	// 倒数函数的导数：d/dx[1/x] = -1/x^2
	reciprocalDerivativeFunc := func(x float64) float64 {
		if math.Abs(x) < rca.Epsilon {
			if x >= 0 {
				return -1 / (rca.Epsilon * rca.Epsilon)
			}
			return -1 / (rca.Epsilon * rca.Epsilon)
		}
		return -1 / (x * x)
	}

	// 在[epsilon, K]区间上近似
	rca.poly = getChebyshevPolyWithEps(rca.K, rca.Epsilon, rca.Degree, reciprocalFunc)
	rca.derivPoly = getChebyshevPolyWithEps(rca.K, rca.Epsilon, rca.Degree, reciprocalDerivativeFunc)
	rca.polyEval = polynomial.NewEvaluator(params, evaluator)
	rca.derivPolyEval = polynomial.NewEvaluator(params, evaluator)
	rca.init = true
}

// Apply 应用Chebyshev多项式近似倒数函数
func (rca *ReciprocalChebyshevActivation) Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	rca.Initialize(params, evaluator)

	ctResult := ct.CopyNew()
	targetScale := params.DefaultScale()

	// 变换基底
	polyRcp := polynomial.NewPolynomial(rca.poly)
	scalar, constant := polyRcp.ChangeOfBasis()
	if err := evaluator.MulRelin(ctResult, scalar, ctResult); err != nil {
		return nil, err
	}
	if err := evaluator.Add(ctResult, constant, ctResult); err != nil {
		return nil, err
	}
	if ctResult.Level() > 0 && ctResult.Scale.Cmp(targetScale) > 0 {
		if err := evaluator.Rescale(ctResult, ctResult); err != nil {
			return nil, err
		}
	}

	// 评估多项式
	result, err := rca.polyEval.Evaluate(ctResult, polyRcp, targetScale)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ApplyDerivative 应用倒数函数的导数
func (rca *ReciprocalChebyshevActivation) ApplyDerivative(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	rca.Initialize(params, evaluator)

	ctResult := ct.CopyNew()
	targetScale := params.DefaultScale()

	// 变换基底
	polyRcpDeriv := polynomial.NewPolynomial(rca.derivPoly)
	scalar, constant := polyRcpDeriv.ChangeOfBasis()
	if err := evaluator.MulRelin(ctResult, scalar, ctResult); err != nil {
		return nil, fmt.Errorf("变换基底失败: %v", err)
	}
	if err := evaluator.Add(ctResult, constant, ctResult); err != nil {
		return nil, fmt.Errorf("添加常数失败: %v", err)
	}
	if ctResult.Level() > 0 && ctResult.Scale.Cmp(targetScale) > 0 {
		if err := evaluator.Rescale(ctResult, ctResult); err != nil {
			return nil, fmt.Errorf("重新缩放失败: %v", err)
		}
	}

	// 评估导数多项式
	result, err := rca.derivPolyEval.Evaluate(ctResult, polyRcpDeriv, targetScale)
	if err != nil {
		return nil, fmt.Errorf("评估导数多项式失败: %v", err)
	}
	return result, nil
}

func (rca *ReciprocalChebyshevActivation) GetName() string {
	return fmt.Sprintf("ReciprocalChebyshev(K=%.1f, eps=%.2f, degree=%d)", rca.K, rca.Epsilon, rca.Degree)
}

// getChebyshevPolyWithEps 返回函数f在区间[eps, K]上的Chebyshev多项式近似
func getChebyshevPolyWithEps(K, eps float64, degree int, f64 func(x float64) (y float64)) bignum.Polynomial {
	FBig := func(x *big.Float) (y *big.Float) {
		xF64, _ := x.Float64()
		return new(big.Float).SetPrec(x.Prec()).SetFloat64(f64(xF64))
	}
	var prec uint = 128
	interval := bignum.Interval{
		A:     *bignum.NewFloat(eps, prec),
		B:     *bignum.NewFloat(K, prec),
		Nodes: degree,
	}
	return bignum.ChebyshevApproximation(FBig, interval)
}
