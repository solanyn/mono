#!/usr/bin/env python3
"""
Demo script showing platform-specific PyTorch dependencies.
This will use different PyTorch builds depending on the target platform.
"""

import torch
import platform
import sys

def main():
    print(f"Python version: {sys.version}")
    print(f"Platform: {platform.platform()}")
    print(f"Architecture: {platform.machine()}")
    print(f"PyTorch version: {torch.__version__}")
    
    # Check CUDA availability
    if torch.cuda.is_available():
        print(f"CUDA available: Yes")
        print(f"CUDA version: {torch.version.cuda}")
        print(f"GPU count: {torch.cuda.device_count()}")
        for i in range(torch.cuda.device_count()):
            print(f"GPU {i}: {torch.cuda.get_device_name(i)}")
    else:
        print("CUDA available: No")
    
    # Test tensor creation
    x = torch.randn(3, 3)
    print(f"Created tensor: {x.shape}")
    
    if torch.cuda.is_available():
        x_gpu = x.cuda()
        print(f"Moved tensor to GPU: {x_gpu.device}")

if __name__ == "__main__":
    main()