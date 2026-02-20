class Box<T> {
    T value;
    Box(T value) { this.value = value; }
    T get() { return value; }
    <U> void set(U item) {}
}

class Pair<A, B> {
    A first;
    B second;
}

class Bounded<T extends Comparable<T>> {
    T min(T a, T b) {
        return a;
    }
}

