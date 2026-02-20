class LocalInterface {
    void method() {
        interface Checker {
            void check(int x);
        }

        record Range(long min, long max) {}

        Checker c = x -> System.out.println(x);
        c.check(42);
    }
}
