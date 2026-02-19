function classify(n) {
  if (n > 10) {
    return "big";
  } else if (n > 0) {
    return "small";
  } else {
    return "zero-or-negative";
  }
}
