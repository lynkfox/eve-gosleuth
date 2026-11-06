TABS.battles = (content, BODY) => {
  const rows = BODY.battles;
  const ORDER = ["high", "low", "null", "C1", "C2", "C3", "C4", "C5", "C6"];
  const classRank = (cls) => {
    const index = ORDER.indexOf(cls);
    return index < 0 ? ORDER.length : index;
  };
  const SORTS = {
    When: (x, y) => new Date(y.start) - new Date(x.start),
    Class: (x, y) => classRank(x.class) - classRank(y.class),
    System: (x, y) => x.system.localeCompare(y.system),
    A: (x, y) => y.a.iskLost - x.a.iskLost,
    B: (x, y) => y.b.iskLost - x.b.iskLost,
    Pilots: (x, y) => y.pilots - x.pilots,
  };
  let sort = "When";
  const filters = new Set();

  content.append(make("p", "note", "Every fight of the war, skirmishes included and battles set aside by hand not. ISK lost leaves out station trash and asset denial."));
  const chips = make("div", "chips");
  const classes = [...new Set(rows.map((row) => row.class))].sort((x, y) => classRank(x) - classRank(y) || x.localeCompare(y));
  for (const cls of classes) {
    const button = make("button", "", `${classLabel(cls)} (${rows.filter((row) => row.class === cls).length})`);
    button.type = "button";
    button.setAttribute("aria-pressed", "false");
    button.addEventListener("click", () => {
      if (filters.has(cls)) filters.delete(cls);
      else filters.add(cls);
      button.setAttribute("aria-pressed", String(filters.has(cls)));
      draw();
    });
    chips.append(button);
  }
  content.append(chips);
  const holder = make("div", "panel scroll battles-table");
  content.append(holder);

  function sortHead(label, key, numeric, sideClass) {
    const cell = make("th", `${numeric ? "num " : ""}${sideClass || ""}`);
    const button = make("button", "", key === sort ? `${label} ▾` : label);
    button.type = "button";
    button.addEventListener("click", () => { sort = key; draw(); });
    cell.append(button);
    return cell;
  }

  function sideCell(side) {
    const cell = make("td");
    cell.append(side.group ? named(side.groupIcon, side.group) : "–");
    return cell;
  }

  function draw() {
    const shown = rows.filter((row) => filters.size === 0 || filters.has(row.class)).slice().sort((x, y) => SORTS[sort](x, y) || SORTS.When(x, y));
    const node = make("table");
    const head = make("tr");
    head.append(sortHead("When", "When"), sortHead("Class", "Class"), sortHead("System", "System"), sortHead("Pilots", "Pilots", true),
      make("th", "head-a wrap", `${sideName("a")}, largest`), sortHead("ISK lost", "A", true, "head-a"), make("th", "num head-a", "Ships"),
      make("th", "head-b wrap", `${sideName("b")}, largest`), sortHead("ISK lost", "B", true, "head-b"), make("th", "num head-b", "Ships"),
      make("th", "num head-others wrap", "Others' ISK"), make("th", "num", "Structs"), make("th", "", "Battle reports"));
    const thead = make("thead");
    thead.append(head);
    const tbody = make("tbody");
    for (const row of shown) {
      const line = make("tr");
      const when = make("td", "when-cell");
      const moment = make("div", "when-line", `${writtenDate(row.start)} ${writtenTime(row.start)}`);
      if (row.skirmish) moment.append(make("span", "chit", "Skirmish"));
      when.append(moment);
      if (row.marks.length > 0) {
        const marks = make("div", "marks");
        marks.append(...row.marks.map(markChip));
        when.append(marks);
      }
      row.notes.forEach((note) => when.append(make("div", "battle-note", note)));
      const cls = make("td");
      cls.append(classChip(row.class));
      const system = make("td", "system-cell");
      system.append(row.system);
      if (row.nickname) system.append(" - ", make("span", "nick", row.nickname));
      if (row.otherSystems.length) {
        const also = make("span", "chit", `+${row.otherSystems.length}`);
        also.title = row.otherSystems.join(", ");
        system.append(also);
      }
      const links = make("td", "links");
      for (const link of row.links) {
        const anchor = make("a", "", link.label);
        anchor.href = link.url;
        anchor.target = "_blank";
        anchor.rel = "noopener noreferrer";
        links.append(anchor);
      }
      line.append(when, cls, system, make("td", "num", count(row.pilots)),
        sideCell(row.a), make("td", "num", isk(row.a.iskLost)), make("td", "num", count(row.a.shipsLost)),
        sideCell(row.b), make("td", "num", isk(row.b.iskLost)), make("td", "num", count(row.b.shipsLost)),
        make("td", "num", isk(row.others.iskLost)), make("td", "num", count(row.a.structures + row.b.structures + row.others.structures)), links);
      tbody.append(line);
    }
    node.append(thead, tbody);
    holder.replaceChildren(node);
  }
  draw();
};
