use std::time::Duration;

async fn fetch_data(url: &str) -> Result<String, Box<dyn std::error::Error>> {
    let body = reqwest::get(url).await?.text().await?;
    Ok(body)
}

async fn process() {
    let result = fetch_data("https://example.com").await;
    match result {
        Ok(data) => println!("Got: {}", data.len()),
        Err(e) => eprintln!("Error: {e}"),
    }
}
