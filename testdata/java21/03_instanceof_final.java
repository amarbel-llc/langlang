class InstanceofFinal {
    boolean test(Object obj) {
        if (obj instanceof final String s) {
            return s.isEmpty();
        }
        if (obj instanceof final Integer i) {
            return i > 0;
        }
        return false;
    }
}
