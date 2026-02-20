interface Greeter {
    default void greet() {
        System.out.println("Hello");
    }
    void sayGoodbye();
}

