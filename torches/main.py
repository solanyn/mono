#!/usr/bin/env python3
"""Main PyTorch application demonstrating neural network training."""

import torch
import torch.nn as nn
import torch.optim as optim
from torches.torch_utils import (
    SimpleNet,
    create_sample_data,
    train_step,
    demonstrate_pytorch,
)


def main():
    """Main application entry point."""
    print("=" * 50)
    print("Torches - PyTorch Application with Bazel")
    print("=" * 50)

    # Demonstrate PyTorch basics
    print("\n1. Demonstrating PyTorch functionality:")
    model, sample_inputs, sample_labels = demonstrate_pytorch()

    print("\n2. Training a simple neural network:")

    # Set up training
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    model = model.to(device)

    criterion = nn.CrossEntropyLoss()
    optimizer = optim.Adam(model.parameters(), lr=0.001)

    num_epochs = 10
    batch_size = 32

    print(f"Training on device: {device}")
    print(f"Number of epochs: {num_epochs}")

    # Simple training loop
    model.train()
    for epoch in range(num_epochs):
        # Generate new random data each epoch (for demonstration)
        inputs, labels = create_sample_data(batch_size)
        inputs, labels = inputs.to(device), labels.to(device)

        loss = train_step(model, inputs, labels, optimizer, criterion)

        if (epoch + 1) % 2 == 0:
            print(f"Epoch [{epoch + 1}/{num_epochs}], Loss: {loss:.4f}")

    print("\n3. Model evaluation:")

    # Evaluate model
    model.eval()
    with torch.no_grad():
        test_inputs, test_labels = create_sample_data(batch_size=10)
        test_inputs, test_labels = test_inputs.to(device), test_labels.to(device)

        outputs = model(test_inputs)
        _, predicted = torch.max(outputs.data, 1)

        # Calculate accuracy (on random data, so expect ~10% accuracy)
        accuracy = (predicted == test_labels).float().mean().item()
        print(f"Test accuracy on random data: {accuracy:.2%}")

    print("\n4. Model summary:")
    total_params = sum(p.numel() for p in model.parameters())
    trainable_params = sum(p.numel() for p in model.parameters() if p.requires_grad)
    print(f"Total parameters: {total_params:,}")
    print(f"Trainable parameters: {trainable_params:,}")

    print("\nTraining completed successfully!")
    print("=" * 50)


if __name__ == "__main__":
    main()
