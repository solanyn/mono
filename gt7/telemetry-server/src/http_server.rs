use axum::{Router, routing::get, extract::ws::{WebSocket, WebSocketUpgrade, Message}, response::Response};
use futures_util::{SinkExt, StreamExt};
use tokio::sync::broadcast;
use log::{info, error};
use std::process;

async fn health_check_handler() -> &'static str {
    "OK"
}

async fn websocket_handler(
    ws: WebSocketUpgrade,
    axum::extract::State(tx): axum::extract::State<broadcast::Sender<String>>,
) -> Response {
    ws.on_upgrade(|socket| websocket_connection(socket, tx))
}

async fn websocket_connection(socket: WebSocket, tx: broadcast::Sender<String>) {
    let mut rx = tx.subscribe();
    let (mut sender, mut receiver) = socket.split();
    
    let mut send_task = tokio::spawn(async move {
        while let Ok(msg) = rx.recv().await {
            if sender.send(Message::Text(msg.into())).await.is_err() {
                break;
            }
        }
    });
    
    let mut recv_task = tokio::spawn(async move {
        while let Some(msg) = receiver.next().await {
            if let Ok(Message::Close(_)) = msg {
                break;
            }
        }
    });
    
    tokio::select! {
        _ = (&mut send_task) => {
            recv_task.abort();
        },
        _ = (&mut recv_task) => {
            send_task.abort();
        }
    }
}

pub async fn run_http_server(http_bind_address: String, ws_tx: broadcast::Sender<String>) {
    let app = Router::new()
        .route("/healthz", get(health_check_handler))
        .route("/ws", get(websocket_handler))
        .with_state(ws_tx);

    info!("HTTP server listening on {}", http_bind_address);

    let listener = match tokio::net::TcpListener::bind(&http_bind_address).await {
        Ok(l) => l,
        Err(e) => {
            error!("Failed to bind HTTP server to {}: {}", http_bind_address, e);
            process::exit(1);
        }
    };

    if let Err(e) = axum::serve(listener, app).await {
        error!("HTTP server error: {}", e);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::{
        body::Body,
        http::{Request, StatusCode},
    };
    use http_body_util::BodyExt;
    use tower::util::ServiceExt;
    use tokio_tungstenite::{connect_async, tungstenite::Message};
    use std::time::Duration;

    fn test_app() -> Router {
        Router::new().route("/healthz", get(health_check_handler))
    }

    #[tokio::test]
    async fn health_check_works() {
        let app = test_app();

        let response = app
            .oneshot(
                Request::builder()
                    .uri("/healthz")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();

        assert_eq!(response.status(), StatusCode::OK);

        let body = response.into_body().collect().await.unwrap().to_bytes();
        assert_eq!(&body[..], b"OK");
    }

    #[tokio::test]
    async fn websocket_connection_test() {
        let (ws_tx, _ws_rx) = broadcast::channel::<String>(100);
        let app = Router::new()
            .route("/ws", get(websocket_handler))
            .with_state(ws_tx.clone());
        
        let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        
        tokio::spawn(async move {
            axum::serve(listener, app).await.unwrap();
        });
        
        tokio::time::sleep(Duration::from_millis(100)).await;
        
        let ws_url = format!("ws://127.0.0.1:{}/ws", addr.port());
        let (ws_stream, _) = connect_async(&ws_url).await.unwrap();
        let (mut write, mut read) = ws_stream.split();
        
        let test_packet = gt7_telemetry_server::packet::Packet::default();
        let test_json = serde_json::to_string(&test_packet).unwrap();
        
        ws_tx.send(test_json.clone()).unwrap();
        
        let timeout = tokio::time::timeout(Duration::from_secs(1), read.next()).await;
        match timeout {
            Ok(Some(Ok(Message::Text(received)))) => {
                let received_packet: gt7_telemetry_server::packet::Packet = 
                    serde_json::from_str(&received).unwrap();
                assert_eq!(received_packet, test_packet);
            }
            _ => panic!("Did not receive expected WebSocket message"),
        }
        
        write.close().await.unwrap();
    }
}