TABS.timeline = (content, BODY) => {
  content.append(make("p", "note", "A bubble per fight, skirmishes included, on the main system of a fight that moved, sized by the ISK all sides lost, leaving out station trash. A ring marks a structure lost or under attack in it. Color goes by the side whose citadels or POCOs killmails show in that system, lost or firing; POS don't count, as invaders often anchor them. It says what was seen, not who holds the system. Click a bubble for its details and battle reports."));
  const BATTLES = BODY.order.map((index) => DATA.body.battles.battles[index]);
  if (BATTLES.length === 0) {
    content.append(make("p", "", "No fights yet."));
    return;
  }

  const totalISK = (battle) => battle.a.iskLost + battle.b.iskLost + battle.others.iskLost;
  const structures = (battle) => battle.a.structures + battle.b.structures + battle.others.structures;
  const ringed = (battle) => structures(battle) > 0 || battle.attacked;
  const largest = Math.max(1, ...BATTLES.map(totalISK));
  const MAX_BUBBLE = 46;
  const sideLine = (key, battle) => `${sideName(key)}: ${isk(battle[key].iskLost)} lost, ${counted(battle[key].pilots, "pilot")}${battle[key].group ? `, mostly ${battle[key].group}` : ""}`;
  const CUT = 40;
  // by whole characters, so an emoji at the cut stays whole
  const cut = (text) => {
    const characters = [...text];
    return characters.length > CUT ? `${characters.slice(0, CUT - 1).join("")}…` : text;
  };

  const hover = (battle) => [
    `${writtenDate(battle.start)} ${writtenTime(battle.start)}${battle.skirmish ? " · Skirmish" : ""}`,
    `${battle.system}${battle.nickname ? ` (${battle.nickname})` : ""} · ${classLabel(battle.class)}`,
    `${counted(battle.pilots, "pilot")}, ${counted(battle.killmails, "killmail")}, ${isk(totalISK(battle))} lost`,
    sideLine("a", battle), sideLine("b", battle),
    ...(battle.others.iskLost > 0 ? [sideLine("others", battle)] : []),
    ...(structures(battle) > 0 ? [`${counted(structures(battle), "structure")} lost`] : []),
    ...battle.marks.map((mark) => `${markLabel(mark)}, ${sideName(mark.side)}`),
    ...(battle.otherSystems.length ? [`Also in ${battle.otherSystems.join(", ")}`] : []),
    ...battle.notes.map((note) => `<i>${note}</i>`),
  ].join("<br>");

  const GROUPS = [
    { holder: "a", name: `${sideName("a")}'s structures seen`, color: COLORS.a },
    { holder: "b", name: `${sideName("b")}'s structures seen`, color: COLORS.b },
    { holder: "", name: "Neither, or both", color: COLORS.neutral },
  ];
  const traces = GROUPS.map((group) => {
    // the largest first, so a small one beside it stays on top and can be clicked
    const battles = BATTLES.filter((battle) => battle.holder === group.holder).sort((x, y) => totalISK(y) - totalISK(x));
    return {
      x: battles.map((battle) => battle.start), y: battles.map((battle) => battle.system),
      name: group.name, type: "scatter", mode: "markers", customdata: battles.map((battle) => BATTLES.indexOf(battle)),
      text: battles.map(hover), hoverinfo: "text",
      marker: {
        color: group.color, opacity: 0.85, sizemode: "area", sizeref: (2 * largest) / (MAX_BUBBLE * MAX_BUBBLE), sizemin: 7,
        size: battles.map(totalISK),
        line: { color: battles.map((battle) => (ringed(battle) ? "#ffffff" : COLORS.panel)), width: battles.map((battle) => (ringed(battle) ? 2 : 1)) },
      },
    };
  }).filter((trace) => trace.x.length > 0);

  // a nicknamed system's dashed line with its name, drawn behind the bubbles
  const first = BATTLES[0].start;
  const last = BATTLES[BATTLES.length - 1].start;
  const nicknames = new Map();
  for (const battle of BATTLES) if (battle.nickname) nicknames.set(battle.system, battle.nickname);
  const shapes = [...nicknames.keys()].map((system) => ({ type: "line", layer: "below", xref: "paper", x0: 0, x1: 1, yref: "y", y0: system, y1: system, line: { color: "#e0a526", width: 1, dash: "dash" } }));
  const annotations = [...nicknames].map(([system, name]) => ({ xref: "paper", x: 0, xanchor: "left", y: system, yanchor: "bottom", text: cut(name), hovertext: name, showarrow: false, font: { color: "#e0a526", size: 11 } }));

  // notes as callouts, cut short, in full on hover
  // at three heights in turn, so those of battles close together stay readable
  let callout = 0;
  for (const battle of BATTLES) {
    for (const note of battle.notes) {
      const height = -18 - 18 * (callout++ % 3);
      annotations.push({ x: battle.start, y: battle.system, text: `<i>${cut(note)}</i>`, hovertext: note, showarrow: true, arrowcolor: COLORS.muted, arrowwidth: 1, ax: 16, ay: height, font: { color: COLORS.text, size: 11 }, bgcolor: "#1a212c", bordercolor: COLORS.border, borderpad: 2 });
    }
  }

  const holder = make("div");
  content.append(holder);
  const pad = Math.max(3600e3 * 6, (new Date(last) - new Date(first)) * 0.03);
  const node = chart(holder, traces, {
    height: Math.max(420, BODY.rows.length * 30 + 130), hovermode: "closest", margin: { l: 90, r: 24, t: 44, b: 44 }, shapes, annotations,
    xaxis: { type: "date", range: [new Date(new Date(first) - pad).toISOString(), new Date(new Date(last).getTime() + pad).toISOString()] },
    yaxis: { type: "category", categoryorder: "array", categoryarray: BODY.rows, autorange: "reversed", dtick: 1 },
  });

  // the details of a clicked bubble, with its battle reports
  let open = null;
  const showBattle = (battle) => {
    if (open) open.remove();
    const panel = make("aside", "battle-panel");
    panel.setAttribute("aria-label", "Battle details");
    const close = make("button", "close", "×");
    close.type = "button";
    close.setAttribute("aria-label", "Close");
    close.addEventListener("click", () => { panel.remove(); open = null; });
    const title = make("h3", "", `${battle.system}${battle.nickname ? ` (${battle.nickname})` : ""}`);
    title.append(" ", classChip(battle.class));
    panel.append(close, title,
      make("p", "note", `${writtenDate(battle.start)}, ${writtenTime(battle.start)} to ${writtenTime(battle.end)}${battle.skirmish ? " · Skirmish" : ""}`),
      make("p", "", `${counted(battle.pilots, "pilot")}, ${counted(battle.killmails, "killmail")}, ${isk(totalISK(battle))} lost`),
      make("p", "", sideLine("a", battle)), make("p", "", sideLine("b", battle)));
    if (battle.others.iskLost > 0) panel.append(make("p", "", sideLine("others", battle)));
    if (structures(battle) > 0) panel.append(make("p", "", `${counted(structures(battle), "structure")} lost`));
    if (battle.marks.length > 0) {
      const marks = make("div", "marks");
      marks.append(...battle.marks.map(markChip));
      panel.append(marks);
      for (const mark of battle.marks) panel.append(make("p", "note", [markWhy(mark).replace(/\.$/, ""), ...mark.notes].join(". ")));
    }
    if (battle.otherSystems.length) panel.append(make("p", "note", `Also in ${battle.otherSystems.join(", ")}`));
    for (const note of battle.notes) panel.append(make("p", "battle-note", note));
    const links = make("p", "links");
    for (const link of battle.links) {
      const anchor = make("a", "", link.label);
      anchor.href = link.url;
      anchor.target = "_blank";
      anchor.rel = "noopener noreferrer";
      links.append(anchor);
    }
    panel.append(links);
    content.append(panel);
    open = panel;
  };
  node.on("plotly_click", (event) => {
    const point = event.points[0];
    if (point && BATTLES[point.customdata]) showBattle(BATTLES[point.customdata]);
  });
};
