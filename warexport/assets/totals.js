TABS.totals = (content, BODY) => {
  content.append(make("p", "note", "What each side lost: its running total on the left, each day's losses on the right. A system counts as lost the first time killmails show the side losing a citadel or POCO there; POS are left out, as invaders often anchor them."));
  if (BODY.days.length === 0) {
    content.append(make("p", "", "No losses yet."));
    return;
  }

  // every date with something on it, oldest first
  const dates = [...new Set(BODY.days.map((day) => day.date).concat(BODY.lost.map((lost) => lost.date)))].sort();
  const byDate = new Map(BODY.days.map((day) => [day.date, day]));
  const lostOn = (date, side) => BODY.lost.filter((lost) => lost.date === date && lost.side === side).length;
  const MEASURES = [
    { title: "ISK lost (billions)", read: (day, side) => day[side].isk / 1e9, format: ".1f", text: (value) => `${value.toFixed(1)}B` },
    { title: "Ships lost", read: (day, side) => day[side].ships, format: ",d", text: count },
    { title: "Structures lost", read: (day, side) => day[side].structures, format: ",d", text: count },
    { title: "Systems lost", read: null, format: ",d", text: count },
  ];
  const daily = (measure, side) => dates.map((date) => {
    if (!measure.read) return lostOn(date, side);
    const day = byDate.get(date);
    return day ? measure.read(day, side) : 0;
  });
  const running = (values) => {
    let sum = 0;
    return values.map((value) => (sum += value));
  };

  const axisFor = (measure, traces) => (measure.format === ".1f" ? { rangemode: "tozero" } : countAxis(traces.flatMap((trace) => trace.y)));
  for (const measure of MEASURES) {
    content.append(make("h2", "", measure.title));
    const row = make("div", "charts");
    content.append(row);
    const lines = ["a", "b"].map((side) => ({
      x: dates, y: running(daily(measure, side)), name: sideName(side), type: "scatter", mode: "lines",
      line: { color: COLORS[side], width: 2 }, hovertemplate: `%{y:${measure.format}}<extra>%{fullData.name}</extra>`,
    }));
    const ends = lines.map((line) => ({
      x: line.x[line.x.length - 1], y: line.y[line.y.length - 1], text: measure.text(line.y[line.y.length - 1]),
      xanchor: "left", xshift: 4, showarrow: false, font: { color: COLORS.text },
    }));
    chart(row, lines, { title: { text: "Running total", font: { size: 13 }, x: 0.01, xanchor: "left", y: 0.97 }, hovermode: "x unified", height: 280, margin: { l: 56, r: 56, t: 56, b: 44 }, xaxis: { type: "date" }, yaxis: axisFor(measure, lines), annotations: ends });
    const bars = ["a", "b"].map((side) => ({
      x: dates, y: daily(measure, side), name: sideName(side), type: "bar", marker: { color: COLORS[side] },
      hovertemplate: `%{y:${measure.format}}<extra>%{fullData.name}</extra>`,
    }));
    chart(row, bars, { title: { text: "Each day", font: { size: 13 }, x: 0.01, xanchor: "left", y: 0.97 }, hovermode: "x unified", barmode: "group", bargap: 0.25, bargroupgap: 0.1, height: 280, xaxis: { type: "date" }, yaxis: axisFor(measure, bars) });
  }

  const rows = dates.map((date) => {
    const day = byDate.get(date) || { a: { isk: 0, ships: 0, structures: 0 }, b: { isk: 0, ships: 0, structures: 0 } };
    return [writtenDate(date), isk(day.a.isk), isk(day.b.isk), count(day.a.ships), count(day.b.ships), count(day.a.structures), count(day.b.structures), count(lostOn(date, "a")), count(lostOn(date, "b"))];
  });
  const a = sideName("a"), b = sideName("b");
  content.append(tableView("The figures as a table", table(["Day", `#ISK, ${a}`, `#ISK, ${b}`, `#Ships, ${a}`, `#Ships, ${b}`, `#Structures, ${a}`, `#Structures, ${b}`, `#Systems, ${a}`, `#Systems, ${b}`], rows)));
};
