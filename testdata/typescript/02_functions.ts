function add(a: number, b: number): number {
  return a + b;
}

function greet(name: string, greeting?: string): void {
  console.log((greeting || "Hello") + ", " + name);
}

function sum(...nums: number[]): number {
  return nums.reduce((a, b) => a + b, 0);
}

const double = (x: number): number => x * 2;

function identity<T>(x: T): T {
  return x;
}

function map<T, U>(arr: T[], fn: (item: T) => U): U[] {
  return arr.map(fn);
}

async function fetchData(url: string): Promise<string> {
  const response = await fetch(url);
  return response.text();
}
