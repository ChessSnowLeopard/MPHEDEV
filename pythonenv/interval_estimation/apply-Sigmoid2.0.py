import numpy as np
import matplotlib.pyplot as plt
from numpy.polynomial import Polynomial
from scipy.integrate import quad
from scipy.optimize import least_squares

def sigmoid(x):
    return 1 / (1 + np.exp(-x))

def taylor_series(x, degree=9):
    """Taylor series approximation of sigmoid function"""
    coefficients = [
        0.5,          # 0th order
        0.25,         # 1st order
        0,             # 2nd order
        -1/48,         # 3rd order
        0,             # 4th order
        1/480,         # 5th order
        0,             # 6th order
        -17/80640,     # 7th order
        0,             # 8th order
        31/1451520     # 9th order
    ]
    return sum(coeff * x**i for i, coeff in enumerate(coefficients[:degree+1]))

def least_squares_coefficients(degree, interval=(-8, 8)):
    """Calculate least squares coefficients for sigmoid approximation"""
    # Define the basis functions (x/8)^k
    def basis(x, k):
        return (x/8)**k
    
    # We'll solve the normal equations A^T A c = A^T b
    # Where A is the design matrix, c are coefficients, b is sigmoid(x)
    
    # We'll use numerical integration to compute the inner products
    n = degree + 1
    ATA = np.zeros((n, n))
    ATb = np.zeros(n)
    
    for i in range(n):
        def integrand_i(x):
            return basis(x, i) * sigmoid(x)
        ATb[i] = quad(integrand_i, interval[0], interval[1])[0]
        
        for j in range(n):
            def integrand_ij(x):
                return basis(x, i) * basis(x, j)
            ATA[i,j] = quad(integrand_ij, interval[0], interval[1])[0]
    
    # Solve the system of equations
    coefficients = np.linalg.solve(ATA, ATb)
    return coefficients

# Coefficients from the paper (for verification)
paper_coeff_3 = np.array([0.5, 1.20096, 0, -0.81562])
paper_coeff_7 = np.array([0.5, 1.73496, 0, -4.19407, 0, 5.43402, 0, -2.50739])

# Calculate our coefficients
calc_coeff_3 = least_squares_coefficients(3)
calc_coeff_7 = least_squares_coefficients(7)

print("Degree 3 coefficients:")
print("Paper:", paper_coeff_3)
print("Calculated:", calc_coeff_3)
print("\nDegree 7 coefficients:")
print("Paper:", paper_coeff_7)
print("Calculated:", calc_coeff_7)

# Create polynomial functions
def g3_paper(x):
    return 0.5 + 1.20096*(x/8) - 0.81562*(x/8)**3

def g7_paper(x):
    return 0.5 + 1.73496*(x/8) - 4.19407*(x/8)**3 + 5.43402*(x/8)**5 - 2.50739*(x/8)**7

def g3_calc(x):
    return calc_coeff_3[0] + calc_coeff_3[1]*(x/8) + calc_coeff_3[3]*(x/8)**3

def g7_calc(x):
    return (calc_coeff_7[0] + calc_coeff_7[1]*(x/8) + calc_coeff_7[3]*(x/8)**3 + 
            calc_coeff_7[5]*(x/8)**5 + calc_coeff_7[7]*(x/8)**7)

# Plotting functions
def plot_comparison():
    x = np.linspace(-8, 8, 1000)
    
    plt.figure(figsize=(12, 8))
    
    # Plot sigmoid
    plt.plot(x, sigmoid(x), label='Sigmoid', linewidth=3, color='black')
    
    # Plot Taylor series
    plt.plot(x, taylor_series(x, 9), label='Taylor (degree 9)', linestyle='--')
    
    # Plot least squares approximations
    plt.plot(x, g3_paper(x), label='LS degree 3 (paper)', linestyle='-.')
    plt.plot(x, g7_paper(x), label='LS degree 7 (paper)', linestyle=':')
    
    # Plot our calculated approximations
    plt.plot(x, g3_calc(x), label='LS degree 3 (calculated)', linestyle='-.', alpha=0.5)
    plt.plot(x, g7_calc(x), label='LS degree 7 (calculated)', linestyle=':', alpha=0.5)
    
    plt.title('Comparison of Sigmoid Approximations')
    plt.xlabel('x')
    plt.ylabel('σ(x)')
    plt.legend()
    plt.grid(True)
    plt.show()

def plot_errors():
    x = np.linspace(-8, 8, 1000)
    
    plt.figure(figsize=(12, 8))
    
    # Calculate errors
    error_taylor = taylor_series(x, 9) - sigmoid(x)
    error_g3 = g3_paper(x) - sigmoid(x)
    error_g7 = g7_paper(x) - sigmoid(x)
    
    plt.plot(x, error_taylor, label='Taylor (degree 9) error')
    plt.plot(x, error_g3, label='LS degree 3 error')
    plt.plot(x, error_g7, label='LS degree 7 error')
    
    plt.title('Approximation Errors')
    plt.xlabel('x')
    plt.ylabel('Error')
    plt.legend()
    plt.grid(True)
    plt.show()

def calculate_mse():
    x = np.linspace(-8, 8, 10000)
    
    def mse(y_true, y_pred):
        return np.mean((y_true - y_pred)**2)
    
    mse_taylor = mse(sigmoid(x), taylor_series(x, 9))
    mse_g3 = mse(sigmoid(x), g3_paper(x))
    mse_g7 = mse(sigmoid(x), g7_paper(x))
    
    print("Mean Squared Errors:")
    print(f"Taylor (degree 9): {mse_taylor:.6f}")
    print(f"LS degree 3: {mse_g3:.6f}")
    print(f"LS degree 7: {mse_g7:.6f}")

# Run the analysis
plot_comparison()
plot_errors()
calculate_mse()