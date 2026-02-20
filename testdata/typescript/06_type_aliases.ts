type ID = string | number;
type Point = { x: number; y: number };
type Nullable<T> = T | null;
type Callback = (data: string) => void;
type Pair<A, B> = [A, B];
type StringOrArray = string | string[];

type Tree<T> = {
  value: T;
  left?: Tree<T>;
  right?: Tree<T>;
};

type ReadOnly<T> = {
  readonly [K in keyof T]: T[K];
};

type Partial<T> = {
  [K in keyof T]?: T[K];
};

type ReturnType<T> = T extends (...args: any[]) => infer R ? R : never;

type Exclude<T, U> = T extends U ? never : T;

let id: ID = "abc";
let point: Point = { x: 1, y: 2 };
let maybe: Nullable<string> = null;
