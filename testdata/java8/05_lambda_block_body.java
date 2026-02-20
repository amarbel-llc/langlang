class A {
    void m() {
        foo(() -> {
            int x = 1;
            int y = 2;
            return x + y;
        });
    }
}

