#include <cstdio>
#include <utility>

// Another copy of sumPositive, which the test file name excludes from clones.
std::pair<int, int> sumPositiveAgain(const int* values, int n) {
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
