abstract class Shape {
    abstract double area();
    abstract double perimeter();

    String describe() {
        return "shape";
    }
}

class Circle extends Shape {
    double radius;
    Circle(double r) { this.radius = r; }
    double area() { return 3.14 * radius * radius; }
    double perimeter() { return 2 * 3.14 * radius; }
}

