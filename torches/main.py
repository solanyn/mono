import torch
import torch.nn as nn
import numpy as np


def main():
    print("PyTorch Import Test")
    print("=" * 30)

    print(f"PyTorch version: {torch.__version__}")
    print(f"CUDA available: {torch.cuda.is_available()}")
    if torch.cuda.is_available():
        print(f"CUDA device count: {torch.cuda.device_count()}")
        print(f"Current device: {torch.cuda.current_device()}")
        print(f"Device name: {torch.cuda.get_device_name()}")

    # Simple tensor operations
    print(
        f"\nDevice being used: {torch.device('cuda' if torch.cuda.is_available() else 'cpu')}"
    )
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")

    x = torch.tensor([1.0, 2.0, 3.0], device=device)
    y = torch.tensor([4.0, 5.0, 6.0], device=device)
    result = x + y

    print(f"Simple computation: {x.cpu()} + {y.cpu()} = {result.cpu()}")

    # Simple neural network
    class SimpleNet(nn.Module):
        def __init__(self):
            super().__init__()
            self.linear = nn.Linear(3, 1)

        def forward(self, x):
            return self.linear(x)

    model = SimpleNet().to(device)
    input_tensor = torch.randn(1, 3, device=device)
    output = model(input_tensor)

    print(f"Neural network output: {output.item():.4f}")
    print("PyTorch import and basic operations successful!")


if __name__ == "__main__":
    main()

