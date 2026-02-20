enum Direction {
  Up,
  Down,
  Left,
  Right,
}

enum Color {
  Red = "RED",
  Green = "GREEN",
  Blue = "BLUE",
}

const enum StatusCode {
  OK = 200,
  NotFound = 404,
  InternalError = 500,
}

enum FileAccess {
  None,
  Read = 1 << 1,
  Write = 1 << 2,
  ReadWrite = Read | Write,
}

let dir: Direction = Direction.Up;
let color: Color = Color.Red;
