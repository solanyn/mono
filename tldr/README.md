# tl;dr

An RSS aggregator, summariser and content recommender. Uses miniflux as the RSS collector and LLM services to summarise content.

## Development

```bash
# Frontend development server
bazel run //tldr/frontend:dev

# Backend server
bazel run //tldr/backend:tldr-backend

# Build all
bazel build //tldr/...
```

## Usage

Frontend runs on `http://localhost:5173`, backend on `http://localhost:8080`
