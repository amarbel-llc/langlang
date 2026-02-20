fn apply<F: Fn(i32) -> i32>(f: F, x: i32) -> i32 {
    f(x)
}

fn main() {
    let double = |x| x * 2;
    let add_three = |x: i32| -> i32 { x + 3 };
    let result = apply(double, 5);
    let nums: Vec<i32> = (0..10).filter(|&x| x % 2 == 0).collect();
    let captured = 42;
    let closure = move || println!("{captured}");
}
