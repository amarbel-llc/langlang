class SwitchStringConcat {
    static final String FOO = "foo";
    static final String BAR = "bar";

    void process(String key) {
        switch (key) {
            case FOO, "-" + FOO -> System.out.println("foo");
            case BAR, "-" + BAR -> System.out.println("bar");
            default -> {}
        }
    }
}
