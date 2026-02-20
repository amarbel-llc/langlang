let value: unknown = "hello";
let x: any = 42;

let s = value as string;
let n = <number>x;

function isString(val: unknown): val is string {
  return typeof val === "string";
}

function assertDefined<T>(val: T | undefined): asserts val is T {
  if (val === undefined) throw new Error("undefined");
}

type EventMap = {
  click: MouseEvent;
  keydown: KeyboardEvent;
};

type EventType = keyof EventMap;

function handleEvent<K extends keyof EventMap>(type: K, handler: (ev: EventMap[K]) => void): void {
  console.log(type);
}

let tuple: [string, number] = ["hello", 42];
let named: [name: string, age: number] = ["Alice", 30];

type IsString<T> = T extends string ? "yes" : "no";

declare function create<T>(o: T): Container<T>;
declare const sym: unique symbol;

namespace Validation {
  export interface Validator {
    validate(s: string): boolean;
  }
}
