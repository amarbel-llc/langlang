class Builder {
    Builder setName(String name) { return this; }
    Builder setValue(int value) { return this; }
    Object build() { return null; }

    void test() {
        Object o = new Builder().setName("foo").setValue(42).build();
    }
}

