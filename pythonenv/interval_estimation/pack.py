__all__ = ['add']


def add(a: int, b: int) -> int:
    
    return a + b

def minus(a, b):
    
    return a - b

class MyClass(object):
    z = 10
    def __init__(self, x):
        ...
        self.x = x

    def my_method(self, y):

        return y + self.z
    
    @classmethod
    def my_classmethod(cls, x):
        return x + cls.z
    
    @staticmethod
    def my_staticmethod(x):
        return x * 2
'''
...
'''

my_class = MyClass(20)
my_class.my_method(30)

tmp = MyClass.z


# '''
# Input: Ciphertexts {ct.zj}0≤j≤d, a polynomial g(x), and number of iterations IterNum 1: for j = 0, 1, . . . , d do 2: ct.betaj ← 0 3: end for  4: for i = 1, 2, . . . , IterNum do 5: ct.ip ← RS(∑d  j=0 Mult(ct.betaj, ct.zj); p) 6: ct.g ← PolyEval(−ct.ip, bp · g(x)e) 7: for j = 0, 1, . . . , d do  8: ct.gradj ← RS(Mult(ct.g, ct.zj); p)  9: ct.gradj ← RS(AllSum(ct.gradj); b n  α e) 10: ct.betaj ← Add(ct.betaj, ct.gradj) 11: end for 12: end for  13: return (ct.beta0, . . . , ct.betad).

# '''
def secure_logistic_regression(ct, g, IterNum, p, b, n, α, e):
    for j in range(d):
        ct.betaj = 0
    for i in range(1, IterNum+1):
        # ...
        ct.ip = RS(sum([ct.betaj * ct.zj for j in range(d)], p))
        ct.g = PolyEval(-ct.ip, bp * g(x) * e)
        for j in range(d):
            ct.gradj = RS(ct.g * ct.zj, p)
            ct.gradj = RS(AllSum(ct.gradj, b, n, α, e))
            ct.betaj = Add(ct.betaj, ct.gradj)
    # return (ct.beta0, . . . , ct.betad)