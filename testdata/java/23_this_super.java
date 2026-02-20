class Base {
    int x;
    Base(int x) { this.x = x; }
}

class Derived extends Base {
    int y;
    Derived(int x, int y) {
        super(x);
        this.y = y;
    }
    int getSum() { return super.x + this.y; }
}

