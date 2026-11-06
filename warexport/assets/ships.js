TABS.ships = (content, BODY) => {
  content.append(make("p", "note", "Each ship either side flew, A to Z, the two sides beside each other: how many were seen, a pilot counting once per battle per ship, with how many were destroyed stacked on it."));
  const byName = (hulls) => new Map(hulls.map((hull) => [hull.name, hull]));
  const a = byName(BODY.a), b = byName(BODY.b);
  const names = [...new Set([...a.keys(), ...b.keys()])].sort((x, y) => x.toLowerCase().localeCompare(y.toLowerCase()));
  if (names.length === 0) {
    content.append(make("p", "", "No ships seen."));
    return;
  }
  const read = (hulls, field) => names.map((name) => (hulls.has(name) ? hulls.get(name)[field] : 0));
  const hover = "%{x:,d}<extra>%{fullData.name}</extra>";

  // each side's own lane in a ship's row: flown, then destroyed starting where flown ends
  const traces = [["a", a], ["b", b]].flatMap(([key, hulls]) => {
    const flown = read(hulls, "seen");
    return [
      { y: names, x: flown, name: `${sideName(key)}, flown`, type: "bar", orientation: "h", offsetgroup: key, marker: { color: COLORS[key], line: { color: COLORS.panel, width: 1 } }, hovertemplate: hover },
      { y: names, x: read(hulls, "lost"), base: flown, name: `${sideName(key)}, destroyed`, type: "bar", orientation: "h", offsetgroup: key, marker: { color: COLORS.neutral, line: { color: COLORS.panel, width: 1 } }, hovertemplate: hover },
    ];
  });
  const totals = traces.flatMap((trace, index) => trace.x.map((value, at) => value + (index % 2 === 1 ? trace.base[at] : 0)));
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
  content.append(tableView("The figures as a table", table(["Ship", `#${sideName("a")}, flown`, `#${sideName("a")}, destroyed`, `#${sideName("b")}, flown`, `#${sideName("b")}, destroyed`],
    names.map((name, index) => [named(icons.get(name), name), count(flownA[index]), count(lostA[index]), count(flownB[index]), count(lostB[index])]))));
};
