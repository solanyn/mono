import jax
import jax.numpy as jnp


def main():
    print("JAX Import Test")
    print("=" * 30)

    print(f"JAX version: {jax.__version__}")
    print(f"JAXlib version: {jax.lib.__version__}")
    print(f"Default backend: {jax.default_backend()}")

    # Simple computation test
    x = jnp.array([1.0, 2.0, 3.0])
    y = jnp.array([4.0, 5.0, 6.0])
    result = x + y

    print(f"Simple computation: {x} + {y} = {result}")
    print("JAX import and basic computation successful!")


if __name__ == "__main__":
    main()
