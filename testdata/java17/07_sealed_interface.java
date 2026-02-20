sealed interface Expr permits Num, Add {}

record Num(int value) implements Expr {}
record Add(Expr left, Expr right) implements Expr {}

