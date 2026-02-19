function makeCounter(start) {
  var n = start || 0;
  return {
    inc: function () {
      n += 1;
      return n;
    },
    dec: function () {
      n -= 1;
      return n;
    }
  };
}

var counter = makeCounter(10);
for (var i = 0; i < 3; i++) {
  counter.inc();
}

var result = counter.dec();
if (result > 10) {
  result = result - 10;
}
