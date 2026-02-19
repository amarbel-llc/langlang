function* range(start, end) {
  for (var i = start; i < end; i++) {
    yield i;
  }
}

function* delegating() {
  yield* range(0, 5);
}

var gen = function*() {
  var val = yield 1;
  yield val + 2;
};
