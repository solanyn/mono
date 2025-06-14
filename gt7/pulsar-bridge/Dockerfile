# Stage 1: Build the application
FROM rust:bullseye AS builder
WORKDIR /usr/src/app

COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo "fn main() {println!(\"Building dependencies...\");}" > src/main.rs && \
    cargo build --release --locked

COPY src ./src
# Ensure src/main.rs or any lib code changes trigger a rebuild of the application
RUN touch src/main.rs && cargo build --release --locked

# Strip the binary to reduce size
RUN strip /usr/src/app/target/release/gt7-pulsar-bridge

# Stage 2: Create the runtime image
FROM gcr.io/distroless/cc-debian12

COPY --from=builder /usr/src/app/target/release/gt7-pulsar-bridge /gt7-pulsar-bridge

EXPOSE 33740/udp
EXPOSE 8080/tcp

ENTRYPOINT ["/gt7-pulsar-bridge"]
