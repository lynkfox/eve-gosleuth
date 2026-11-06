TABS.summary = (content, BODY) => {
  const counts = BODY.counts;

  const countRows = make("div", "panel");
  const classes = BODY.classes.map((c) => `${c.count} ${classLabel(c.class)}`).join(", ");
  countRows.append(table(["#Battles", "#Skirmishes", "#Systems", "#Killmails"].concat(counts.ignored ? ["#Ignored"] : []),
    [[count(counts.battles), count(counts.skirmishes), count(counts.systems), count(counts.killmails)].concat(counts.ignored ? [count(counts.ignored)] : [])]));
  if (classes) countRows.append(make("p", "note", `${counts.systems} systems: ${classes}`));
  content.append(countRows);

  const columns = [["a", BODY.a], ["b", BODY.b], ["others", BODY.others]];
  const measures = make("table");
  const head = make("tr");
  head.append(make("th"));
  for (const [key] of columns) head.append(make("th", `num head-${key}`, sideName(key)));
  const thead = make("thead");
  thead.append(head);
  const tbody = make("tbody");
  measures.append(thead, tbody);
  const row = (label, read, options = {}) => {
    const line = make("tr", options.strong ? "strong" : "");
    const name = make("td", options.indent === 2 ? "indent2" : options.indent ? "indent" : "");
    name.append(label);
    line.append(name);
    for (const [key, values] of columns) line.append(make("td", "num", read(values, key)));
    tbody.append(line);
  };
  const any = (read) => columns.some(([, values]) => read(values) > 0);

  row("ISK lost", (m) => isk(m.iskLost), { strong: true });
  row("In fights", (m) => isk(m.iskFights), { indent: 1 });
  if (any((m) => m.iskTrash)) row("Station trash and asset denial", (m) => isk(m.iskTrash), { indent: 1 });
  row("Structures", (m) => isk(m.iskStructures), { indent: 1 });
  row("Ships lost", (m) => count(m.shipsLost), { strong: true });
  row("In fights", (m) => count(m.shipsFights), { indent: 1 });
  if (any((m) => m.shipsTrash)) row("Station trash and asset denial", (m) => count(m.shipsTrash), { indent: 1 });
  row("All citadels lost", (m) => count(m.citadelCount), { strong: true });
  const kinds = [...new Set(columns.flatMap(([, m]) => m.citadels.map((c) => c.name)))].sort();
  const kindIcon = (kind) => columns.flatMap(([, m]) => m.citadels).find((c) => c.name === kind).icon;
  for (const kind of kinds) {
    row(named(kindIcon(kind), kind), (m) => {
      const found = m.citadels.find((c) => c.name === kind);
      return count(found ? found.count : 0);
    }, { indent: 1 });
  }
  row("All POS and POCOs lost", (m) => count(m.pos), { strong: true });
  if (any((m) => m.stationRefs)) row("Citadel Ref'd", (m) => count(m.stationRefs), { indent: 1 });
  if (any((m) => m.refAttempts)) row("Attacked", (m) => count(m.refAttempts), { indent: 1 });
  if (any((m) => m.refAttemptsUnsure)) row("Attacked?", (m) => count(m.refAttemptsUnsure), { indent: 1 });
  row("Efficiency", (m) => (m.hasEfficiency ? `${m.efficiency.toFixed(1)}%` : "–"), { strong: true });

  const panel = make("div", "panel scroll");
  panel.append(measures);
  if (any((m) => m.stationRefs + m.refAttempts + m.refAttemptsUnsure)) panel.append(tableView("What the station marks mean", make("p", "note", MARK_KEY)));
  panel.append(make("p", "note", "Efficiency is the other side's ISK lost as a share of both sides'. No battle is called won."));
  content.append(panel);

  if (BODY.reach.length > 0) {
    const reach = make("div", "panel");
    reach.append(make("h2", "", "Lists that stop short"),
      make("p", "note", "zKillboard's lists stop before the start of the war for these groups, so older killmails may be missing."),
      table(["Group", "Reaches back to"], BODY.reach.map((line) => [line.group, writtenDate(line.oldest)])));
    content.append(reach);
  }

  const sides = make("div", "sides");
  for (const [key, groups] of [["a", BODY.groupsA], ["b", BODY.groupsB]]) {
    const side = make("div", "panel");
    side.append(make("h2", "", `${sideName(key)}: who fought`), table(["Alliance or corp", "#Pilots"], groups.map((group) => [named(group.icon, group.name), count(group.pilots)])));
    sides.append(side);
  }
  content.append(sides);
};
