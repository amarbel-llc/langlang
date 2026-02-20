class TryCatch {
    void test() {
        try {
            int x = 1;
        } catch (Exception e) {
            int y = 2;
        } finally {
            int z = 3;
        }

        try {
            int a = 1;
        } catch (RuntimeException | IllegalArgumentException e) {
            int b = 2;
        }
    }
}

