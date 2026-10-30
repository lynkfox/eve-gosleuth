// The new user tour: a darkened window with the control being explained left lit, a dialogue box in a corner, and a pretend search whose report it pages through.
// Loaded after main.js, whose functions it calls. While it runs, tourBackend (main.js) stands in for the Go backend, answering from the demo files in tour/,
// so the tour reaches no API, cache, or setting. See .claude/docs/tour-plan.md.

/*
  TOUR_STEPS are the tour's steps, in order. Each is
  {
    id: "search-bar",                  // unique, for debugging and jumps
    target: "#search-input",           // the element lit up: a selector, an element, a function returning either, or a list of them (lit as one box). None centers the box with nothing lit.
    title: "Search",
    text: "...",                       // a string, or an array of strings for paragraphs
    before: async (tour) => {},        // sets the stage for this step: type, run the search, select a tab. Run again on a replay.
    after: (tour) => {},               // optional: tidies what before opened, ie closes the limits menu, when leaving the step either way
  }
  A step must be rebuildable from the steps before it: Back and the dots replay steps 0 to N-1 from a clean stage with tour.quick set,
  which skips every animation and wait, so the screen is exactly what a forward walk showed.
  Helpers for steps: tourSleep(ms), tourType(text), tourRunSearch(), and tour.demo.alliance / tour.demo.system, the demo files. A step that runs a search sets tour.runMs, how long it takes.
*/
const TOUR_STEPS = [
  {
    id: "welcome",
    title: "Welcome to GoSleuth",
    text: [
      "GoSleuth reads a pilot's, corporation's, alliance's, or system's killmails and turns them into intel: who flies with whom, where, in what, and how often.",
      "This tour runs a pretend search with made-up players and walks through the report. Nothing in it is real, and nothing you see changes your settings.",
    ],
  },
  {
    id: "search-bar",
    target: "#search-input",
    title: "The search bar",
    text: "Type a name, a corp or alliance ticker, or paste a zKillboard or EveWho link, and press Search. Pilots, corporations, alliances, and systems all work.",
  },
  {
    id: "recent-searches",
    target: ["#recent-toggle", "#recent-list"],
    title: "Recent searches",
    text: "This arrow lists the targets you searched before. Pick one to search it again.",
    before: () => showRecent(true),
    after: () => setRecentOpen(false),
  },
  {
    id: "search-limits",
    target: ["#limits-button", "#limits-menu"],
    title: "Search limits",
    text: "How many killmails a search collects, and how far back, are set here for the next search. Fewer means a faster report.",
    before: () => { if (el("limits-menu").hidden) toggleLimitsMenu(); },
    after: () => closeLimitsMenu(),
  },
  {
    id: "type-name",
    target: ["#search-input", "#search-button"],
    title: "Searching for an alliance",
    text: "The tour types an alliance's name for you. Next presses Search.",
    before: async (t) => {
      setRecentOpen(false);
      await tourType(t.demo.alliance.searchName);
      if (tourStopped(t)) return;
      searchTypeHint = t.demo.alliance.target.type;
      searchIdHint = t.demo.alliance.target.id;
    },
  },
  {
    id: "search-runs",
    target: "#ticker",
    title: "The search runs",
    text: [
      "GoSleuth pages through zKillboard for the killmails, fills in what they leave out from the ESI, and looks up the names. The bar at the bottom shows each step, with the time it has taken. Stop builds the report from what is collected so far.",
      "This one is pretend, and takes a few seconds.",
    ],
    before: async (t) => {
      t.runMs = 3000;
      await tourRunSearch();
    },
  },
  {
    id: "target",
    target: "#target",
    title: "The target",
    text: [
      "The alliance found, with its executor corp and CEO. Corps show their alliance, pilots their corp, and the icons beside each open it on zKillboard, EveWho, or Dotlan.",
      "Affiliates you link to an alliance or corp, ie its alt or holding corps, show at the right, and can be searched with it.",
    ],
  },
  {
    id: "target-pull",
    target: "#target-pull",
    title: "More room",
    text: "This bar pulls the target's box shut, to give the report more room. Click it or drag it; the target's name stays on the bar.",
  },
  {
    id: "glance",
    target: ["#tab-glance", "#tab-main"],
    title: "At a glance",
    text: [
      "The report opens here: the target's profile across the top, then the first few rows of each tab.",
      "Each card's Open goes to its full tab.",
    ],
    before: () => selectTab("tab-glance"),
  },
  {
    id: "sidebar",
    target: "#tab-sidebar",
    title: "The tabs",
    text: [
      "Each tab looks at the killmails one way. The number beside a tab is how many rows it has.",
      "Drag a tab, or use Alt+↑/↓, to put them in your own order. « shrinks the sidebar to its icons.",
    ],
  },
  tourTabStep("tab-killmails", "Killmails", [
    "Every killmail the search collected, newest first. Click one to open it on zKillboard.",
    "Kills, Losses, and All show one side only.",
  ], "#killmail-filter"),
  tourTabStep("tab-activity", "Activity", "When the killmails happened, by weekday and hour: when this alliance is out, and when it's not."),
  tourTabStep("tab-members", "Top Pilots", "The pilots on the most of its killmails. For a pilot, this tab is Top Fleetmates instead: who they fly with most."),
  tourTabStep("tab-systems", "Top Systems", "Where its killmails happen, with its kills and losses in each. Home is usually near the top."),
  tourTabStep("tab-battles", "Battles", "Killmails close together in time and place, put together into fights, with who was on each side."),
  tourTabStep("tab-capitals", "Capital Pilots", "The capitals its pilots were seen flying, and who flew them."),
  tourTabStep("tab-structures", "Structures", "Citadels and POSes it used on kills, helped destroy, and lost."),
  tourTabStep("tab-fits", "Fits", [
    "The fits it lost, by ship, with their tags and the stats worked out from them. Losses with the same fit are merged.",
    "Click a fit to open it in the fitting window.",
  ]),
  tourTabStep("tab-roster", "Members", "Its member corporations now, with their member counts."),
  tourTabStep("tab-allies", "Allies?", [
    "Alliances often on its kills: friends, or blues, or just third parties that turned up. A guess, not a fact.",
    "Right-click one to link it as an affiliate, or ignore it.",
  ]),
  {
    id: "menus",
    target: ["#tab-main"],
    title: "Right-click for more",
    text: "Nearly every pilot, corp, alliance, system, and killmail in a report has a right-click menu: search it, or open it on zKillboard, EveWho, or Dotlan.",
    before: () => selectTab("tab-members"),
  },
  {
    id: "help",
    target: ".panel-header",
    title: "What's this?",
    text: "Each tab's name sits here, with its color key, and a ? that explains the tab when you're not sure what it shows.",
  },
  {
    id: "report",
    target: "#export-pdf-top",
    title: "Generate Report",
    text: "A corp's or alliance's report can be saved as a PDF, to pass around.",
  },
  {
    id: "system",
    target: "#target",
    title: "Searching a system",
    text: [
      "A system's report is built the same way, from the killmails in it, but asks other questions: who lives here, who rats and does PI here, and, in wormholes, what its statics are.",
    ],
    before: async (t) => {
      await tourType(t.demo.system.searchName);
      if (tourStopped(t)) return;
      searchTypeHint = t.demo.system.target.type;
      searchIdHint = t.demo.system.target.id;
      t.runMs = 1200;
      await tourRunSearch();
    },
  },
  {
    id: "system-glance",
    target: ["#tab-glance", "#tab-main"],
    title: "A system at a glance",
    text: [
      "A system's profile has its security, region, and stargates, or for a wormhole its class and statics.",
      "Its cards are its own: who's on its killmails, who lives here, and its last structure lost.",
    ],
    before: () => selectTab("tab-glance"),
  },
  tourTabStep("tab-pilots", "Top Pilots and Corps", "The pilots, and under Top Corps the corporations, on the most of its killmails: who fights here.", "#tab-main"),
  tourTabStep("tab-activity", "Last structure destroyed", [
    "In a wormhole, Activity also shows the last citadel or POS destroyed here, however long ago, who owned it, and who killed it.",
    "A structure killed and none since can mean the hole changed hands, or was emptied.",
  ], "#structure-loss"),
  tourTabStep("tab-pi", "PI Pilots", "Haulers lost near a planet, and ships lost with PI aboard. Pilots who do their planets here likely live here, or nearby. Hover a PI tag to see the cargo."),
  tourTabStep("tab-ratters", "Ratters", "Ships lost here to rats: someone was running sites or anomalies. Like PI, a sign of who lives here. Hover a loss to see who was on it."),
  {
    id: "menu",
    target: ["#myintel-button", "#themes-button", "#preferences-button"],
    title: "The rest",
    text: [
      "My Intel keeps your notes on pilots, corps, and alliances, and can share them.",
      "Theme changes the colors. Preferences sets your search defaults, times, and cache.",
    ],
  },
  {
    id: "about",
    target: "#brand",
    title: "That's the tour",
    text: [
      "The bear opens About: what GoSleuth is, what killmails can't tell you, and how to report a bug. Take the tour again is there too.",
      "Good hunting.",
    ],
  },
];

// tourTabStep is a step for one of the report's tabs: it selects the tab and lights it with its panel, or only with part of the panel when focus is given
function tourTabStep(tabId, title, text, focus = "#tab-main") {
  return {
    id: tabId,
    target: [`#${tabId}`, focus],
    title,
    text,
    before: () => selectTab(tabId),
  };
}

// tour is the running tour's state, null when none runs
let tour = null;

// TOUR_PAD is how far, in pixels, the highlight reaches past what it lights
const TOUR_PAD = 5;
// TOUR_MARGIN is the space, in pixels, the dialogue box keeps from the window's edges
const TOUR_MARGIN = 16;

// tourOfferThen runs when the first launch offer closes, or after the tour it led to
let tourOfferThen = null;

// tourSleep waits ms milliseconds, or the next tick only on a replay or when the tour is ending
function tourSleep(ms) {
  const quick = !tour || tour.quick || tour.ending;
  return new Promise((resolve) => setTimeout(resolve, quick ? 0 : ms));
}

// tourCopy returns a copy of a piece of demo data, as the app code changes the reports it is given
function tourCopy(value) {
  return typeof structuredClone === "function" ? structuredClone(value) : JSON.parse(JSON.stringify(value));
}

// tourType types text into the search box a letter at a time, or at once on a replay
async function tourType(text) {
  const t = tour;
  if (tourStopped(t)) return;
  const input = el("search-input");
  input.value = "";
  if (!t.quick) {
    for (const letter of text) {
      input.value += letter;
      await tourSleep(55);
      // Skip pressed while typing: the search box is the user's again
      if (tourStopped(t)) return;
    }
  }
  input.value = text;
}

// tourStopped is whether the tour t has ended or is ending, so a step still setting up leaves the screen alone
function tourStopped(t) {
  return !t || tour !== t || t.ending;
}

// tourRunSearch presses Search, so main.js's own search() runs against the stand-in backend, and waits for it to finish
async function tourRunSearch() {
  // once the tour is ending, a search would reach the real backend
  if (tourStopped(tour)) return;
  el("search-form").requestSubmit();
  // search() starts after a tick of its own
  await new Promise((resolve) => setTimeout(resolve, 0));
  while (searching) await new Promise((resolve) => setTimeout(resolve, 40));
}

/*
  tourStandIn builds the stand-in for the Go backend. The demo's own methods answer from the demo files; a few read-only ones pass through to the real backend;
  every other method, including any added later, does nothing and resolves null, logging its name at debug level.
*/
function tourStandIn(t) {
  const demos = [t.demo.alliance, t.demo.system];
  const demoFor = (id) => demos.find((demo) => demo.target.id === id);
  const sample = [
    { name: t.demo.system.searchName, type: "SYSTEM" },
    { name: "Jita", type: "SYSTEM" },
    { name: "Lunar Vanguard", type: "CORPORATION" },
    { name: "Mira Valdane", type: "PILOT" },
  ];
  const noHoldings = { holdings: [], heldFor: [] };

  // progress pushes the scripted lines, spread over the share of the run asked for, as the backend's progress reports are
  const progress = async (lines, share) => {
    const each = (t.runMs * share) / Math.max(lines.length, 1);
    for (const line of lines) {
      await tourSleep(each);
      tickerPush(line, true);
    }
  };

  const own = {
    // the demo whose name or id is asked for
    async SearchTarget(name, type, id) {
      const wanted = String(name || "").trim().toLowerCase();
      const demo = demos.find((candidate) => candidate.target.id === id || candidate.searchName.toLowerCase() === wanted || candidate.target.display.Name.toLowerCase() === wanted);
      if (!demo) throw new Error("The tour has no pretend result for that name.");
      await progress([`Looking up ${demo.target.display.Name}`], 0.2);
      return tourCopy(demo.target);
    },
    // the demo's report, after the scripted lines a real one pushes
    async CollectIntel(id) {
      const demo = demoFor(id);
      if (!demo) throw new Error("The tour has no pretend report for that target.");
      const total = (demo.report.killmails || []).length;
      const processed = [0.1, 0.3, 0.5, 0.75, 1].map((part) => `Processed killmail ${Math.max(1, Math.round(total * part))} of ${total}`);
      await progress(["Fetching zKillboard page 1", "Fetching zKillboard page 2", ...processed, "Looking up names", "Building the report"], 0.8);
      return tourCopy(demo.report);
    },
    // a slice of the demo's members, as the Members tab asks for them a block at a time
    async GetMembers(id, type, offset, count) {
      const demo = demoFor(id);
      // an empty answer would end the list it is for there
      if (!demo) throw new Error("The tour has no members for that target.");
      const members = demo.members || { rows: [], corporations: [] };
      const corporations = type === "ALLIANCE";
      const all = (corporations ? members.corporations : members.rows) || [];
      const page = tourCopy(all.slice(offset, offset + count));
      return { rows: corporations ? [] : page, corporations: corporations ? page : [], total: all.length, hasMore: offset + count < all.length };
    },
    async RankAllies(id, type, showKilled) {
      const demo = demoFor(id);
      if (!demo || !demo.allies) return { allies: null, ignoredAllies: [], killedAllies: 0, holdings: [] };
      return tourCopy(showKilled && demo.alliesWithKilled ? demo.alliesWithKilled : demo.allies);
    },
    async GetHoldings(id) {
      const demo = demoFor(id);
      return tourCopy((demo && demo.holdings) || noHoldings);
    },
    // a fixed list, so the recent searches step shows something to a new user, and the real list is never read or changed
    async GetRecentSearches() { return tourCopy(sample); },
    async RemoveRecentSearch() { return tourCopy(sample); },
    // nothing saved is shown, or added to
    async GetMyIntel() { return []; },
    async SaveHolding() { return tourCopy(noHoldings); },
    async SaveIgnoredAllies() { return null; },
    async LookupEntity() { throw new Error("Not available in the tour."); },
  };

  // PASS_THROUGH are the read-only methods the real backend answers: the page's own settings and version, and the page's error log
  const passThrough = new Set(["GetSettings", "Version", "IsNewUser", "LogFrontendError"]);

  return new Proxy(own, {
    get(target, name) {
      if (typeof name === "symbol") return undefined;
      if (name in target) return target[name];
      if (passThrough.has(name)) return (...args) => window.go.gui.App[name](...args);
      return () => {
        console.debug(`tour: ${name} did nothing`);
        return Promise.resolve(null);
      };
    },
  });
}

// tourSelectedTab is the id of the sidebar tab shown, "" when none
function tourSelectedTab() {
  const tab = document.querySelector("#tab-list .tab[aria-selected=\"true\"]");
  return tab ? tab.id : "";
}

// tourStash saves what is on screen, to put back when the tour ends
function tourStash() {
  return {
    search: el("search-input").value,
    typeHint: searchTypeHint,
    idHint: searchIdHint,
    target: renderedTarget,
    shown: shownTarget,
    report: intelReport,
    intel: intelTarget,
    roster,
    tab: tourSelectedTab(),
    sidebar: sidebarCollapsed,
    targetCollapsed,
    holdingsTucked,
    profileOnly: profileOnlyReason,
    killmailPage,
    killmailFilter,
    battlePage,
    collapsePreference: preferences.collapseTarget,
    order: shownTabOrder(),
    status: { text: el("status").textContent, error: el("status").classList.contains("error") },
    warning: { text: el("report-warning").textContent, hidden: el("report-warning").hidden },
    pdf: el("export-pdf-top").hidden,
    hidden: {
      empty: el("empty").hidden,
      wrap: el("target-wrap").hidden,
      pull: el("target-pull").hidden,
      intel: el("intel").hidden,
      tabs: el("intel-tabs").hidden,
      roster: el("tab-roster").hidden,
    },
    tickerMessage: el("ticker-message").textContent,
    tickerHistory: [...el("ticker-history").children],
  };
}

// tourClearStage puts the screen to what a fresh launch shows, before any step has run
function tourClearStage() {
  closeLimitsMenu();
  setRecentOpen(false);
  setHelpWindowOpen(false);
  el("search-input").value = "";
  searchTypeHint = "";
  searchIdHint = 0;
  setStatus("");
  clearIntel();
  shownTarget = null;
  el("tab-roster").hidden = true;
  el("target-wrap").hidden = true;
  el("target-pull").hidden = true;
  el("empty").hidden = false;
  setSidebarCollapsed(false);
  setTargetCollapsed(false);
  setHoldingsTucked(false);
  arrangeTabs(DEFAULT_TAB_ORDER);
  el("ticker-history").replaceChildren();
  el("ticker-message").textContent = "Ready";
}

// tourRestore puts back what tourStash saved
function tourRestore(saved) {
  closeLimitsMenu();
  setRecentOpen(false);
  setHelpWindowOpen(false);
  preferences.collapseTarget = saved.collapsePreference;
  clearIntel();
  shownTarget = saved.shown;
  roster = null;

  if (saved.report && saved.target) {
    // as search() draws a finished search
    renderTarget(saved.target);
    renderIntel(saved.report);
    // the pages and filter the user was on, which renderIntel starts again
    killmailFilter = saved.killmailFilter;
    showKillmailPage(saved.killmailPage);
    showBattlePage(saved.battlePage);
    showReportWarning(saved.report);
    setExportPDF(!saved.pdf);
    intelTarget = saved.intel;
    if (saved.roster) {
      startRoster(saved.roster.id, saved.roster.type);
      roster = saved.roster;
      renderRosterPage();
    } else {
      el("tab-roster").hidden = saved.hidden.roster;
    }
    el("report-warning").textContent = saved.warning.text;
    el("report-warning").hidden = saved.warning.hidden;
  } else {
    if (saved.target) renderTarget(saved.target);
    el("empty").hidden = saved.hidden.empty;
    el("target-wrap").hidden = saved.hidden.wrap;
    el("target-pull").hidden = saved.hidden.pull;
    el("intel").hidden = saved.hidden.intel;
    el("intel-tabs").hidden = saved.hidden.tabs;
    el("tab-roster").hidden = saved.hidden.roster;
    intelTarget = saved.intel;
    roster = saved.roster;
    // a search that ended without a report showed only the target's profile and why, which the tour's report drew over
    if (saved.profileOnly !== null) showProfileOnly(saved.profileOnly);
  }

  arrangeTabs(saved.order);
  if (saved.tab) selectTab(saved.tab);
  setSidebarCollapsed(saved.sidebar);
  setTargetCollapsed(saved.targetCollapsed);
  setHoldingsTucked(saved.holdingsTucked);
  el("search-input").value = saved.search;
  searchTypeHint = saved.typeHint;
  searchIdHint = saved.idHint;
  setStatus(saved.status.text, saved.status.error);
  el("ticker-message").textContent = saved.tickerMessage;
  el("ticker-history").replaceChildren(...saved.tickerHistory);
}

// tourStepTarget is the lit area of a step, the box around its target elements in window coordinates, or null when it has none or none shows
function tourStepTarget(step) {
  if (!step || !step.target) return null;
  const given = typeof step.target === "function" ? step.target() : step.target;
  let area = null;
  for (const item of [].concat(given)) {
    const node = typeof item === "string" ? document.querySelector(item) : item;
    if (!node) continue;
    const rect = node.getBoundingClientRect();
    if (rect.width === 0 && rect.height === 0) continue;
    area = area
      ? { left: Math.min(area.left, rect.left), top: Math.min(area.top, rect.top), right: Math.max(area.right, rect.right), bottom: Math.max(area.bottom, rect.bottom) }
      : { left: rect.left, top: rect.top, right: rect.right, bottom: rect.bottom };
  }
  if (!area) return null;
  return { left: area.left - TOUR_PAD, top: area.top - TOUR_PAD, right: area.right + TOUR_PAD, bottom: area.bottom + TOUR_PAD };
}

// tourStepNodes are the elements of a step's target, for watching their size
function tourStepNodes(step) {
  if (!step || !step.target) return [];
  const given = typeof step.target === "function" ? step.target() : step.target;
  return [].concat(given).map((item) => (typeof item === "string" ? document.querySelector(item) : item)).filter(Boolean);
}

// tourPlace puts the highlight over the step's target and the dialogue box in the corner farthest from it, or centered when nothing is lit
function tourPlace() {
  if (!tour) return;
  const step = TOUR_STEPS[tour.step];
  const area = tourStepTarget(step);
  const highlight = el("tour-highlight");
  highlight.hidden = !area;
  el("tour-catch").classList.toggle("dim", !area);
  if (area) {
    highlight.style.left = `${area.left}px`;
    highlight.style.top = `${area.top}px`;
    highlight.style.width = `${area.right - area.left}px`;
    highlight.style.height = `${area.bottom - area.top}px`;
  }

  const box = el("tour-box");
  const width = box.offsetWidth;
  const height = box.offsetHeight;
  const viewWidth = window.innerWidth;
  const viewHeight = window.innerHeight;
  let left = (viewWidth - width) / 2;
  let top = (viewHeight - height) / 2;
  if (area) {
    const m = TOUR_MARGIN;
    const corners = [[m, m], [viewWidth - width - m, m], [m, viewHeight - height - m], [viewWidth - width - m, viewHeight - height - m]];
    let best = Infinity;
    for (const [x, y] of corners) {
      const overlapWidth = Math.max(0, Math.min(x + width, area.right) - Math.max(x, area.left));
      const overlapHeight = Math.max(0, Math.min(y + height, area.bottom) - Math.max(y, area.top));
      const distance = Math.hypot(x + width / 2 - (area.left + area.right) / 2, y + height / 2 - (area.top + area.bottom) / 2);
      // the least covered corner, then the farthest
      const score = overlapWidth * overlapHeight * 1000 - distance;
      if (score < best) {
        best = score;
        [left, top] = [x, y];
      }
    }
  }
  box.style.left = `${Math.max(0, left)}px`;
  box.style.top = `${Math.max(0, top)}px`;
}

// tourWatch places the step again whenever its target or the report's area changes size, and for a moment after, as the sidebar and target panel animate
function tourWatch() {
  if (!tour) return;
  if (tour.observer) tour.observer.disconnect();
  if (typeof ResizeObserver === "function") {
    tour.observer = new ResizeObserver(() => tourPlace());
    tour.observer.observe(el("tab-main"));
    for (const node of tourStepNodes(TOUR_STEPS[tour.step])) tour.observer.observe(node);
  }
  tourPlace();
  const t = tour;
  let frames = 0;
  const follow = () => {
    if (tour !== t) return;
    tourPlace();
    if (++frames < 40) requestAnimationFrame(follow);
  };
  requestAnimationFrame(follow);
}

// tourDots draws the progress dots: filled once viewed, the current one larger, the rest hollow and inert
function tourDots() {
  const t = tour;
  // each dot is the button in its list item
  const dots = [...el("tour-dots").querySelectorAll(".tour-dot")];
  TOUR_STEPS.forEach((step, index) => {
    const dot = dots[index];
    dot.classList.toggle("viewed", Boolean(t.viewed[index]));
    dot.classList.toggle("current", index === t.step);
    dot.disabled = !t.viewed[index] || index === t.step || t.busy;
    if (index === t.step) dot.setAttribute("aria-current", "step");
    else dot.removeAttribute("aria-current");
  });
}

// tourNav sets the Back and Next buttons: Back gone on the first step, Next saying Finish on the last, both off while a step is being set up
function tourNav() {
  const t = tour;
  el("tour-back").hidden = t.step <= 0;
  el("tour-back").disabled = t.busy;
  el("tour-next").textContent = t.step === TOUR_STEPS.length - 1 ? "Finish" : "Next";
  el("tour-next").disabled = t.busy;
  tourDots();
}

// tourFocus keeps keyboard focus in the dialogue box: on the answer to Leave the tour, else on Next, or on the box while Next waits
function tourFocus() {
  if (!tour) return;
  const target = tour.leaving ? el("tour-leave-no") : (el("tour-next").disabled ? el("tour-box") : el("tour-next"));
  target.focus({ preventScroll: true });
}

// tourShowText writes a step's title and paragraphs into the box
function tourShowText(step) {
  el("tour-title").textContent = step.title;
  const paragraphs = [].concat(step.text).map((text) => {
    const paragraph = document.createElement("p");
    paragraph.textContent = text;
    return paragraph;
  });
  el("tour-text").replaceChildren(...paragraphs);
  el("tour-count").textContent = `Step ${tour.step + 1} of ${TOUR_STEPS.length}`;
}

// tourRunHook runs a step's before or after, reporting a failure in the console so one broken step does not strand the tour
async function tourRunHook(step, hook) {
  if (typeof step[hook] !== "function") return;
  try {
    await step[hook](tour);
  } catch (err) {
    console.error(`tour: step ${step.id} ${hook} failed`, err);
  }
}

/*
  tourGo shows step index. The next step just follows the one shown; anything else (Back, a dot) clears the stage and replays the steps before it,
  quickly, so the screen is what a forward walk shows. Next, Back, and the dots wait while it runs.
*/
async function tourGo(index) {
  const t = tour;
  if (!t || t.busy || t.ending) return;
  if (index >= TOUR_STEPS.length) {
    endTour();
    return;
  }
  t.busy = true;
  const from = t.step;
  if (from >= 0) tourNav();
  try {
    if (from >= 0 && index === from + 1) {
      await tourRunHook(TOUR_STEPS[from], "after");
    } else if (from >= 0) {
      await tourRunHook(TOUR_STEPS[from], "after");
      t.quick = true;
      tourClearStage();
      for (let i = 0; i < index; i++) {
        await tourRunHook(TOUR_STEPS[i], "before");
        await tourRunHook(TOUR_STEPS[i], "after");
        if (tourStopped(t)) return;
      }
      t.quick = false;
    }
    if (tour !== t) return;

    const step = TOUR_STEPS[index];
    t.step = index;
    t.viewed[index] = true;
    el("tour-step").hidden = false;
    el("tour-leave").hidden = true;
    el("tour-box").classList.remove("leaving");
    t.leaving = false;
    tourShowText(step);
    tourNav();
    tourWatch();
    // the lit control may appear only as the step sets up, so it is placed again while that runs
    const follow = setInterval(tourPlace, 200);
    try {
      await tourRunHook(step, "before");
    } finally {
      clearInterval(follow);
    }
    if (tour !== t) return;
    const first = tourStepNodes(step)[0];
    if (first && first.scrollIntoView) first.scrollIntoView({ block: "nearest", inline: "nearest" });
  } finally {
    t.quick = false;
    t.busy = false;
    if (tour === t) {
      tourNav();
      tourWatch();
      tourFocus();
    }
  }
}

// tourAskLeave swaps the box's body for the question Leave the tour?, Keep going focused
function tourAskLeave() {
  if (!tour || tour.leaving) return;
  tour.leaving = true;
  el("tour-step").hidden = true;
  el("tour-leave").hidden = false;
  el("tour-box").classList.add("leaving");
  tourPlace();
  tourFocus();
}

// tourKeepGoing puts the step back in place of the question
function tourKeepGoing() {
  if (!tour || !tour.leaving) return;
  tour.leaving = false;
  el("tour-step").hidden = false;
  el("tour-leave").hidden = true;
  el("tour-box").classList.remove("leaving");
  tourPlace();
  tourFocus();
}

// tourNext goes on to the next step, or ends the tour from the last
function tourNext() {
  if (!tour || tour.busy || tour.leaving) return;
  tourGo(tour.step + 1);
}

// tourBack goes to the step before
function tourBack() {
  if (!tour || tour.busy || tour.leaving || tour.step <= 0) return;
  tourGo(tour.step - 1);
}

// tourKeydown drives the tour from the keyboard, and keeps every key from the app's own handlers while it runs
function tourKeydown(event) {
  if (!tour) return;
  event.stopPropagation();
  if (tour.ending) {
    event.preventDefault();
    return;
  }
  const onButton = el("tour-box").contains(event.target) && event.target.tagName === "BUTTON";
  switch (event.key) {
    case "Escape":
      event.preventDefault();
      if (tour.leaving) tourKeepGoing();
      else tourAskLeave();
      break;
    case "Enter":
      // a focused button does its own thing
      if (onButton) break;
      event.preventDefault();
      if (tour.leaving) endTour();
      else tourNext();
      break;
    case "ArrowRight":
      event.preventDefault();
      tourNext();
      break;
    case "ArrowLeft":
      event.preventDefault();
      tourBack();
      break;
    case "Tab": {
      // focus cycles through the box's buttons
      const buttons = [...el("tour-box").querySelectorAll("button")].filter((button) => !button.disabled && button.offsetParent !== null);
      if (buttons.length === 0) {
        event.preventDefault();
        break;
      }
      const at = buttons.indexOf(document.activeElement);
      const next = event.shiftKey ? (at <= 0 ? buttons.length - 1 : at - 1) : (at === buttons.length - 1 ? 0 : at + 1);
      event.preventDefault();
      buttons[next].focus();
      break;
    }
    default:
      if (!onButton) event.preventDefault();
  }
}

/*
  startTour starts the tour from its first step: it saves what is on screen (the search box, report, selected tab, sidebar and target panel states,
  tab order) to put back at the end, sets tourBackend, and shows the overlay. onEnd, if given, runs once the tour has ended and the screen is put back.
*/
async function startTour(onEnd) {
  const then = typeof onEnd === "function" ? onEnd : null;
  if (tour || searching) {
    if (then) then();
    return;
  }

  let demo;
  try {
    const [alliance, system] = await Promise.all(["tour/alliance.json", "tour/system.json"].map(async (path) => {
      const response = await fetch(path);
      if (!response.ok) throw new Error(`${path}: ${response.status}`);
      return response.json();
    }));
    demo = { alliance, system };
  } catch (err) {
    setStatus(`The tour's pretend data could not be loaded. ${err}`, true);
    if (then) then();
    return;
  }
  // a search may have started while the files loaded
  if (tour || searching) {
    if (then) then();
    return;
  }

  const t = {
    step: -1,
    viewed: [],
    quick: false,
    busy: false,
    leaving: false,
    ending: false,
    runMs: 3000,
    demo,
    saved: tourStash(),
    onEnd: then,
    observer: null,
  };
  tour = t;
  tourBackend = tourStandIn(t);
  // the report opens as the preferences would not have it, for every viewer the same
  preferences.collapseTarget = false;
  tourClearStage();
  loadRecentSearches();

  el("tour-dots").replaceChildren(...TOUR_STEPS.map((step, index) => {
    const item = document.createElement("li");
    const dot = document.createElement("button");
    dot.type = "button";
    dot.className = "tour-dot";
    dot.setAttribute("aria-label", `Step ${index + 1}: ${step.title}`);
    dot.title = step.title;
    dot.addEventListener("click", () => {
      if (tour && t.viewed[index] && !tour.busy && !tour.leaving) tourGo(index);
    });
    item.append(dot);
    return item;
  }));
  el("tour-step").hidden = false;
  el("tour-leave").hidden = true;
  el("tour-box").classList.remove("leaving");
  el("tour").hidden = false;
  await tourGo(0);
}

// endTour ends the tour, from Skip, Leave, or Finish: it waits for a pretend search still running, hides the overlay, clears tourBackend, and puts back what startTour saved
async function endTour() {
  const t = tour;
  if (!t || t.ending) return;
  t.ending = true;
  el("tour-box").hidden = true;
  // the stand-in answers at once now, so a step setting up, a search running, and the Members tab's pages all finish before the real backend is back
  for (let waited = 0; (t.busy || searching || (roster && roster.loading.size > 0)) && waited < 100; waited++) {
    await new Promise((resolve) => setTimeout(resolve, 30));
  }

  if (t.observer) t.observer.disconnect();
  el("tour").hidden = true;
  el("tour-highlight").hidden = true;
  el("tour-box").hidden = false;
  tour = null;
  // the real backend is back before the restore, so the user's Members list loads its pages from it
  tourBackend = null;
  tourRestore(t.saved);
  loadRecentSearches();
  if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
  if (t.onEnd) t.onEnd();
}

// offerTour opens the first launch offer of the tour (#tour-offer); then runs once it closes either way, after the tour if it was taken
function offerTour(then) {
  tourOfferThen = typeof then === "function" ? then : null;
  el("tour-offer").returnValue = "";
  el("tour-offer").showModal();
}

// the offer: Take the tour starts it, anything else closes it, and either way the launch goes on
el("tour-offer-start").addEventListener("click", () => el("tour-offer").close("start"));
el("tour-offer-later").addEventListener("click", () => el("tour-offer").close("later"));
el("tour-offer-close").addEventListener("click", () => el("tour-offer").close("later"));
el("tour-offer").addEventListener("close", () => {
  const next = tourOfferThen;
  tourOfferThen = null;
  if (el("tour-offer").returnValue === "start") startTour(next);
  else if (next) next();
});

// the box: its buttons, the Leave question, and the keys
el("tour-box").setAttribute("tabindex", "-1");
el("tour-next").addEventListener("click", tourNext);
el("tour-back").addEventListener("click", tourBack);
el("tour-skip").addEventListener("click", () => endTour());
el("tour-leave-yes").addEventListener("click", () => endTour());
el("tour-leave-no").addEventListener("click", tourKeepGoing);
document.addEventListener("keydown", tourKeydown, true);
document.addEventListener("keyup", (event) => { if (tour) event.stopPropagation(); }, true);
// focus never leaves the box
document.addEventListener("focusin", (event) => {
  if (tour && !tour.ending && !el("tour-box").contains(event.target)) tourFocus();
}, true);
// nothing in the overlay reaches the app's document handlers, ie the ones that close menus on a click
for (const type of ["click", "mousedown", "mouseup", "pointerdown", "pointerup", "dblclick"]) {
  el("tour").addEventListener(type, (event) => event.stopPropagation());
}
el("tour").addEventListener("contextmenu", (event) => {
  event.preventDefault();
  event.stopPropagation();
});
window.addEventListener("resize", () => tourPlace());
