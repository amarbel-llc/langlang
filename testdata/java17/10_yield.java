class A {
    void m(int x) {
        int y = 0;
        switch (x) {
            case 1:
                yield 10;
            default:
                yield 0;
        }
    }
}

