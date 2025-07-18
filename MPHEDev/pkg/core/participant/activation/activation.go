package activation

import (
	"MPHEDev/pkg/core/participant/interfaces"
	"fmt"
	"math"
	"math/big"

	"github.com/tuneinsight/lattigo/v6/circuits/ckks/polynomial"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
	"github.com/tuneinsight/lattigo/v6/utils/bignum"
)

// IdentityActivation 恒等激活函数（不做任何处理）
type IdentityActivation struct{}

func (ia *IdentityActivation) Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	return ct, nil
}

func (ia *IdentityActivation) GetName() string {
	return "Identity"
}

// SigmoidChebyshevActivation 使用Chebyshev多项式近似的Sigmoid激活函数
type SigmoidChebyshevActivation struct {
	K        float64
	Degree   int
	poly     bignum.Polynomial
	polyEval *polynomial.Evaluator
	init     bool
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

	// Chebyshev多项式在[-1,1]区间拟合sigmoid(x*K)
	sigmoidK := func(x float64) float64 {
		return 1 / (math.Exp(-x*sca.K) + 1)
	}

	sca.poly = getChebyshevPoly(1.0, sca.Degree, sigmoidK)
	sca.polyEval = polynomial.NewEvaluator(params, evaluator)
	sca.init = true
}

// Apply 应用Chebyshev多项式近似Sigmoid
func (sca *SigmoidChebyshevActivation) Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	sca.Initialize(params, evaluator)

	ctResult := ct.CopyNew()
	targetScale := params.DefaultScale()

	// 1. 归一化输入到[-1,1]（除以K）
	scaleDown := 1.0 / sca.K
	if err := evaluator.MulRelin(ctResult, scaleDown, ctResult); err != nil {
		return nil, err
	}
	if ctResult.Level() > 0 && ctResult.Scale.Cmp(targetScale) > 0 {
		if err := evaluator.Rescale(ctResult, ctResult); err != nil {
			return nil, err
		}
	}
	if ctResult.Scale.Cmp(targetScale) != 0 {
		_ = evaluator.SetScale(ctResult, targetScale) // 忽略错误
	}

	// 2. 评估Chebyshev多项式
	poly := polynomial.NewPolynomial(sca.poly)
	result, err := sca.polyEval.Evaluate(ctResult, poly, targetScale)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (sca *SigmoidChebyshevActivation) GetName() string {
	return fmt.Sprintf("SigmoidChebyshev(K=%.1f, degree=%d)", sca.K, sca.Degree)
}

// ReLUActivation ReLU激活函数（使用多项式近似）
type ReLUActivation struct {
	K        float64
	Degree   int
	poly     bignum.Polynomial
	polyEval *polynomial.Evaluator
	init     bool
}

// NewReLUActivation 创建ReLU激活函数
func NewReLUActivation(K float64, degree int) *ReLUActivation {
	return &ReLUActivation{
		K:      K,
		Degree: degree,
		init:   false,
	}
}

// Initialize 初始化多项式和评估器
func (ra *ReLUActivation) Initialize(params ckks.Parameters, evaluator *ckks.Evaluator) {
	if ra.init {
		return
	}

	// ReLU函数：max(0, x)
	reluFunc := func(x float64) float64 {
		if x > 0 {
			return x
		}
		return 0
	}

	ra.poly = getChebyshevPoly(ra.K, ra.Degree, reluFunc)
	ra.polyEval = polynomial.NewEvaluator(params, evaluator)
	ra.init = true
}

// Apply 应用ReLU激活函数
func (ra *ReLUActivation) Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	ra.Initialize(params, evaluator)

	ctResult := ct.CopyNew()
	targetScale := params.DefaultScale()

	// 评估多项式
	poly := polynomial.NewPolynomial(ra.poly)
	result, err := ra.polyEval.Evaluate(ctResult, poly, targetScale)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (ra *ReLUActivation) GetName() string {
	return fmt.Sprintf("ReLU(K=%.1f, degree=%d)", ra.K, ra.Degree)
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
	result, err := eca.polyEval.Evaluate(ctResult, poly, targetScale)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (eca *ExpChebyshevActivation) GetName() string {
	return fmt.Sprintf("ExpChebyshev(K=%.1f, degree=%d)", eca.K, eca.Degree)
}

// ReciprocalChebyshevActivation 使用Chebyshev多项式近似的倒数函数
type ReciprocalChebyshevActivation struct {
	K        float64
	Epsilon  float64
	Degree   int
	poly     bignum.Polynomial
	polyEval *polynomial.Evaluator
	init     bool
}

// NewReciprocalChebyshevActivation 创建Chebyshev多项式近似的倒数激活
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

	// 倒数函数：1/x，避免x=0
	recipFunc := func(x float64) float64 {
		if math.Abs(x) < rca.Epsilon {
			return 1.0 / rca.Epsilon
		}
		return 1.0 / x
	}

	rca.poly = getChebyshevPolyWithEps(rca.K, rca.Epsilon, rca.Degree, recipFunc)
	rca.polyEval = polynomial.NewEvaluator(params, evaluator)
	rca.init = true
}

// Apply 应用Chebyshev多项式近似倒数函数
func (rca *ReciprocalChebyshevActivation) Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	rca.Initialize(params, evaluator)

	ctResult := ct.CopyNew()
	targetScale := params.DefaultScale()

	// 评估多项式
	poly := polynomial.NewPolynomial(rca.poly)
	result, err := rca.polyEval.Evaluate(ctResult, poly, targetScale)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (rca *ReciprocalChebyshevActivation) GetName() string {
	return fmt.Sprintf("ReciprocalChebyshev(K=%.1f, eps=%.3f, degree=%d)", rca.K, rca.Epsilon, rca.Degree)
}

// LogarithmChebyshevActivation 使用Chebyshev多项式近似的对数激活函数
type LogarithmChebyshevActivation struct {
	K        float64
	Eps      float64
	Degree   int
	poly     bignum.Polynomial
	polyEval *polynomial.Evaluator
	init     bool
}

// NewLogarithmChebyshevActivation 创建Chebyshev多项式近似的对数激活
func NewLogarithmChebyshevActivation(K, eps float64, degree int) *LogarithmChebyshevActivation {
	return &LogarithmChebyshevActivation{
		K:      K,
		Eps:    eps,
		Degree: degree,
		init:   false,
	}
}

// Initialize 初始化多项式和评估器
func (lca *LogarithmChebyshevActivation) Initialize(params ckks.Parameters, evaluator *ckks.Evaluator) {
	if lca.init {
		return
	}

	// 对数函数
	logFunc := func(x float64) float64 {
		if x <= 0 {
			return math.Log(lca.Eps) // 避免log(0)
		}
		return math.Log(x)
	}

	lca.poly = getChebyshevPolyForLog(lca.K, lca.Eps, lca.Degree, logFunc)
	lca.polyEval = polynomial.NewEvaluator(params, evaluator)
	lca.init = true
}

// Apply 应用Chebyshev多项式近似对数函数
func (lca *LogarithmChebyshevActivation) Apply(ct *rlwe.Ciphertext, evaluator *ckks.Evaluator, params ckks.Parameters) (*rlwe.Ciphertext, error) {
	lca.Initialize(params, evaluator)

	ctResult := ct.CopyNew()
	targetScale := params.DefaultScale()

	// 确保输入密文是度数1（重线性化）
	if ctResult.Degree() > 1 {
		if err := evaluator.Relinearize(ctResult, ctResult); err != nil {
			return nil, fmt.Errorf("输入重线性化失败: %v", err)
		}
	}

	// 创建多项式并评估
	polyLog := polynomial.NewPolynomial(lca.poly)

	// 关键修复：添加ChangeOfBasis变换
	scalar, constant := polyLog.ChangeOfBasis()

	// 应用线性变换：ct = scalar * ct + constant
	if err := evaluator.Mul(ctResult, scalar, ctResult); err != nil {
		return nil, fmt.Errorf("应用scalar变换失败: %v", err)
	}
	if err := evaluator.Add(ctResult, constant, ctResult); err != nil {
		return nil, fmt.Errorf("应用constant变换失败: %v", err)
	}
	if err := evaluator.Rescale(ctResult, ctResult); err != nil {
		return nil, fmt.Errorf("ChangeOfBasis rescale失败: %v", err)
	}

	// 评估多项式
	result, err := lca.polyEval.Evaluate(ctResult, polyLog, targetScale)
	if err != nil {
		return nil, fmt.Errorf("多项式求值失败: %v", err)
	}

	return result, nil
}

func (lca *LogarithmChebyshevActivation) GetName() string {
	return fmt.Sprintf("LogChebyshev(K=%.1f, eps=%.3f, degree=%d)", lca.K, lca.Eps, lca.Degree)
}

// GetActivation 根据名称获取激活函数
func GetActivation(name string) interfaces.ActivationFunction {
	switch name {
	case "sigmoid_chebyshev":
		return NewSigmoidChebyshevActivation(6.0, 20)
	case "relu":
		return NewReLUActivation(5.0, 15)
	case "exp_chebyshev":
		return NewExpChebyshevActivation(10.0, 20)
	case "reciprocal_chebyshev":
		return NewReciprocalChebyshevActivation(100.0, 0.1, 20)
	case "log_chebyshev":
		return NewLogarithmChebyshevActivation(1.0, 0.01, 20)
	case "identity", "":
		return &IdentityActivation{}
	default:
		return &IdentityActivation{}
	}
}

// 辅助函数

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

// getChebyshevPolyForLog 返回对数函数在区间[eps,K]上的Chebyshev多项式近似
func getChebyshevPolyForLog(K, eps float64, degree int, f64 func(x float64) (y float64)) bignum.Polynomial {
	FBig := func(x *big.Float) (y *big.Float) {
		xF64, _ := x.Float64()
		return new(big.Float).SetPrec(x.Prec()).SetFloat64(f64(xF64))
	}

	var prec uint = 128

	// 在区间[eps, K]上进行切比雪夫近似
	interval := bignum.Interval{
		A:     *bignum.NewFloat(eps, prec),
		B:     *bignum.NewFloat(K, prec),
		Nodes: degree,
	}
	return bignum.ChebyshevApproximation(FBig, interval)
}
