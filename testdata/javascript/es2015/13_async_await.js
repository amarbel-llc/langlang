async function fetchData(url) {
  var response = await fetch(url);
  var data = await response.json();
  return data;
}

var load = async () => {
  var result = await fetchData("/api");
  return result;
};

async function processAll(items) {
  for (var item of items) {
    await process(item);
  }
}
