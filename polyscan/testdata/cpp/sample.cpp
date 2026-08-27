#include <cstdio>
#include <utility>

// Adds the positive values and reports how many there were.
std::pair<int, int> sumPositive(const int* values, int n) {
  int total = 0;
  int count = 0;
  for (int i = 0; i < n; i++) {
    if (values[i] > 0) {
      total += values[i];
      count++;
    }
  }
  std::printf("%d positive values\n", count);
  return {total, count};
}

// sumPositive with the names and the comparison changed.
std::pair<int, int> sumNegative(const int* numbers, int n) {
  int sum = 0;
  int seen = 0;
  for (int i = 0; i < n; i++) {
    if (numbers[i] < 0) {
      sum += numbers[i];
      seen++;
    }
  }
  std::printf("%d negative values\n", seen);
  return {sum, seen};
}

class Server {
 public:
  // Two loops, two switch cases, an if, its &&, a ternary and a catch are
  // eight decision points, so the complexity is 9.
  int handle(const int* items, int n, int mode) const {
    int acc = 0;
    for (int i = 0; i < 10; i++) {
      acc += i;
    }
    while (acc > 100) {
      acc--;
    }
    switch (mode) {
      case 1:
        acc += 10;
        break;
      case 2:
        acc += 20;
        break;
      default:
        break;
    }
    if (acc > 5 && n > 0) {
      acc += items[0];
    }
    try {
      acc += n > 0 ? n : -n;
    } catch (...) {
      acc = 0;
    }
    return acc;
  }
};
