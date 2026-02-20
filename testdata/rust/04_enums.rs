enum Message {
    Quit,
    Move { x: i32, y: i32 },
    Write(String),
    ChangeColor(i32, i32, i32),
}

impl Message {
    fn call(&self) {
        match self {
            Message::Quit => println!("quit"),
            Message::Move { x, y } => println!("move to {x},{y}"),
            Message::Write(text) => println!("{text}"),
            Message::ChangeColor(r, g, b) => println!("color {r},{g},{b}"),
        }
    }
}
