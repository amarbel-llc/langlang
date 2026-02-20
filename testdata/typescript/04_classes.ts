class Point {
  x: number;
  y: number;

  constructor(x: number, y: number) {
    this.x = x;
    this.y = y;
  }

  distance(other: Point): number {
    return Math.sqrt((this.x - other.x) ** 2 + (this.y - other.y) ** 2);
  }
}

class Animal {
  constructor(public name: string) {}

  speak(): void {
    console.log(this.name + " makes a noise.");
  }
}

class Dog extends Animal {
  constructor(name: string, private breed: string) {
    super(name);
  }

  speak(): void {
    console.log(this.name + " barks.");
  }
}

abstract class Shape {
  abstract area(): number;
  abstract perimeter(): number;

  describe(): string {
    return "Shape with area " + this.area();
  }
}

class Circle extends Shape {
  constructor(private radius: number) {
    super();
  }

  area(): number {
    return Math.PI * this.radius ** 2;
  }

  perimeter(): number {
    return 2 * Math.PI * this.radius;
  }
}

class Container<T> {
  private items: T[] = [];

  add(item: T): void {
    this.items.push(item);
  }

  get(index: number): T {
    return this.items[index];
  }
}
