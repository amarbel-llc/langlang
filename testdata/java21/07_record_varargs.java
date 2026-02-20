class RecordVarargs {
    record TaggedValue(String tags, Double... values) {
        String getTags() { return tags; }
    }

    record DataPoint(long orgId, String metricName, long timestampSecs,
                     double value, String... tags) {}
}
