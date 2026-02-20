class ComplexExpressions {
    void test() {
        // Array access
        int[] a = new int[]{1, 2, 3};
        int x = a[0];
        a[1] = 42;

        // Nested method calls
        String s = String.valueOf(42);

        // Conditional expression
        int max = (x > 0) ? x : -x;

        // Complex boolean
        boolean b = (x > 0 && x < 100) || (x == -1);

        // Parenthesized
        int y = (1 + 2) * (3 + 4);

        // String concatenation
        String msg = "val=" + x + " flag=" + b;
    }
}

