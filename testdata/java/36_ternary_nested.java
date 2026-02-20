class TernaryNested {
    void test() {
        int a = 1;
        int b = 2;
        int c = 3;
        int max = a > b ? (a > c ? a : c) : (b > c ? b : c);
        String s = a > 0 ? "positive" : a < 0 ? "negative" : "zero";
    }
}

