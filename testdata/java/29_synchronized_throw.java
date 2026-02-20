class SyncThrow {
    Object lock;

    synchronized void syncMethod() {}

    void test() {
        synchronized (lock) {
            int x = 1;
        }

        throw new RuntimeException("error");
    }

    void checked() throws Exception {
        throw new Exception("checked");
    }
}

