TABS.ships = (content, BODY) => {
  content.append(make("p", "note", "Each ship either side flew, A to Z, the two sides beside each other. A bar's length is the total flown, a pilot counting once per battle per ship, with the part lost in diagonal lines."));
  const byName = (hulls) => new Map(hulls.map((hull) => [hull.name, hull]));
  const a = byName(BODY.a), b = byName(BODY.b);
  const names = [...new Set([...a.keys(), ...b.keys()])].sort((x, y) => x.toLowerCase().localeCompare(y.toLowerCase()));
  if (names.length === 0) {
    content.append(make("p", "", "No ships seen."));
    return;
  }
  const read = (hulls, field) => names.map((name) => (hulls.has(name) ? hulls.get(name)[field] : 0));

  // each side's own lane in a ship's row, as long as its total: those lost in a pattern, then the rest of the total in full color. The rest's hover gives the total.
  // A row can have more lost than flown, as a pilot counts once per battle, so it runs to the number lost.
  const traces = [["a", a], ["b", b]].flatMap(([key, hulls]) => {
    const flown = read(hulls, "seen"), lost = read(hulls, "lost");
    return [
      { y: names, x: lost, name: `${sideName(key)}, Lost`, type: "bar", orientation: "h", offsetgroup: key,
        marker: { color: COLORS[key], line: { color: COLORS.panel, width: 1 }, pattern: { shape: "/", fgcolor: COLORS[key], bgcolor: `${COLORS[key]}40`, size: 6, solidity: 0.35 } },
        hovertemplate: "%{x:,d}<extra>%{fullData.name}</extra>" },
      { y: names, x: flown.map((total, index) => Math.max(0, total - lost[index])), base: lost, customdata: flown, name: `${sideName(key)}, Total flown`, type: "bar", orientation: "h", offsetgroup: key,
        marker: { color: COLORS[key], line: { color: COLORS.panel, width: 1 } },
        hovertemplate: "%{customdata:,d}<extra>%{fullData.name}</extra>" },
    ];
  });
  const totals = [a, b].flatMap((hulls) => read(hulls, "seen").concat(read(hulls, "lost")));
  // the chart is tall, so its numbers are along the top, and the legend is placed by pixels as a share of this height would leave a gap
  const height = names.length * 30 + 110;
  const margin = { l: 150, r: 24, t: 76, b: 16 };
  chart(content, traces, {
    barmode: "group", bargap: 0.2, bargroupgap: 0.05, hovermode: "y unified", height, margin,
    legend: { orientation: "h", x: 0, y: 1 + 28 / (height - margin.t - margin.b), yanchor: "bottom" },
    yaxis: { type: "category", categoryorder: "array", categoryarray: names, autorange: "reversed", dtick: 1 }, xaxis: { ...countAxis(totals), side: "top" },
  });

  const icons = new Map([...BODY.a, ...BODY.b].map((hull) => [hull.name, hull.icon]));
  const flownA = read(a, "seen"), lostA = read(a, "lost"), flownB = read(b, "seen"), lostB = read(b, "lost");
  content.append(tableView("The figures as a table", table(["Ship", `#${sideName("a")}, Total flown`, `#${sideName("a")}, Lost`, `#${sideName("b")}, Total flown`, `#${sideName("b")}, Lost`],
    names.map((name, index) => [named(icons.get(name), name), count(flownA[index]), count(lostA[index]), count(flownB[index]), count(lostB[index])]))));
};
