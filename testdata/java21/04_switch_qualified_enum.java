class SwitchQualifiedEnum {
    enum Color { RED, GREEN, BLUE }

    String name(Color c) {
        return switch (c) {
            case Color.RED -> "red";
            case Color.GREEN -> "green";
            case Color.BLUE -> "blue";
        };
    }

    // Mixed qualified and unqualified
    int value(Color c) {
        return switch (c) {
            case Color.RED, GREEN -> 1;
            case Color.BLUE -> 2;
        };
    }
}
