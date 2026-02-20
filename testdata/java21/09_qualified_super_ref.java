class QualifiedSuperRef {
    interface Base {
        default String name() { return "base"; }
    }

    static class Child implements Base {
        void run() {
            Runnable r = Base.super::name;
        }

        String delegated() {
            return Base.super.name();
        }
    }
}
