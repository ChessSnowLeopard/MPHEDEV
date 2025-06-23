import torch

print(f"PyTorch 版本: {torch.__version__}")
print(f"GPU 是否可用: {'可用' if torch.cuda.is_available() else '不可用'}")