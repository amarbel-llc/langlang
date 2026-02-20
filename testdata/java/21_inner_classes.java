class Outer {
    int x;

    class Inner {
        int y;
        void test() { x = 1; }
    }

    static class StaticInner {
        int z;
    }

    void test() {
        Inner i = new Inner();
    }
}

