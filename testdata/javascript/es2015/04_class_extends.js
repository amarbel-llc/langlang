class Base {
  value() {
    return 1;
  }
}

class Child extends Base {
  value() {
    return this.n || 0;
  }
}
