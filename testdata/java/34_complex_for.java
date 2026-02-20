class ComplexFor {
    void test() {
        // Multiple init and update expressions
        for (int i = 0, j = 10; i < j; i++, j--) {}

        // Empty for
        for (;;) { break; }

        // Complex condition
        for (int i = 0; i < 10 && i > -1; i++) {}
    }
}

