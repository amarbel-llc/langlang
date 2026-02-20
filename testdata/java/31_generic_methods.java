class GenericMethods {
    <T> T identity(T t) { return t; }
    <T extends Comparable<T>> T max(T a, T b) { return a; }
    <K, V> void put(K key, V value) {}
}

