# tl;dr

**Note**: This project requires Nix for hermetic cross-compilation toolchains. Install Nix before development:

```bash
# macOS/Linux
curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh -s -- install
```

# Backend

```bash
cd backend
set +a; source .env; set -a
go run ./cmd/server
```

# Frontend

```bash
cd frontend
VITE_API_URL=http://localhost:8080 bun run dev
```

# Protos

## Frontend

```bash
cd backend
protoc --go_out=./gen --go_opt=paths=source_relative ../proto/news.proto
```

## Backend

```bash
cd frontend
# TODO: add for each proto
bun run pbjs -t static-module -w es6 -o src/proto/news_pb.js ../proto/news.proto
bun run pbts -o src/proto/news_pb.d.ts src/proto/news_pb.js
cp ../proto/*.proto public/proto
```
