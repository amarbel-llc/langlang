for (var item of items) {
  process(item);
}

for (let [key, value] of map) {
  store(key, value);
}

for (const { name, age } of people) {
  greet(name, age);
}
