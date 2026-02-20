import java.util.List;
import java.util.Optional;

class DeconstructionPatterns {
    record Pair(String a, String b) {}
    record Outer(String label, Pair inner) {}
    sealed interface Node permits Leaf, Branch {}
    record Leaf(String value) implements Node {}
    record Branch(Node left, Node right) implements Node {}

    void matchPair(Object obj) {
        if (obj instanceof Pair(String a, String b)) {
            System.out.println(a + b);
        }
    }

    void nestedPattern(Object obj) {
        if (obj instanceof Outer(String label, Pair(String a, String b))) {
            System.out.println(label + a + b);
        }
    }

    int depth(Node node) {
        return switch (node) {
            case Leaf(String v) -> 0;
            case Branch(Node left, Node right) -> 1 + Math.max(depth(left), depth(right));
        };
    }

    void switchNested(Object o) {
        switch (o) {
            case Outer(String path, Pair(String a, String b)) ->
                System.out.println(path + a + b);
            default -> {}
        }
    }
}
