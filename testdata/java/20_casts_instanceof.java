class CastsInstanceof {
    void test(Object o) {
        if (o instanceof String) {
            String s = (String) o;
        }
        int x = (int) 3.14;
        double d = (double) 42;
    }
}

