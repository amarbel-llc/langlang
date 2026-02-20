interface Greetable {
    String greet();
    void wave(int times);
}

interface Closeable extends Greetable {
    void close();
}

class Greeter implements Greetable {
    public String greet() { return "hi"; }
    public void wave(int times) {}
}

