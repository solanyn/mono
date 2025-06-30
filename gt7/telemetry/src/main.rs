use env_logger::Env;
use lib::config::Config;
use log::error;
use std::process;
use std::sync::Arc;
use tokio::runtime::Runtime;
use tokio::sync::broadcast;

mod http_server;
mod kafka_handler;
mod telemetry_loop;

use http_server::run_http_server;
use kafka_handler::KafkaHandler;
use telemetry_loop::run_telemetry_loop;

fn main() {
    env_logger::Builder::from_env(Env::default().default_filter_or("info")).init();

    let config = match Config::load() {
        Ok(c) => c,
        Err(e) => {
            error!("Configuration error: {}", e);
            process::exit(1);
        }
    };

    let runtime = Arc::new(Runtime::new().expect("Failed to create Tokio runtime"));
    let (ws_tx, _ws_rx) = broadcast::channel::<String>(1000);

    let kafka_handler = match KafkaHandler::new(
        config.kafka_bootstrap_servers.clone(),
        config.kafka_topic.clone(),
        Arc::clone(&runtime),
    ) {
        Ok(handler) => handler,
        Err(e) => {
            error!("Failed to initialize KafkaHandler: {}", e);
            process::exit(1);
        }
    };

    let http_bind_address = config.http_bind_address();
    let ws_tx_clone = ws_tx.clone();
    runtime.spawn(run_http_server(http_bind_address, ws_tx_clone));

    run_telemetry_loop(config, ws_tx, kafka_handler);
}
