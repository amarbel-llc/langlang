fn longest<'a>(x: &'a str, y: &'a str) -> &'a str {
    if x.len() > y.len() {
        x
    } else {
        y
    }
}

struct Important<'a> {
    content: &'a str,
}

impl<'a> Important<'a> {
    fn level(&self) -> usize {
        self.content.len()
    }
}
