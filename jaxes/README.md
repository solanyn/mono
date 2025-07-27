# jaxes

JAX ML components with platform-specific GPU support.

## Features

- JAX import testing and validation
- Platform-specific GPU acceleration (CUDA/Metal)
- Basic computation and tensor operations
- Compatible with ML workflows

## Development

```bash
# Setup virtual environment
bazelisk run //jaxes:venv

# Run JAX test application
cd jaxes
source .venv/bin/activate
python -m jaxes.app
```

## Usage

Tests JAX installation, version, backend detection, and basic computation:

```python
import jax
import jax.numpy as jnp
```

## Container

```bash
bazelisk build //jaxes:image
```