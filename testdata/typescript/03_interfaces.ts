interface Point {
  x: number;
  y: number;
}

interface Named {
  name: string;
  displayName?: string;
}

interface Shape {
  area(): number;
  perimeter(): number;
}

interface Container<T> {
  value: T;
  getValue(): T;
  setValue(v: T): void;
}

interface ReadonlyPoint {
  readonly x: number;
  readonly y: number;
}

interface StringMap {
  [key: string]: string;
}

interface EventHandler {
  (event: string, data: unknown): void;
}

interface Animal {
  name: string;
  speak(): void;
}

interface Dog extends Animal {
  breed: string;
}
