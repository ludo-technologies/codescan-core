//! A test module split into its own file, declared as `#[cfg(test)] mod
//! tests;` by its parent. The file name alone marks it as test code.

/// Another copy of sum_positive that must not be reported as a clone.
fn roundtrip(values: &[i32]) -> (i32, usize) {
    let mut total = 0;
    let mut count = 0;
    for value in values {
        if *value > 0 {
            total += value;
            count += 1;
        }
    }
    println!("{} positive values", count);
    (total, count)
}
