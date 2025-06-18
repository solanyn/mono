#!/usr/bin/env python3
"""PyTorch utility functions for the torches application."""

import torch
import torch.nn as nn
import torch.nn.functional as F
import numpy as np


class SimpleNet(nn.Module):
    """A simple neural network for demonstration."""
    
    def __init__(self, input_size=784, hidden_size=128, num_classes=10):
        super(SimpleNet, self).__init__()
        self.fc1 = nn.Linear(input_size, hidden_size)
        self.fc2 = nn.Linear(hidden_size, hidden_size)
        self.fc3 = nn.Linear(hidden_size, num_classes)
        self.dropout = nn.Dropout(0.2)
        
    def forward(self, x):
        x = torch.flatten(x, 1)
        x = F.relu(self.fc1(x))
        x = self.dropout(x)
        x = F.relu(self.fc2(x))
        x = self.dropout(x)
        x = self.fc3(x)
        return x


def create_sample_data(batch_size=32, input_size=784, num_classes=10):
    """Create sample data for testing."""
    # Generate random input data
    inputs = torch.randn(batch_size, input_size)
    # Generate random labels
    labels = torch.randint(0, num_classes, (batch_size,))
    return inputs, labels


def demonstrate_pytorch():
    """Demonstrate basic PyTorch functionality."""
    print("PyTorch version:", torch.__version__)
    print("CUDA available:", torch.cuda.is_available())
    
    # Create model
    model = SimpleNet()
    print(f"Model created with {sum(p.numel() for p in model.parameters())} parameters")
    
    # Create sample data
    inputs, labels = create_sample_data()
    print(f"Sample data shape: {inputs.shape}")
    
    # Forward pass
    with torch.no_grad():
        outputs = model(inputs)
        print(f"Output shape: {outputs.shape}")
        
    # Compute loss
    criterion = nn.CrossEntropyLoss()
    loss = criterion(outputs, labels)
    print(f"Sample loss: {loss.item():.4f}")
    
    return model, inputs, labels


def train_step(model, inputs, labels, optimizer, criterion):
    """Perform a single training step."""
    optimizer.zero_grad()
    outputs = model(inputs)
    loss = criterion(outputs, labels)
    loss.backward()
    optimizer.step()
    return loss.item()


if __name__ == "__main__":
    demonstrate_pytorch()