class AnonymousClass {
    void test() {
        Object o = new Object() {
            public String toString() { return "anonymous"; }
        };
    }
}

