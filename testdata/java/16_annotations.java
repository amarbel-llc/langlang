@interface MyAnnotation {
    String value();
    int count() default 0;
}

@MyAnnotation(value = "test", count = 5)
class Annotated {
    @Override
    public String toString() { return ""; }

    @SuppressWarnings("unchecked")
    void test() {}

    @Deprecated
    void old() {}
}

