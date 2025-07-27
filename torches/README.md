# torches

A library and/or CLI for training models using pytorch. Since Bazel and pip don't play nice, we use `rules_uv` to create a uv venv for dependency resolution. Source code is built and run outside of Bazel.

## Development

```bash
bazel run //torches:venv

cd torches
uv run python torches/app.py
```

## Container

```bash
bazel build //torches:image
```

