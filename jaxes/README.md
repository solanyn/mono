# jaxes

A library and/or CLI for training models using jax and friends. Since Bazel and pip don't play nice, we use `rules_uv` to create a uv venv for dependency resolution. Source code is built and run outside of Bazel.

## Development

```bash
bazel run //jaxes:venv

cd jaxes
uv run python jaxes/app.py
```

## Container

```bash
bazel build //jaxes:image
```

