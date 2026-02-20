class LabeledAssert {
    void test() {
        // Labeled statement
        outer:
        for (int i = 0; i < 10; i++) {
            inner:
            for (int j = 0; j < 10; j++) {
                if (j == 5) break outer;
                if (j == 3) continue inner;
            }
        }

        // Assert statements
        assert true;
        assert 1 > 0 : "one is greater than zero";
    }
}

