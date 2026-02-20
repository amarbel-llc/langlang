class A {
    void m(Object o) {
        if (o instanceof String s) {
            System.out.println(s.length());
        }
        if (o instanceof Integer i) {
            System.out.println(i + 1);
        }
    }
}

