/// Adds the positive values and reports how many there were.
pub fn sum_positive(values: &[i32]) -> (i32, usize) {
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

/// sum_positive with the names and the comparison changed.
pub fn sum_negative(numbers: &[i32]) -> (i32, usize) {
    let mut sum = 0;
    let mut seen = 0;
    for number in numbers {
        if *number < 0 {
            sum += number;
            seen += 1;
        }
    }
    println!("{} negative values", seen);
    (sum, seen)
}

pub struct Server;

impl Server {
    /// Two loops, a match with three arms, an if with && and a ? make 8.
    pub fn handle(&self, items: &[i32], n: i32, r: Result<i32, ()>) -> Result<i32, ()> {
        let mut acc = 0;
        for i in 0..10 {
            acc += i;
        }
        while acc > 100 {
            acc -= 1;
        }
        let bonus = match n {
            1 => 10,
            2 | 3 => 20,
            _ => 0,
        };
        if acc > 5 && n > 0 {
            acc += bonus;
        }
        let v = r?;
        Ok(acc + v + items.len() as i32)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Another copy of sum_positive, which test code excludes from clones.
    #[test]
    fn sums_positive_values() {
        let values = [1, -2, 3];
        let mut total = 0;
        let mut count = 0;
        for value in values.iter() {
            if *value > 0 {
                total += value;
                count += 1;
            }
        }
        assert_eq!((total, count), (4, 2));
        assert_eq!(sum_positive(&values), (4, 2));
    }
}
