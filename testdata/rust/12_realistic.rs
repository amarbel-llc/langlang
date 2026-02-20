use std::collections::HashMap;
use std::sync::Arc;

#[derive(Debug, Clone)]
pub struct Config {
    pub name: String,
    pub values: HashMap<String, String>,
    pub enabled: bool,
}

impl Config {
    pub fn new(name: &str) -> Self {
        Config {
            name: name.to_string(),
            values: HashMap::new(),
            enabled: true,
        }
    }

    pub fn get(&self, key: &str) -> Option<&str> {
        self.values.get(key).map(|s| s.as_str())
    }

    pub fn set(&mut self, key: String, value: String) {
        self.values.insert(key, value);
    }
}

pub trait Configurable {
    fn config(&self) -> &Config;
    fn reload(&mut self) -> Result<(), Box<dyn std::error::Error>>;
}

pub struct Service {
    config: Arc<Config>,
    running: bool,
}

impl Service {
    pub fn new(config: Config) -> Self {
        Service {
            config: Arc::new(config),
            running: false,
        }
    }

    pub async fn start(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        self.running = true;
        println!("Service {} started", self.config.name);
        Ok(())
    }

    pub fn is_running(&self) -> bool {
        self.running
    }
}

impl Configurable for Service {
    fn config(&self) -> &Config {
        &self.config
    }

    fn reload(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        println!("Reloading config for {}", self.config.name);
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_config_new() {
        let config = Config::new("test");
        assert_eq!(config.name, "test");
        assert!(config.enabled);
    }

    #[test]
    fn test_config_get_set() {
        let mut config = Config::new("test");
        config.set("key".to_string(), "value".to_string());
        assert_eq!(config.get("key"), Some("value"));
        assert_eq!(config.get("missing"), None);
    }
}
