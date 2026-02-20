import java.lang.annotation.*;

class TypeAnnotationsQualified {
    @Target(ElementType.TYPE_USE)
    @interface NotNull {}

    @Target(ElementType.TYPE_USE)
    @interface Nullable {}

    static class Outer {
        static class Inner {}
    }

    Outer.@NotNull Inner field1;
    Outer.@Nullable Inner field2;

    static Outer.@NotNull Inner method() { return null; }
}
