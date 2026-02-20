package com.example;

import java.util.List;
import java.util.ArrayList;

/**
 * A realistic Java class combining many features.
 */
public class Calculator {
    private int result;
    private final String name;
    private static int instanceCount;

    static {
        instanceCount = 0;
    }

    public Calculator(String name) {
        this.name = name;
        this.result = 0;
        instanceCount++;
    }

    public Calculator add(int value) {
        result += value;
        return this;
    }

    public Calculator subtract(int value) {
        result -= value;
        return this;
    }

    public Calculator multiply(int factor) {
        if (factor == 0) {
            result = 0;
        } else {
            result *= factor;
        }
        return this;
    }

    public int getResult() {
        return result;
    }

    public String getName() {
        return name;
    }

    public static int getInstanceCount() {
        return instanceCount;
    }

    public boolean isPositive() {
        return result > 0;
    }

    public String describe() {
        return name + ": " + (isPositive() ? "positive" : "non-positive")
            + " (" + result + ")";
    }

    public int[] toArray(int size) {
        int[] arr = new int[size];
        for (int i = 0; i < size; i++) {
            arr[i] = result + i;
        }
        return arr;
    }

    public void process(int[] values) {
        for (int v : values) {
            if (v > 0) {
                add(v);
            } else if (v < 0) {
                subtract(-v);
            }
        }
    }

    public void safeCompute() {
        try {
            int x = 100 / result;
            result = x;
        } catch (ArithmeticException e) {
            result = 0;
        } finally {
            // Always log
        }
    }

    @Override
    public String toString() {
        return describe();
    }

    @Override
    public boolean equals(Object obj) {
        if (this == obj) return true;
        if (!(obj instanceof Calculator)) return false;
        Calculator other = (Calculator) obj;
        return this.result == other.result;
    }

    @Override
    public int hashCode() {
        return result * 31 + name.hashCode();
    }

    public static void main(String[] args) {
        Calculator calc = new Calculator("main");
        calc.add(10).subtract(3).multiply(2);

        int[] data = new int[]{1, -2, 3, -4, 5};
        calc.process(data);

        assert calc.getResult() != 0 : "Result should not be zero";
    }
}

