interface Printable {
    String display();
}

record Color(int r, int g, int b) implements Printable {
    public String display() {
        return "rgb(" + r + "," + g + "," + b + ")";
    }
}

