#!/usr/bin/env python3
"""PyTorch utility functions and models for the Torches application."""

import torch
import torch.nn as nn
import torch.nn.functional as F


class SimpleNet(nn.Module):
    """A simple neural network for demonstration purposes."""
    
    def __init__(self, input_size=784, hidden_size=128, num_classes=10):
        """Initialize the network.
        
        Args:
            input_size: Size of input features (default: 784 for flattened 28x28 images)
            hidden_size: Size of hidden layer
            num_classes: Number of output classes
        """
        super(SimpleNet, self).__init__()
        self.fc1 = nn.Linear(input_size, hidden_size)
        self.fc2 = nn.Linear(hidden_size, hidden_size)
        self.fc3 = nn.Linear(hidden_size, num_classes)
        self.dropout = nn.Dropout(0.2)
        
    def forward(self, x):
        """Forward pass through the network."""
        # Flatten input if needed
        if len(x.shape) > 2:
            x = x.view(x.size(0), -1)
            
        x = F.relu(self.fc1(x))
        x = self.dropout(x)
        x = F.relu(self.fc2(x))
        x = self.dropout(x)
        x = self.fc3(x)
        return x


def create_sample_data(batch_size=32, input_size=784, num_classes=10):
    """Create random sample data for testing.
    
    Args:
        batch_size: Number of samples in the batch
        input_size: Size of each input sample
        num_classes: Number of classes for labels
        
    Returns:
        Tuple of (inputs, labels) tensors
    """
    # Create random input data
    inputs = torch.randn(batch_size, input_size)
    
    # Create random labels
    labels = torch.randint(0, num_classes, (batch_size,))
    
    return inputs, labels


def train_step(model, inputs, labels, optimizer, criterion):
    """Perform a single training step.
    
    Args:
        model: The neural network model
        inputs: Input tensor
        labels: Target labels tensor
        optimizer: Optimizer for training
        criterion: Loss function
        
    Returns:
        Loss value for this step
    """
    # Zero gradients
    optimizer.zero_grad()
    
    # Forward pass
    outputs = model(inputs)
    
    # Compute loss
    loss = criterion(outputs, labels)
    
    # Backward pass
    loss.backward()
    
    # Update parameters
    optimizer.step()
    
    return loss.item()


def demonstrate_pytorch():
    """Demonstrate basic PyTorch functionality.
    
    Returns:
        Tuple of (model, sample_inputs, sample_labels)
    """
    print("PyTorch version:", torch.__version__)
    print("CUDA available:", torch.cuda.is_available())
    if torch.cuda.is_available():
        print("CUDA devices:", torch.cuda.device_count())
        print("Current CUDA device:", torch.cuda.current_device())
        print("CUDA device name:", torch.cuda.get_device_name())
    
    # Create a simple model
    model = SimpleNet()
    print("Model created successfully")
    print("Model architecture:")
    print(model)
    
    # Create sample data
    sample_inputs, sample_labels = create_sample_data(batch_size=4)
    print(f"Sample input shape: {sample_inputs.shape}")
    print(f"Sample labels shape: {sample_labels.shape}")
    
    # Test forward pass
    with torch.no_grad():
        outputs = model(sample_inputs)
        print(f"Model output shape: {outputs.shape}")
    
    return model, sample_inputs, sample_labels