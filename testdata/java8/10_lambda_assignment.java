import java.util.function.Function;
import java.util.function.Predicate;

class A {
    Function<String, Integer> f = s -> s.length();
    Predicate<String> p = s -> s.isEmpty();

    void m() {
        Function<Integer, Integer> square = x -> x * x;
    }
}

