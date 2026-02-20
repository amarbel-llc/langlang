import java.util.function.Predicate;
import java.util.function.BiPredicate;

class LambdaTernary {
    Predicate<Object> p = true ? x -> true : x -> false;

    BiPredicate<String, Integer> bp = true
        ? (a, b) -> a.length() > b
        : (a, b) -> a.isEmpty();

    void m(boolean flag) {
        var fn = flag
            ? (Predicate<Object>) x -> x != null
            : (Predicate<Object>) x -> x == null;
    }
}
