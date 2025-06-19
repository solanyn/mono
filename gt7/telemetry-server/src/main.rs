use env_logger::Env;
use gt7_telemetry_core::config::Config;
use log::error;
use std::process;
use std::sync::Arc;
use tokio::runtime::Runtime;
use tokio::sync::broadcast;

mod http_server;
mod pulsar_handler;
mod telemetry_loop;

use http_server::run_http_server;
use pulsar_handler::PulsarHandler;
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

    let pulsar_handler = match PulsarHandler::new(
        config.pulsar_service_url.clone(),
        config.pulsar_topic.clone(),
        Arc::clone(&runtime),
    ) {
        Ok(handler) => handler,
        Err(e) => {
            error!("Failed to initialize PulsarHandler: {}", e);
            process::exit(1);
        }
    };

    let http_bind_address = config.http_bind_address();
    let ws_tx_clone = ws_tx.clone();
    runtime.spawn(run_http_server(http_bind_address, ws_tx_clone));

    run_telemetry_loop(config, ws_tx, pulsar_handler);
}
