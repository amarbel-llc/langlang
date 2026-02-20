class StaticInit {
    static int x;
    static {
        x = 42;
    }

    int y;
    {
        y = 10;
    }
}

