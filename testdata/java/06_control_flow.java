class ControlFlow {
    void test() {
        if (true) {}
        if (true) {} else {}
        if (true) {} else if (false) {} else {}

        for (int i = 0; i < 10; i++) {}

        while (true) { break; }

        do { continue; } while (false);

        switch (1) {
            case 1: break;
            case 2: break;
            default: break;
        }
    }
}

