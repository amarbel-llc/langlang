#![allow(dead_code)]

mod inner {
    pub struct Foo {
        pub x: i32,
        y: i32,
    }

    impl Foo {
        pub fn new(x: i32, y: i32) -> Self {
            Foo { x, y }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::inner::Foo;

    #[test]
    fn test_foo() {
        let f = Foo::new(1, 2);
        assert_eq!(f.x, 1);
    }
}
