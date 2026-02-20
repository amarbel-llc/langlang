class Expressions {
    void test() {
        int a = 1 + 2;
        int b = a * 3 - 4 / 2;
        int c = a % b;
        boolean d = a > b;
        boolean e = a == b;
        boolean f = a != b && d || e;
        int g = d ? 1 : 0;
        a += 5;
        a -= 1;
        a *= 2;
        a /= 3;
        a++;
        a--;
        ++a;
        --a;
        int h = ~a;
        boolean i = !d;
        int j = a << 2;
        int k = a >> 1;
        int l = a >>> 1;
        int m = a & b;
        int n = a | b;
        int o = a ^ b;
    }
}

