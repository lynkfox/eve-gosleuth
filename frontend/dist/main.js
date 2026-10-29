// Bound Go methods are injected by the Wails runtime as window.go.<package>.<struct>.<method>, each returning a Promise
const backend = () => window.go.gui.App;

// reportFrontendError sends a JavaScript error to the backend's log, and so to the debug log, as it is otherwise invisible. First, so it catches errors as the rest of this file loads.
function reportFrontendError(message, error) {
  const stack = error && error.stack ? `\n${error.stack}` : "";
  try {
    backend().LogFrontendError(`${message}${stack}`).catch(() => {});
  } catch {
    // the backend is not there yet, as in a test page
  }
}
window.addEventListener("error", (event) => reportFrontendError(`${event.message} at ${event.filename}:${event.lineno}:${event.colno}`, event.error));
window.addEventListener("unhandledrejection", (event) => reportFrontendError(`Unhandled rejection: ${event.reason}`, event.reason));

const el = (id) => document.getElementById(id);

const TICKER_HISTORY_LIMIT = 50;
const KILLMAILS_PER_PAGE = 10;

let killmails = [];
let killmailPage = 0;

function setStatus(message, isError = false) {
  const status = el("status");
  status.textContent = message;
  status.classList.toggle("error", isError);
  status.hidden = !message;
}

function setBusy(busy) {
  searching = busy;
  el("ticker-stop").hidden = !busy;
  el("ticker-stop").disabled = false;
  el("ticker-stop").textContent = "Stop";
  if (busy) startSearchTimer();
  else stopSearchTimer();
  el("search-button").disabled = busy;
  el("search-input").disabled = busy;
  el("recent-toggle").disabled = busy;
  if (busy) setRecentOpen(false);
  el("ticker").classList.toggle("busy", busy);
}

// searching is true while a search runs, see setBusy
let searching = false;
// stopRequested is true once the running search's Stop was pressed
let stopRequested = false;

// searchTimer counts the running search's time in the ticker, the interval updating it while one runs
let searchTimer = null;

// startSearchTimer shows the ticker's timer from 0:00, counting up each second
function startSearchTimer() {
  clearInterval(searchTimer);
  const started = performance.now();
  const timer = el("ticker-timer");
  const show = () => { timer.textContent = formatTimer(performance.now() - started); };
  show();
  timer.classList.remove("idle");
  searchTimer = setInterval(show, 1000);
}

// stopSearchTimer hides the timer once the search's own summary with its time is in the ticker, keeping its space
function stopSearchTimer() {
  clearInterval(searchTimer);
  searchTimer = null;
  el("ticker-timer").classList.add("idle");
}

// formatTimer writes elapsed time as the ticker's timer shows it, ie 0:07, 2:05, or 1:02:05
function formatTimer(milliseconds) {
  const total = Math.floor(milliseconds / 1000);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = pad2(total % 60);
  return hours ? `${hours}:${pad2(minutes)}:${seconds}` : `${minutes}:${seconds}`;
}

// stopSearch asks the backend to stop the running search where it is. It builds the report from what it has collected, if anything.
function stopSearch() {
  if (!searching || stopRequested) return;
  stopRequested = true;
  el("ticker-stop").disabled = true;
  el("ticker-stop").textContent = "Stopping…";
  // a search waiting on a question is answered by the stop
  for (const id of SEARCH_PROMPTS) {
    if (el(id).open) el(id).close("stop");
  }
  backend().StopSearch().catch(() => {});
}

/*
  tickerPush shows a status update in the ticker bar and adds it, timestamped, to the top of the history list.
  The bar itself has no time, since it would only change when a new update arrives.
  A background update, ie the backend's progress, only reaches the bar while a search runs, so a finished search's summary stays shown. Otherwise it only goes in the history.
*/
// TICKER_LATEST_ONLY are the kinds of line, by how they start, that the bar shows each of but the history keeps only the latest of in a row, ie each killmail processed.
// Each list's zKillboard pages follow a line naming the list, so one list's last page is kept before the next list's start.
const TICKER_LATEST_ONLY = ["Processed killmail ", "Fetching zKillboard page "];

function tickerPush(message, background = false) {
  const time = clockTime(timeParts(new Date().toISOString()), true);

  if (!background || searching) el("ticker-message").textContent = message;

  const entry = document.createElement("li");
  const stamp = document.createElement("time");
  stamp.textContent = time;
  const text = document.createElement("span");
  text.textContent = message;
  entry.append(stamp, text);

  // a line of a kind only the latest of which is kept replaces the one before it, rather than pushing the rest of the history down
  const history = el("ticker-history");
  const kind = TICKER_LATEST_ONLY.find((start) => message.startsWith(start)) || "";
  entry.dataset.kind = kind;
  const newest = history.firstElementChild;
  if (kind && newest && newest.dataset.kind === kind) {
    newest.replaceWith(entry);
  } else {
    history.prepend(entry);
  }
  while (history.children.length > TICKER_HISTORY_LIMIT) {
    history.lastElementChild.remove();
  }
}

function toggleTickerHistory() {
  const history = el("ticker-history");
  history.hidden = !history.hidden || history.children.length === 0;
}

// SEARCH_TYPES are the kinds of entity a search finds, with their names, ie Pilot for PILOT
const SEARCH_TYPES = { PILOT: "Pilot", CORPORATION: "Corporation", ALLIANCE: "Alliance", SYSTEM: "System" };

// isSearchable returns true if a search can find the entity type, ie PILOT
function isSearchable(type) {
  return Object.hasOwn(SEARCH_TYPES, type);
}

// searchTypeLabel is the name for an entity type, ie Pilot for PILOT
function searchTypeLabel(type) {
  return SEARCH_TYPES[type] || type;
}

// searchTypeHint and searchIdHint are the type and eve id of the entity filled into the search bar from the app, ie a recent search, so it is searched by id,
// needing no choosing between names and no name lookup, which the ESI can not do for every name. Typing clears them.
let searchTypeHint = "";
let searchIdHint = 0;

// offerSearch fills the search bar with an entity from the target panel, then asks before searching for it
function offerSearch(entity) {
  if (el("search-button").disabled) {
    return; // a search is already running
  }

  searchTypeHint = entity.type;
  searchIdHint = entity.id || 0;
  el("search-input").value = entity.name;

  el("search-confirm-target").textContent = `${searchTypeLabel(entity.type)}: ${entity.name}`;

  const dialog = el("search-confirm");
  dialog.returnValue = "";
  dialog.showModal();
}

// SITE_LINKS are the external sites an entity or row links to, shown as small icons in this order. Dotlan is for systems.
const SITE_LINKS = [
  { key: "zkillUrl", name: "zKillboard", icon: "icons/zkillboard.png" },
  { key: "evewhoUrl", name: "EveWho", icon: "icons/evewho.png" },
  { key: "dotlanUrl", name: "Dotlan", icon: "icons/dotlan.png" },
];

/*
  siteLinks builds the icon buttons that open an entity on each site in SITE_LINKS it has a url for
  name: what the links are for, ie a pilot's name, for screen readers. urls: { zkillUrl, evewhoUrl, dotlanUrl }, any may be missing.
  vertical: stacked, as beside a row's portrait, rather than in a line under the target's
*/
function siteLinks(name, urls, vertical = false) {
  const links = document.createElement("div");
  links.className = vertical ? "site-links site-links-vertical" : "site-links";
  for (const site of SITE_LINKS) {
    const url = urls[site.key];
    if (!url) continue;

    const link = document.createElement("button");
    link.type = "button";
    link.className = "site-link";
    link.title = `Open on ${site.name}`;
    link.setAttribute("aria-label", `Open ${name} on ${site.name}`);
    const icon = document.createElement("img");
    icon.src = site.icon;
    icon.alt = "";
    link.append(icon);
    link.addEventListener("click", () => window.runtime.BrowserOpenURL(url));
    links.append(link);
  }
  return links;
}

/*
  avatarCell is a table row's first cell: its site links stacked to the left of its portrait or logo
  imageUrl: the portrait or logo, if any. lazy: load the image only when scrolled to, for long lists
*/
function avatarCell(name, urls, imageUrl, lazy = false) {
  const cell = document.createElement("td");
  cell.className = "avatar";
  const inner = document.createElement("div");
  inner.className = "avatar-with-links";
  inner.append(siteLinks(name, urls, true));
  if (imageUrl) {
    const image = document.createElement("img");
    image.src = imageUrl;
    image.alt = "";
    if (lazy) image.loading = "lazy";
    inner.append(image);
  }
  cell.append(inner);
  return cell;
}

/*
  entityBlock builds a portrait with site link icons under it, beside a label and name. Used for the target and each related entity, ie a pilot's corp and alliance.

  entity: { label, name, type, imageUrl, zkillUrl, evewhoUrl, dotlanUrl }, links without a url are left out, and a searchable type makes the label pill start a search
  isTarget: the target is shown larger than its related entities
*/
// formatTicker writes a ticker as EVE does, [URSA] for a corporation and <URSA.> for an alliance
function formatTicker(type, ticker) {
  return type === "ALLIANCE" ? `<${ticker}>` : `[${ticker}]`;
}

// tickerTag is a corp's or alliance's ticker beside its name
function tickerTag(type, ticker) {
  const tag = document.createElement("span");
  tag.className = "entity-ticker";
  tag.textContent = formatTicker(type, ticker);
  return tag;
}

function entityBlock(entity, isTarget) {
  const block = document.createElement("div");
  block.className = isTarget ? "entity entity-target" : "entity";

  const side = document.createElement("div");
  side.className = "entity-side";

  if (entity.imageUrl) {
    const portrait = document.createElement("img");
    portrait.className = "portrait";
    portrait.src = entity.imageUrl;
    portrait.alt = "";
    side.append(portrait);
  }

  // the links stand in a column to the left of the portrait, keeping the target row short
  side.prepend(siteLinks(entity.name, entity, true));

  const text = document.createElement("div");
  text.className = "entity-text";
  // the target's own pill does nothing, it is already the search
  const searchable = !isTarget && isSearchable(entity.type);
  const label = document.createElement(searchable ? "button" : "span");
  label.className = "badge";
  label.textContent = entity.label;
  if (searchable) {
    label.type = "button";
    label.classList.add("badge-search");
    label.title = "Search for this";
    label.addEventListener("click", () => offerSearch(entity));
  }
  const name = document.createElement(isTarget ? "h1" : "p");
  name.className = "entity-name";
  name.textContent = entity.name;
  if (entity.spaceLabel || entity.ticker) {
    const nameLine = document.createElement("div");
    nameLine.className = "entity-name-line";
    nameLine.append(name);
    if (entity.spaceLabel) nameLine.append(spaceTag(entity.spaceLabel));
    if (entity.ticker) nameLine.append(tickerTag(entity.type, entity.ticker));
    text.append(label, nameLine);
  } else {
    text.append(label, name);
  }

  block.append(side, text);
  return block;
}

// corpHistoryGrid lays a pilot's corp history, or a corp's alliance history, out in columns: logo, bold name, then italic start and end dates
function corpHistoryGrid(history) {
  const grid = document.createElement("div");
  grid.className = "corp-history";

  for (const stint of history) {
    const icon = document.createElement("img");
    icon.src = stint.IconUrl;
    icon.alt = "";

    const name = document.createElement("strong");
    name.textContent = stint.Name;
    name.title = stint.Name;

    const start = document.createElement("em");
    start.textContent = stint.Start;
    const arrow = document.createElement("span");
    arrow.className = "corp-history-arrow";
    arrow.textContent = "→";
    const end = document.createElement("em");
    end.textContent = stint.End;

    grid.append(icon, name, start, arrow, end);
  }

  return grid;
}

// stargateGrid lists the systems a k-space system's stargates lead to, with their security. Clicking a system offers a search for it.
function stargateGrid(stargates) {
  const grid = document.createElement("div");
  grid.className = "stargates";

  for (const gate of stargates) {
    const name = document.createElement("button");
    name.type = "button";
    name.className = "link-button";
    name.textContent = gate.Name;
    name.title = "Search for this";
    name.addEventListener("click", () => offerSearch({ type: "SYSTEM", name: gate.Name, id: gate.SystemID }));

    const security = document.createElement("span");
    security.className = securityClass(gate.SecurityBand);
    security.textContent = gate.SecurityStatus.toFixed(1);

    grid.append(name, security);
  }

  return grid;
}

// memberCorpGrid lists an alliance's member corps in columns: logo, name, and member count. Clicking a corp offers a search for it.
function memberCorpGrid(corps) {
  const grid = document.createElement("div");
  grid.className = "member-corps";

  for (const corp of corps) {
    const icon = document.createElement("img");
    icon.src = corp.IconUrl;
    icon.alt = "";
    icon.loading = "lazy"; // big alliances have hundreds of corps, most scrolled out of view

    const name = document.createElement("button");
    name.type = "button";
    name.className = "link-button";
    name.textContent = corp.Name;
    name.title = "Search for this";
    name.addEventListener("click", () => offerSearch({ type: "CORPORATION", name: corp.Name, id: corp.CorporationID }));

    const members = document.createElement("span");
    members.className = "member-count";
    members.textContent = `${corp.MemberCount.toLocaleString("en-US")} ${corp.MemberCount === 1 ? "member" : "members"}`;

    grid.append(icon, name, members);
  }

  return grid;
}

// detailGrid lays the target details out in label and value columns, coloring a security status value
function detailGrid(rows) {
  const grid = document.createElement("div");
  grid.className = "detail-grid";

  for (const row of rows) {
    const label = document.createElement("span");
    label.className = "detail-label";
    label.textContent = row.Label;

    const value = document.createElement("span");
    value.textContent = row.Value;
    if (row.SecurityBand) value.className = securityClass(row.SecurityBand);

    grid.append(label, value);
  }
  return grid;
}

function renderTarget(result) {
  const display = result.display;

  const target = { label: result.type, id: result.id, name: display.Name, type: result.type, imageUrl: display.TargetImageUrl, zkillUrl: display.ZkillUrl, evewhoUrl: display.EveWhoUrl, dotlanUrl: display.DotlanUrl, spaceLabel: display.SpaceLabel, ticker: display.Ticker };
  const related = (display.Related || []).map((entity) => ({
    label: entity.Title, id: entity.ID, name: entity.Name, type: entity.TypeName, imageUrl: entity.ImageUrl, zkillUrl: entity.ZkillUrl, evewhoUrl: entity.EveWhoUrl, ticker: entity.Ticker,
  }));
  const relatedGroup = document.createElement("div");
  relatedGroup.className = "related-entities";
  relatedGroup.append(...related.map((entity) => entityBlock(entity, false)));
  el("target-entities").replaceChildren(entityBlock(target, true), relatedGroup);
  el("left-details-title").textContent = display.LeftDetailsTitle;
  if (display.DetailRows && display.DetailRows.length > 0) {
    el("left-details-value").replaceChildren(detailGrid(display.DetailRows));
  } else {
    el("left-details-value").textContent = display.LeftDetailsValue;
  }
  el("right-details-title").textContent = display.RightDetailsTitle;
  if (display.CorpHistory && display.CorpHistory.length > 0) {
    el("right-details-value").replaceChildren(corpHistoryGrid(display.CorpHistory));
  } else if (display.AllianceHistory && display.AllianceHistory.length > 0) {
    el("right-details-value").replaceChildren(corpHistoryGrid(display.AllianceHistory));
  } else if (display.MemberCorps && display.MemberCorps.length > 0) {
    el("right-details-value").replaceChildren(memberCorpGrid(display.MemberCorps));
  } else if (display.Stargates && display.Stargates.length > 0) {
    el("right-details-value").replaceChildren(stargateGrid(display.Stargates));
  } else if (display.Statics && display.Statics.length > 0) {
    el("right-details-value").replaceChildren(...display.Statics.map((static_) => {
      const line = document.createElement("div");
      line.append(staticLabel(static_), ` → ${SPACE_ABBREVIATIONS[static_.destination] || static_.destination}`);
      return line;
    }));
  } else {
    el("right-details-value").textContent = display.RightDetailsValue;
  }

  shownTarget = { id: result.id, type: result.type, name: display.Name, imageUrl: display.TargetImageUrl };
  renderHoldings(result.holdings);

  el("empty").hidden = true;
  el("target-wrap").hidden = false;
  el("target-pull").hidden = false;
  // a panel pulled shut stays so for the next search, its bar naming the new target. The Collapse target preference shuts it at each search.
  setTargetCollapsed(targetCollapsed || preferences.collapseTarget);
}

// targetCollapsed is true while the pull bar has the target panel pulled shut
let targetCollapsed = false;
// holdingsTucked is true while the pull bar has the target's holdings put away, the first step shut for a target with any. It stays for the next search only if that target has holdings too.
let holdingsTucked = false;
// targetHasHoldings is true while the target shown has holding links saved, at either end
let targetHasHoldings = false;
// holdingsShownFor and holdingsCount are the target whose holdings were last drawn and how many links it had, to tell when one is added
let holdingsShownFor = null;
let holdingsCount = 0;

// PULL_DISTANCE is how far, in pixels, the pull bar is dragged up or down to shut or open the target panel
const PULL_DISTANCE = 24;
// TARGET_SLIDE_MS is how long the target panel takes to slide shut or open, as in style.css
const TARGET_SLIDE_MS = 200;

// setTargetCollapsed slides the target panel shut or open, the pull bar showing the target's icon and name while it is shut
function setTargetCollapsed(collapsed) {
  targetCollapsed = collapsed;
  const wrap = el("target-wrap");
  // clipped while it slides, so the panel does not spill over the report
  wrap.classList.add("sliding");
  wrap.classList.toggle("collapsed", collapsed);
  clearTimeout(setTargetCollapsed.timer);
  setTargetCollapsed.timer = setTimeout(() => wrap.classList.remove("sliding"), TARGET_SLIDE_MS + 50);
  describeTargetPull();

  const label = el("target-pull-label");
  label.replaceChildren();
  if (collapsed && shownTarget) {
    if (shownTarget.imageUrl) {
      const icon = document.createElement("img");
      icon.src = shownTarget.imageUrl;
      icon.alt = "";
      label.append(icon);
    }
    label.append(shownTarget.name);
  }
  label.hidden = !collapsed || !shownTarget;
}

// setHoldingsTucked puts the target's holdings away, or shows them again. Without holdings saved there is nothing to put away, so the Add holding button stays.
function setHoldingsTucked(tucked) {
  holdingsTucked = tucked;
  el("target").classList.toggle("holdings-tucked", tucked && targetHasHoldings);
  describeTargetPull();
}

// holdingsShown is true while the target's holdings are saved and not put away, so the pull bar's next step shut tucks them
function holdingsShown() {
  return targetHasHoldings && !holdingsTucked;
}

// describeTargetPull names what the pull bar's next click or drag does
function describeTargetPull() {
  const bar = el("target-pull");
  bar.setAttribute("aria-expanded", String(!targetCollapsed));
  const action = targetCollapsed ? "show the target panel" : holdingsShown() ? "hide the holdings" : "hide the target panel";
  bar.title = `Click to ${action}`;
  bar.setAttribute("aria-label", action[0].toUpperCase() + action.slice(1));
}

/*
  stepTargetPanel moves the target panel one step shut, or all the way open from shut: with holdings shown, the first step puts them away,
  the next shuts the panel. Opening it shows the holdings again too.
*/
function stepTargetPanel() {
  if (targetCollapsed) {
    setHoldingsTucked(false);
    setTargetCollapsed(false);
  } else if (holdingsShown()) {
    setHoldingsTucked(true);
  } else {
    setTargetCollapsed(true);
  }
}

/*
  wireTargetPull lets the pull bar be clicked, or dragged up or down past PULL_DISTANCE, to step the target panel shut or open it (stepTargetPanel).
  A drag is not also taken as a click when it ends.
*/
function wireTargetPull() {
  const bar = el("target-pull");
  let pull = null; // the drag under way: where the pointer last turned the panel, and whether it moved
  bar.addEventListener("pointerdown", (event) => {
    if (event.button !== 0) return;
    pull = { y: event.clientY, moved: false };
    bar.setPointerCapture(event.pointerId);
  });
  bar.addEventListener("pointermove", (event) => {
    if (!pull) return;
    const moved = event.clientY - pull.y;
    if (Math.abs(moved) > 4) pull.moved = true;
    if ((moved < -PULL_DISTANCE && !targetCollapsed) || (moved > PULL_DISTANCE && targetCollapsed)) {
      stepTargetPanel();
      pull.y = event.clientY;
    }
  });
  bar.addEventListener("pointercancel", () => { pull = null; });
  bar.addEventListener("click", () => {
    const dragged = pull && pull.moved;
    pull = null;
    if (!dragged) stepTargetPanel();
  });
}

// shownTarget is the target in the target panel, as { id, type, name }, for its holding links
let shownTarget = null;

/*
  renderHoldings fills the target panel's holding links strip, for a corp or alliance target: the holdings it keeps, the mains it holds for,
  and a button to add one. A pilot or system target has none, holdings being null, and the strip is hidden.
*/
function renderHoldings(holdings) {
  const strip = el("target-holdings");
  const count = holdings ? holdings.holdings.length + holdings.heldFor.length : 0;
  // put away, a holding just added to the same target would not show, so they come out again. With none, nothing stays put away.
  const added = holdingsShownFor === shownTarget && count > holdingsCount;
  holdingsShownFor = shownTarget;
  holdingsCount = count;
  targetHasHoldings = count > 0;
  setHoldingsTucked(holdingsTucked && targetHasHoldings && !added);
  if (!holdings) {
    strip.replaceChildren();
    strip.hidden = true;
    return;
  }

  const rows = [];
  const row = (label, items) => {
    const line = document.createElement("div");
    line.className = "holding-row";
    const title = document.createElement("span");
    title.className = "holding-label";
    title.textContent = label;
    line.append(title, ...items);
    return line;
  };

  const add = document.createElement("button");
  add.type = "button";
  add.className = "secondary holding-add";
  add.textContent = "+ Add holding";
  add.title = "Tie a holding corporation or alliance to this target";
  add.addEventListener("click", () => openHoldingDialog());
  rows.push(row("Holding", [...holdings.holdings.map((holding) => holdingChip(holding, true)), add]));
  if (holdings.heldFor.length > 0) {
    rows.push(row("Holding for", holdings.heldFor.map((main) => holdingChip(main, false))));
  }

  strip.replaceChildren(...rows);
  strip.hidden = false;
}

/*
  holdingChip is one end of a holding link: its logo, name (click to search it), and note. Its site links are in its right-click menu.
  isHolding is true for the target's own holdings, which can have their note edited. Either end can be removed.
*/
function holdingChip(entity, isHolding) {
  const chip = document.createElement("span");
  chip.className = "holding";
  if (entity.note) chip.title = entity.note;

  const logo = document.createElement("img");
  logo.src = entity.imageUrl;
  logo.alt = "";
  const name = document.createElement("button");
  name.type = "button";
  name.className = "link-button";
  name.textContent = entity.name;
  name.title = "Search for this";
  name.addEventListener("click", () => offerSearch({ type: entity.type, name: entity.name, id: entity.id }));
  chip.append(logo, name);

  if (entity.note) {
    const note = document.createElement("span");
    note.className = "holding-note";
    note.textContent = entity.note;
    chip.append(note);
  }

  if (isHolding) {
    const edit = document.createElement("button");
    edit.type = "button";
    edit.className = "holding-action";
    edit.textContent = "✎";
    edit.title = "Edit the note";
    edit.setAttribute("aria-label", `Edit the note on ${entity.name}`);
    edit.addEventListener("click", () => openHoldingDialog(entity));
    chip.append(edit);
  }

  const remove = document.createElement("button");
  remove.type = "button";
  remove.className = "holding-action";
  remove.textContent = "×";
  remove.title = isHolding ? "Remove this holding" : "Remove this link";
  remove.setAttribute("aria-label", `Remove the holding link with ${entity.name}`);
  remove.addEventListener("click", () => removeHolding(entity, isHolding));
  chip.append(remove);
  return withMenu(chip, [entitySection(entity)]);
}

// openHoldingDialog opens the add holding popup for the shown target, or with editing set, the popup for that holding's note
function openHoldingDialog(editing = null, prefill = null) {
  if (!shownTarget) return;
  el("holding-title").textContent = editing ? "Edit holding note" : "Add holding";
  el("holding-main").textContent = `For ${searchTypeLabel(shownTarget.type)}: ${shownTarget.name}`;
  // an ally added from its menu comes with its type and name, leaving the note to write
  const given = editing || prefill;
  el("holding-type").value = given ? given.type : "ALLIANCE";
  el("holding-name").value = given ? given.name : "";
  el("holding-note").value = editing ? editing.note : "";
  el("holding-type").disabled = Boolean(editing);
  el("holding-name").disabled = Boolean(editing);
  el("holding-error").hidden = true;
  el("holding-save").disabled = false;
  el("holding-dialog").showModal();
  (given ? el("holding-note") : el("holding-name")).focus();
}

// saveHolding saves the popup's holding for the shown target, keeping the popup open with the reason if it can not
async function saveHolding() {
  const target = shownTarget;
  if (!target) return;
  el("holding-save").disabled = true;
  try {
    const holdings = await backend().SaveHolding(target.id, target.type, target.name, el("holding-name").value, el("holding-type").value, el("holding-note").value);
    el("holding-dialog").close();
    if (shownTarget === target) {
      renderHoldings(holdings);
      holdingsChanged();
    }
    tickerPush(`Saved a holding for ${target.name}`);
  } catch (err) {
    el("holding-error").textContent = String(err);
    el("holding-error").hidden = false;
    el("holding-save").disabled = false;
  }
}

// removeHolding removes a link between the shown target and a holding of it, or a main it holds for
async function removeHolding(entity, isHolding) {
  const target = shownTarget;
  if (!target) return;
  const [main, holding] = isHolding ? [target, entity] : [entity, target];
  // RemoveHolding resolves to null when it works, as it returns only an error, so success is made true to tell it from a failure
  const removed = await runBackend(() => backend().RemoveHolding(main.type, main.id, holding.type, holding.id).then(() => true));
  if (removed === null || shownTarget !== target) return;
  const holdings = await runBackend(() => backend().GetHoldings(target.id, target.type));
  if (holdings && shownTarget === target) {
    renderHoldings(holdings);
    holdingsChanged();
  }
  tickerPush(`Removed the holding link between ${main.name} and ${holding.name}`);
}

// holdingsChanged ranks the shown report's allies again, since holdings are left out of them like the target's own alliance
function holdingsChanged() {
  if (intelTarget && shownTarget && intelTarget.id === shownTarget.id && intelTarget.type === shownTarget.type) rankAlliesAgain();
}

// HOLDING_TYPES are the entity types on either end of a holding link, and the kinds of ally
const HOLDING_TYPES = ["ALLIANCE", "CORPORATION"];

// myIntel are My Intel's cards as the backend's IntelGroup, myIntelOpen the keys of the cards opened, and myIntelAdded the cards added here that have nothing saved yet
let myIntel = [];
const myIntelOpen = new Set();
let myIntelAdded = [];
// myIntelPicked are the keys of the cards ticked, for Export and Delete
const myIntelPicked = new Set();

const intelKey = (entity) => `${entity.type}:${entity.id}`;

function openMyIntel() {
  closeThemesMenu();
  closeLimitsMenu();
  showMyIntelError("");
  // an undo lasts only while the window it was offered in is open
  myIntelRemoved = [];
  el("myintel-undo").hidden = true;
  el("myintel").showModal();
  loadMyIntel();
}

async function loadMyIntel() {
  const groups = await runBackend(() => backend().GetMyIntel());
  if (!groups) return;
  myIntel = groups;
  // a card added here stays until it has something saved, when it is one of the backend's
  myIntelAdded = myIntelAdded.filter((added) => !groups.some((group) => intelKey(group.entity) === intelKey(added.entity)));
  renderMyIntel();
}

function showMyIntelError(message) {
  el("myintel-error").textContent = message;
  el("myintel-error").hidden = !message;
}

// renderMyIntel draws the cards that match the filter, by any name on them, those added here first
function renderMyIntel() {
  const filter = el("myintel-filter").value.trim().toLowerCase();
  const groups = [...myIntelAdded, ...myIntel];
  const matches = (group) => !filter || [group.entity, ...group.holdings, ...group.heldFor, ...group.ignoredAllies].some((entity) => entity.name.toLowerCase().includes(filter));
  const shown = groups.filter(matches);
  myIntelShown = shown;
  el("myintel-list").replaceChildren(...shown.map(myIntelCard));
  el("myintel-empty").hidden = groups.length > 0;
  el("myintel-bulk").hidden = groups.length === 0;
  showPickedCount();
}

// myIntelShown are the cards the filter shows, which Check all ticks
let myIntelShown = [];

// pickedCards are the ticked cards the filter shows, those added here first. A ticked card the filter hides is left out, so Export and Delete never act on cards out of sight.
function pickedCards() {
  return myIntelShown.filter((group) => myIntelPicked.has(intelKey(group.entity)));
}

// showPickedCount names how many cards Export and Delete act on, and whether Check all ticks or unticks the cards shown
function showPickedCount() {
  const picked = pickedCards();
  const savedPicked = picked.filter((group) => myIntel.includes(group)).length;
  el("myintel-export-button").textContent = savedPicked ? `Export ${savedPicked}…` : "Export…";
  el("myintel-export-button").disabled = savedPicked === 0;
  el("myintel-delete-button").textContent = picked.length ? `Delete ${picked.length}…` : "Delete…";
  el("myintel-delete-button").disabled = picked.length === 0;
  const allShownPicked = myIntelShown.length > 0 && myIntelShown.every((group) => myIntelPicked.has(intelKey(group.entity)));
  el("myintel-pick-all").textContent = allShownPicked ? "Uncheck all" : "Check all";
}

// pickAllShown ticks every card the filter shows, or unticks them if they all are
function pickAllShown() {
  const keys = myIntelShown.map((group) => intelKey(group.entity));
  const allPicked = keys.length > 0 && keys.every((key) => myIntelPicked.has(key));
  for (const key of keys) {
    if (allPicked) myIntelPicked.delete(key);
    else myIntelPicked.add(key);
  }
  renderMyIntel();
}

// exportMyIntel saves the ticked cards to a file the user picks
async function exportMyIntel() {
  showMyIntelError("");
  const cards = pickedCards().filter((group) => myIntel.includes(group)).map((group) => intelKey(group.entity));
  try {
    const file = await backend().ExportMyIntel(cards);
    if (file) tickerPush(`Exported ${cards.length} My Intel ${cards.length === 1 ? "card" : "cards"} to ${file}`);
  } catch (err) {
    showMyIntelError(String(err));
  }
}

// importMyIntel asks for a file, shows what it would add, and adds it once the user agrees
async function importMyIntel() {
  showMyIntelError("");
  let preview;
  try {
    preview = await backend().PreviewMyIntelImport();
  } catch (err) {
    showMyIntelError(String(err));
    return;
  }
  if (preview.cancelled) return;

  const changes = preview.newHoldings + preview.newIgnored + preview.noteChanges.length;
  el("myintel-import-title").textContent = changes ? `Import ${preview.file}?` : `Nothing new in ${preview.file}`;
  const lines = [];
  if (preview.newHoldings) lines.push(`${plural(preview.newHoldings, "new holding", "new holdings")}`);
  if (preview.newIgnored) lines.push(`${plural(preview.newIgnored, "new ignored ally", "new ignored allies")}`);
  if (preview.knownHoldings || preview.knownIgnored) lines.push(`${preview.knownHoldings + preview.knownIgnored} already saved`);
  el("myintel-import-summary").replaceChildren(...lines.map((line) => {
    const item = document.createElement("li");
    item.textContent = line;
    return item;
  }));

  el("myintel-import-notes").hidden = preview.noteChanges.length === 0;
  el("myintel-import-theirs").checked = false;
  el("myintel-import-note-list").replaceChildren(...preview.noteChanges.map((change) => {
    const item = document.createElement("li");
    item.textContent = `${change.holding} of ${change.main}: yours “${change.mine}”, theirs “${change.theirs}”`;
    return item;
  }));

  const skipped = el("myintel-import-skipped");
  skipped.hidden = preview.skipped === 0;
  skipped.textContent = preview.skipped ? `${plural(preview.skipped, "entry was", "entries were")} left out, as they were not complete: ${preview.skippedExamples.join("; ")}${preview.skipped > preview.skippedExamples.length ? "; …" : ""}` : "";
  el("myintel-import-apply").hidden = changes === 0;

  const dialog = el("myintel-import");
  dialog.returnValue = "";
  dialog.showModal();
  await new Promise((resolve) => dialog.addEventListener("close", resolve, { once: true }));
  if (dialog.returnValue !== "import") return;

  try {
    await backend().ApplyMyIntelImport(el("myintel-import-theirs").checked);
  } catch (err) {
    showMyIntelError(String(err));
    return;
  }
  tickerPush(`Imported ${preview.file} into My Intel`);
  await loadMyIntel();
  syncShownTarget();
}

// myIntelCard is a target's card: its holdings with their notes, the mains it holds for, and its ignored allies, each with an x, and rows to add more
function myIntelCard(group) {
  const entity = group.entity;
  const key = intelKey(entity);
  const card = document.createElement("details");
  card.className = "myintel-card";
  card.open = myIntelOpen.has(key);
  card.addEventListener("toggle", () => (card.open ? myIntelOpen.add(key) : myIntelOpen.delete(key)));

  const summary = document.createElement("summary");
  // ticked to be exported
  const pick = document.createElement("input");
  pick.type = "checkbox";
  pick.className = "myintel-card-pick";
  pick.checked = myIntelPicked.has(key);
  pick.title = "Tick to export or delete";
  pick.setAttribute("aria-label", `Pick ${entity.name}`);
  pick.addEventListener("change", () => {
    if (pick.checked) myIntelPicked.add(key);
    else myIntelPicked.delete(key);
    showPickedCount();
  });
  const icon = document.createElement("img");
  icon.src = entity.iconUrl;
  icon.alt = "";
  const name = document.createElement("span");
  name.className = "myintel-card-name";
  name.textContent = entity.name;
  const kind = document.createElement("span");
  kind.className = "myintel-card-kind";
  kind.textContent = searchTypeLabel(entity.type);
  const counts = document.createElement("span");
  counts.className = "myintel-card-counts";
  const parts = [];
  if (group.holdings.length) parts.push(`${group.holdings.length} ${group.holdings.length === 1 ? "holding" : "holdings"}`);
  if (group.heldFor.length) parts.push(`holds for ${group.heldFor.length}`);
  if (group.ignoredAllies.length) parts.push(`${group.ignoredAllies.length} ignored`);
  counts.textContent = parts.join(" · ");
  const remove = document.createElement("button");
  remove.type = "button";
  remove.className = "allies-ignored-remove myintel-card-remove";
  remove.textContent = "×";
  remove.title = `Remove ${entity.name} from My Intel`;
  remove.setAttribute("aria-label", `Remove ${entity.name} from My Intel`);
  remove.addEventListener("click", (event) => {
    event.preventDefault(); // not a toggle of the card
    removeMyIntelCards([group]);
  });
  summary.append(pick, icon, name);
  if (entity.ticker) summary.append(tickerTag(entity.type, entity.ticker));
  summary.append(kind, counts, remove);
  withMenu(summary, [entitySection(entity)]);

  const body = document.createElement("div");
  body.className = "myintel-card-body";
  const error = document.createElement("p");
  error.className = "preferences-error myintel-card-error";
  error.hidden = true;
  const showError = (message) => {
    error.textContent = message;
    error.hidden = !message;
  };

  if (HOLDING_TYPES.includes(entity.type)) {
    const add = myIntelAddRow(`${key}:holdings`, "Holding name", true, (name, type, note) =>
      myIntelChange(() => backend().SaveHolding(entity.id, entity.type, entity.name, name, type, note), showError, `Saved a holding for ${entity.name}`));
    body.append(myIntelTitle("Holdings", add, "Add a holding"));
    if (group.holdings.length === 0) body.append(myIntelNone("No holdings"));
    body.append(...group.holdings.map((holding) => myIntelRow(holding, {
      note: holding.note,
      // by its link, so by id, as the ESI can not find every name
      onNote: (note) => myIntelChange(() => backend().SaveHolding(entity.id, entity.type, entity.name, holding.zkillUrl, holding.type, note), showError, `Saved the note on ${holding.name}`),
      onRemove: () => myIntelChange(() => backend().RemoveHolding(entity.type, entity.id, holding.type, holding.id), showError, `Removed ${holding.name} as a holding of ${entity.name}`),
    })));
    body.append(add);
  }

  if (group.heldFor.length > 0) {
    body.append(myIntelTitle("Holds for"));
    body.append(...group.heldFor.map((main) => myIntelRow(main, {
      noteText: main.note,
      onRemove: () => myIntelChange(() => backend().RemoveHolding(main.type, main.id, entity.type, entity.id), showError, `Removed ${entity.name} as a holding of ${main.name}`),
    })));
  }

  const addAlly = myIntelAddRow(`${key}:allies`, "Ally name", false, (name, type) =>
    myIntelChange(() => backend().AddIgnoredAlly(entity.type, entity.id, name, type), showError, `Ignoring ${name} for ${entity.name}`));
  body.append(myIntelTitle("Ignored allies", addAlly, "Ignore an ally"));
  if (group.ignoredAllies.length === 0) body.append(myIntelNone("None ignored"));
  body.append(...group.ignoredAllies.map((ally) => myIntelRow(ally, {
    onRemove: () => myIntelChange(() => backend().RemoveIgnoredAlly(entity.type, entity.id, ally.type, ally.id), showError, `No longer ignoring ${ally.name} for ${entity.name}`),
  })));
  body.append(addAlly);

  body.append(error);
  card.append(summary, body);
  return card;
}

// myIntelRemoved are the cards last removed, kept to undo, empty when there is nothing to undo
let myIntelRemoved = [];

const plural = (count, one, many) => `${count.toLocaleString("en-US")} ${count === 1 ? one : many}`;

/*
  removeMyIntelCards asks, then removes cards' holdings and ignored allies at once, offering to undo it. Links where a card is another main's holding belong to
  that main's card, so they stay. Cards added here with nothing saved just go.
*/
async function removeMyIntelCards(groups) {
  const addedKeys = new Set(myIntelAdded.map((added) => intelKey(added.entity)));
  const saved = groups.filter((group) => !addedKeys.has(intelKey(group.entity)));
  const dropAdded = () => {
    const removing = new Set(groups.map((group) => intelKey(group.entity)));
    myIntelAdded = myIntelAdded.filter((added) => !removing.has(intelKey(added.entity)));
    for (const key of removing) {
      myIntelOpen.delete(key);
      myIntelPicked.delete(key);
    }
  };
  if (saved.length === 0) {
    dropAdded();
    renderMyIntel();
    return;
  }

  if (!(await confirmMyIntelRemove(saved))) return;
  dropAdded();

  showMyIntelError("");
  try {
    await backend().RemoveMyIntelGroups(saved.map((group) => intelKey(group.entity)));
  } catch (err) {
    showMyIntelError(String(err));
    return;
  }
  myIntelRemoved = saved;
  const what = saved.length === 1 ? saved[0].entity.name : plural(saved.length, "card", "cards");
  el("myintel-undo-text").textContent = `Removed ${what}`;
  el("myintel-undo").hidden = false;
  tickerPush(`Removed ${what} from My Intel`);
  await loadMyIntel();
  syncShownTarget();
}

// confirmMyIntelRemove asks before cards are removed, saying what goes with them and what stays, resolving true to remove them
async function confirmMyIntelRemove(groups) {
  const holdings = groups.reduce((total, group) => total + group.holdings.length, 0);
  const allies = groups.reduce((total, group) => total + group.ignoredAllies.length, 0);
  const owned = [];
  if (holdings) owned.push(plural(holdings, "holding", "holdings"));
  if (allies) owned.push(plural(allies, "ignored ally", "ignored allies"));
  const one = groups.length === 1;
  el("myintel-remove-title").textContent = one ? `Remove ${groups[0].entity.name} from My Intel?` : `Remove ${plural(groups.length, "card", "cards")} from My Intel?`;
  let detail = `${owned.length ? `${one ? "Its" : "Their"} ${owned.join(" and ")}` : "Everything saved on them"} will be removed.`;
  const heldFor = groups.filter((group) => group.heldFor.length);
  if (one && heldFor.length) {
    const mains = groups[0].heldFor.map((main) => main.name);
    detail += ` It stays a holding of ${mains.join(", ")}, as those links belong to ${mains.length === 1 ? "that main" : "those mains"}.`;
  } else if (heldFor.length) {
    detail += ` Links where one is another main's holding stay, as they belong to that main.`;
  }
  el("myintel-remove-detail").textContent = detail;

  const dialog = el("myintel-remove-confirm");
  dialog.returnValue = "";
  dialog.showModal();
  await new Promise((resolve) => dialog.addEventListener("close", resolve, { once: true }));
  return dialog.returnValue === "remove";
}

// undoMyIntelRemove saves the removed cards' holdings, with their notes, and ignored allies again
async function undoMyIntelRemove() {
  const groups = myIntelRemoved;
  if (groups.length === 0) return;
  myIntelRemoved = [];
  el("myintel-undo").hidden = true;
  const failed = [];
  for (const group of groups) {
    const entity = group.entity;
    try {
      // by their links, so by id, as the ESI can not find every name
      for (const holding of group.holdings) await backend().SaveHolding(entity.id, entity.type, entity.name, holding.zkillUrl, holding.type, holding.note);
      for (const ally of group.ignoredAllies) await backend().AddIgnoredAlly(entity.type, entity.id, ally.zkillUrl, ally.type);
    } catch (err) {
      failed.push(`${entity.name}: ${err}`);
    }
  }
  if (failed.length) showMyIntelError(`Could not bring all of them back. ${failed.join("; ")}`);
  tickerPush(`Brought ${groups.length === 1 ? groups[0].entity.name : plural(groups.length, "card", "cards")} back to My Intel`);
  await loadMyIntel();
  syncShownTarget();
}

// myIntelAdding are the add rows opened with their section's +, by card and section, ie ALLIANCE:99013330:holdings, kept open while the window redraws
const myIntelAdding = new Set();

// myIntelTitle is a card section's title, with a + at its right that shows or hides the section's add row, when it has one
function myIntelTitle(text, addRow = null, addLabel = "") {
  const line = document.createElement("div");
  line.className = "myintel-section-line";
  const title = document.createElement("p");
  title.className = "myintel-section-title";
  title.textContent = text;
  line.append(title);
  if (addRow) {
    const plus = document.createElement("button");
    plus.type = "button";
    plus.className = "myintel-plus";
    plus.textContent = "+";
    plus.title = addLabel;
    plus.setAttribute("aria-label", addLabel);
    plus.setAttribute("aria-expanded", String(!addRow.hidden));
    plus.addEventListener("click", () => {
      addRow.hidden = !addRow.hidden;
      plus.setAttribute("aria-expanded", String(!addRow.hidden));
      if (addRow.hidden) myIntelAdding.delete(addRow.dataset.key);
      else {
        myIntelAdding.add(addRow.dataset.key);
        addRow.querySelector("input").focus();
      }
    });
    line.append(plus);
  }
  return line;
}

function myIntelNone(text) {
  const none = document.createElement("p");
  none.className = "myintel-none";
  none.textContent = text;
  return none;
}

/*
  myIntelRow is one entity on a card: an x, its logo, its name, which offers a search for it, and its ticker, then at the right a note.
  options: { note, onNote } for a note shown with a pencil to edit it, { noteText } for one only shown, and onRemove for the x.
*/
function myIntelRow(entity, options) {
  const row = document.createElement("div");
  row.className = "myintel-row";
  const icon = document.createElement("img");
  icon.src = entity.iconUrl;
  icon.alt = "";
  const who = document.createElement("span");
  who.className = "myintel-who";
  const name = document.createElement("button");
  name.type = "button";
  name.className = "link-button";
  name.textContent = entity.name;
  name.title = `Search for ${entity.name}`;
  name.addEventListener("click", () => {
    el("myintel").close();
    offerSearch({ type: entity.type, name: entity.name, id: entity.id });
  });
  who.append(name);
  if (entity.ticker) who.append(tickerTag(entity.type, entity.ticker));

  let note;
  if (options.onNote) {
    note = myIntelNote(entity, options.note, options.onNote);
  } else {
    note = document.createElement("span");
    note.className = "myintel-note-text";
    note.textContent = options.noteText || "";
  }

  const remove = document.createElement("button");
  remove.type = "button";
  remove.className = "allies-ignored-remove";
  remove.textContent = "×";
  remove.title = "Remove";
  remove.setAttribute("aria-label", `Remove ${entity.name}`);
  remove.addEventListener("click", options.onRemove);

  row.append(remove, icon, who, note);
  return withMenu(row, [entitySection(entity)]);
}

// myIntelNote is a holding's note, None in italics if it has none, with a pencil that turns it into a box: Enter or leaving it saves, Escape keeps the note as it was
function myIntelNote(entity, text, save) {
  const cell = document.createElement("span");
  cell.className = "myintel-note-cell";
  const shown = document.createElement("span");
  shown.className = text ? "myintel-note-text" : "myintel-note-text myintel-note-none";
  shown.textContent = text || "None";
  const pencil = document.createElement("button");
  pencil.type = "button";
  pencil.className = "myintel-pencil";
  pencil.textContent = "✎";
  pencil.title = "Edit the note";
  pencil.setAttribute("aria-label", `Edit the note on ${entity.name}`);
  pencil.addEventListener("click", () => {
    const box = document.createElement("input");
    box.type = "text";
    box.className = "myintel-note";
    box.maxLength = 200; // maxHoldingNote, which the backend checks again
    box.value = text || "";
    box.setAttribute("aria-label", `Note on ${entity.name}`);
    let done = false;
    const finish = (keep) => {
      if (done) return;
      done = true;
      if (keep && box.value.trim() !== (text || "")) save(box.value.trim());
      else cell.replaceChildren(shown, pencil);
    };
    box.addEventListener("keydown", (event) => {
      if (event.key === "Enter") {
        event.preventDefault();
        finish(true);
      } else if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation(); // not a close of the window
        finish(false);
      }
    });
    box.addEventListener("blur", () => finish(true));
    cell.replaceChildren(box);
    box.focus();
    box.select();
  });
  cell.append(shown, pencil);
  return cell;
}

// myIntelAddRow is a card section's row to add a holding, withNote, or an ignored ally: a type, a name, and Add, which calls add(name, type, note). Hidden until its +.
function myIntelAddRow(key, placeholder, withNote, add) {
  const form = document.createElement("form");
  form.className = withNote ? "myintel-row-add" : "myintel-row-add no-note";
  form.dataset.key = key;
  form.hidden = !myIntelAdding.has(key);
  const type = document.createElement("select");
  type.setAttribute("aria-label", "Type");
  for (const value of HOLDING_TYPES) type.append(new Option(searchTypeLabel(value), value));
  const name = document.createElement("input");
  name.type = "text";
  name.placeholder = placeholder;
  name.autocomplete = "off";
  name.spellcheck = false;
  name.setAttribute("aria-label", placeholder);
  form.append(type, name);
  let note = null;
  if (withNote) {
    note = document.createElement("input");
    note.type = "text";
    note.maxLength = 200;
    note.placeholder = "Note (optional)";
    note.setAttribute("aria-label", "Note");
    form.append(note);
  }
  const submit = document.createElement("button");
  submit.type = "submit";
  submit.className = "secondary";
  submit.textContent = "Add";
  form.append(submit);
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    if (!name.value.trim()) return;
    // closed before the redraw that follows a save, and open again, with the name to try again, if it could not be saved
    myIntelAdding.delete(key);
    add(name.value.trim(), type.value, note ? note.value : "").then((added) => {
      if (!added) myIntelAdding.add(key);
    });
  });
  return form;
}

// myIntelChange saves a change from a card, showing why on the card if it can not, then draws My Intel again and brings the shown target up to date. It resolves whether it saved.
async function myIntelChange(save, showError, message) {
  showError("");
  try {
    await save();
  } catch (err) {
    showError(String(err));
    return false;
  }
  tickerPush(message);
  await loadMyIntel();
  syncShownTarget();
  return true;
}

// syncShownTarget shows the target panel's holdings and the Allies tab again after My Intel changed them
async function syncShownTarget() {
  const target = shownTarget;
  if (target && HOLDING_TYPES.includes(target.type)) {
    const holdings = await runBackend(() => backend().GetHoldings(target.id, target.type));
    if (holdings && shownTarget === target) renderHoldings(holdings);
  }
  rankAlliesAgain();
}

// addMyIntelCard looks up a pilot, corp, or alliance by name and opens a card for it, at the top, until something is saved on it
async function addMyIntelCard(event) {
  event.preventDefault();
  showMyIntelError("");
  const name = el("myintel-add-name").value.trim();
  if (!name) return;
  let entity;
  try {
    entity = await backend().LookupEntity(name, el("myintel-add-type").value);
  } catch (err) {
    showMyIntelError(String(err));
    return;
  }
  const key = intelKey(entity);
  if (![...myIntel, ...myIntelAdded].some((group) => intelKey(group.entity) === key)) {
    myIntelAdded.unshift({ entity, holdings: [], heldFor: [], ignoredAllies: [] });
  }
  myIntelOpen.add(key);
  el("myintel-add-name").value = "";
  el("myintel-filter").value = "";
  renderMyIntel();
}

const MEMBERS_PER_PAGE = 50;
// MEMBER_PAGES_PER_LOAD pages of members are looked up at a time, the next ones only once the last of them is shown
const MEMBER_PAGES_PER_LOAD = 3;

/*
  roster is the Members tab of a corp or alliance search: { id, type, rows, total, end, page, loading, error }.
  rows are filled a block of MEMBER_PAGES_PER_LOAD pages at a time from GetMembers, so they can have gaps: jumping to the last page loads only its block.
  total is the member count the backend expects, end the real count once a block reaches it, null before. loading is the set of blocks being looked up.
  A corp's rows are its pilots: those seen on the killmails, most recent first, then the rest newest to the corp first. An alliance's are its corporations, newest to the alliance first.
*/
let roster = null;

// ROSTER_COLUMNS are the Members tab's headings and their tooltips: a corp lists its pilots, an alliance its corporations
const ROSTER_COLUMNS = {
  CORPORATION: [["", ""], ["Member", ""], ["Born", ""], ["Time in corp", "How long since they joined the corp"], ["NPC corp time", "Their longest stay in an NPC corporation"], ["Last killmail", "Their most recent killmail among the collected killmails"]],
  ALLIANCE: [["", ""], ["Corporation", ""], ["Members", ""], ["Time in alliance", "How long since the corporation joined the alliance"]],
};

// startRoster begins a corp or alliance target's Members tab, looking up its first pages in the background
function startRoster(id, type) {
  roster = { id, type, rows: [], total: 0, end: null, page: 0, loading: new Set(), error: "" };
  const heading = document.createElement("tr");
  heading.append(...ROSTER_COLUMNS[type].map(([text, title]) => {
    const cell = document.createElement("th");
    cell.textContent = text;
    if (title) cell.title = title;
    return cell;
  }));
  el("roster-head").replaceChildren(heading);
  el("tab-roster").hidden = false;
  renderRosterPage();
}

const MEMBER_BLOCK_SIZE = MEMBERS_PER_PAGE * MEMBER_PAGES_PER_LOAD;

// blockLoaded reports whether a block of members has been looked up, it is then filled from its start
function blockLoaded(block) {
  return roster.rows[block * MEMBER_BLOCK_SIZE] !== undefined || (roster.end !== null && block * MEMBER_BLOCK_SIZE >= roster.end);
}

// loadMemberBlock looks up a block of MEMBER_PAGES_PER_LOAD pages of members, unless it is loaded, loading, or past the end
async function loadMemberBlock(block) {
  const current = roster;
  const offset = block * MEMBER_BLOCK_SIZE;
  if (!current || current.error || current.loading.has(block) || blockLoaded(block)) return;

  current.loading.add(block);
  if (roster === current) renderRosterPage();
  try {
    const page = await backend().GetMembers(current.id, current.type, offset, MEMBER_BLOCK_SIZE);
    const rows = current.type === "ALLIANCE" ? page.corporations : page.rows;
    rows.forEach((row, index) => { current.rows[offset + index] = row; });
    current.total = page.total;
    // past the end the backend answers with fewer rows, or none, and no more after them
    if (!page.hasMore) current.end = Math.min(current.end ?? Infinity, offset + rows.length);
  } catch (err) {
    current.error = String(err);
  } finally {
    current.loading.delete(block);
  }
  if (roster === current) renderRosterPage(); // a new search may have replaced it while this loaded
}

// rosterPageCount is how many pages of members there are, or are expected to be until the end is reached
function rosterPageCount() {
  const known = roster.end ?? Math.max(roster.total, roster.rows.length);
  return Math.max(1, Math.ceil(known / MEMBERS_PER_PAGE));
}

// daysAgo says how long ago a time was in whole days, ie today, 1 day ago, or 12 days ago
function daysAgo(isoTime) {
  const days = Math.floor((Date.now() - new Date(isoTime).getTime()) / 86400000);
  if (days <= 0) return "today";
  return days === 1 ? "1 day ago" : `${days.toLocaleString("en-US")} days ago`;
}

function textCell(text) {
  const cell = document.createElement("td");
  cell.textContent = text;
  return cell;
}

function renderMemberRow(member) {
  const row = document.createElement("tr");

  const nameCell = document.createElement("td");
  const name = document.createElement("button");
  name.type = "button";
  name.className = "link-button";
  name.textContent = member.name;
  name.title = "Search for this";
  name.addEventListener("click", () => offerSearch({ type: "PILOT", name: member.name, id: member.id }));
  nameCell.append(name);

  const lastSeen = member.lastSeen ? `${spokenDate(member.lastSeen)} (${daysAgo(member.lastSeen)})` : "Not seen";
  row.append(avatarCell(member.name, member, member.imageUrl, true), nameCell, textCell(member.born), textCell(member.timeInCorp), textCell(member.npcTime), textCell(lastSeen));
  // EveWho still lists them, but the ESI has them out of the corp
  if (member.left) {
    row.className = "member-left";
    row.title = "Has left the corp";
  }
  return withMenu(row, pilotSections(member));
}

// renderMemberCorpRow is one of an alliance's corporations: logo, name to search for, member count, and time in the alliance
function renderMemberCorpRow(corp) {
  const row = document.createElement("tr");

  const nameCell = document.createElement("td");
  const name = document.createElement("button");
  name.type = "button";
  name.className = "link-button";
  name.textContent = corp.name;
  name.title = "Search for this";
  name.addEventListener("click", () => offerSearch({ type: "CORPORATION", name: corp.name, id: corp.id }));
  nameCell.append(name);

  row.append(avatarCell(corp.name, corp, corp.iconUrl, true), nameCell, textCell(corp.memberCount.toLocaleString("en-US")), textCell(corp.timeInAlliance));
  return withMenu(row, [entitySection({ type: "CORPORATION", id: corp.id, name: corp.name, zkillUrl: corp.zkillUrl, evewhoUrl: corp.evewhoUrl })]);
}

// rosterDivider is the line on a corp's Members tab between the members seen on killmails and the rest
function rosterDivider() {
  const row = document.createElement("tr");
  row.className = "roster-divider";
  const cell = document.createElement("td");
  cell.colSpan = ROSTER_COLUMNS.CORPORATION.length;
  cell.textContent = "No killmails · newest to the corp first";
  row.append(cell);
  return row;
}

/*
  renderRosterPage shows the current page of members, looking up its block if it is not loaded yet,
  and starts looking up the next block once the last page of a loaded one is shown
*/
function renderRosterPage() {
  if (!roster) return;

  const pageCount = rosterPageCount();
  roster.page = Math.min(roster.page, pageCount - 1);
  const start = roster.page * MEMBERS_PER_PAGE;
  const block = Math.floor(roster.page / MEMBER_PAGES_PER_LOAD);
  const rows = blockLoaded(block) ? roster.rows.slice(start, start + MEMBERS_PER_PAGE).filter(Boolean) : [];

  if (roster.type === "ALLIANCE") {
    el("roster-body").replaceChildren(...rows.map(renderMemberCorpRow));
  } else {
    // the divider goes before the first member with no killmails, after the last one seen on them
    const body = [];
    rows.forEach((member, index) => {
      const overall = start + index;
      if (!member.lastSeen && (overall === 0 || roster.rows[overall - 1]?.lastSeen)) body.push(rosterDivider());
      body.push(renderMemberRow(member));
    });
    el("roster-body").replaceChildren(...body);
  }

  let status = "";
  if (roster.error) status = roster.error;
  else if (rows.length === 0 && !blockLoaded(block)) status = "Loading members…";
  else if (roster.rows.length === 0) status = "No members found.";
  el("roster-status").textContent = status;
  el("roster-status").hidden = !status;

  el("roster-pager").hidden = pageCount <= 1;
  renderPager(el("roster-label"), roster.page, pageCount, showRosterPage, { first: el("roster-first"), last: el("roster-last") });
  el("roster-prev").disabled = roster.page === 0;
  el("roster-next").disabled = roster.page >= pageCount - 1;

  if (!blockLoaded(block)) loadMemberBlock(block);
  else if ((roster.page + 1) % MEMBER_PAGES_PER_LOAD === 0) loadMemberBlock(block + 1);
}

function changeRosterPage(step) {
  if (roster) showRosterPage(roster.page + step);
}

function showRosterPage(page) {
  if (!roster) return;
  roster.page = page;
  renderRosterPage();
}

// intelReport is the report shown, kept so its dates can be redrawn when the time zone preference changes
let intelReport = null;

// renderIntel fills a tab for each section of the killmail based intel report. Sections the backend left null do not apply to this target type.
function renderIntel(report) {
  intelReport = report;
  killmails = report.killmails || [];
  killmailPage = 0;
  renderIntelSections(report);

  selectTab("tab-glance");
  el("intel-loading").hidden = true;
  el("intel-tabs").hidden = false;
}

// renderIntelSections draws the killmail page and each tab of a report, keeping the page and tab the user is on
function renderIntelSections(report) {
  // every search has these two, though showProfileOnly hides them
  el("tab-killmails").hidden = false;
  el("tab-activity").hidden = false;
  setTabCount("tab-killmails", killmails.length);
  setTabCount("tab-pi", report.piPilots && report.piPilots.length);
  setTabCount("tab-ratters", report.ratters && report.ratters.length);
  // its pilots, each once though they flew more than one hull
  setTabCount("tab-capitals", report.capitals && new Set(report.capitals.flatMap((capital) => capital.pilots.map((pilot) => pilot.id))).size);
  setTabCount("tab-structures", report.structures && report.structures.used.length + report.structures.destroyed.length + report.structures.lost.length);
  setTabCount("tab-fits", report.fits && report.fits.length);
  renderKillmailPage();
  renderActivity(report.killmails || []);

  el("tab-fleetmates").hidden = !report.fleetmates;
  if (report.fleetmates) {
    renderPilotRows("fleetmates", report.fleetmates);
  }

  el("tab-members").hidden = !report.members;
  if (report.members) {
    renderPilotRows("members", report.members);
  }

  el("tab-pilots").hidden = !report.pilots;
  if (report.pilots) {
    renderPilotRows("pilots", report.pilots);
  }

  el("tab-allies").hidden = !report.allies;
  if (report.allies) {
    // a new search hides the allies the target killed again, as the preference says
    showingKilledAllies = false;
    renderAllies(report.allies, report.ignoredAllies || [], report.killedAllies || 0, report.holdings || []);
  }

  el("tab-corps").hidden = !report.corporations;
  if (report.corporations) {
    renderCorporations(report.corporations);
  }

  el("tab-systems").hidden = !report.systems;
  if (report.systems) {
    renderSystems(report.systems);
  }

  el("tab-pi").hidden = !report.piPilots;
  if (report.piPilots) {
    renderPIPilots(report.piPilots);
  }

  el("tab-ratters").hidden = !report.ratters;
  if (report.ratters) {
    renderRatters(report.ratters);
  }

  el("tab-capitals").hidden = !report.capitals;
  if (report.capitals) {
    renderCapitals(report.capitals);
  }

  el("tab-fits").hidden = !report.fits;
  if (report.fits) {
    renderFits(report.fits);
  } else {
    fitsShown = noFitsShown();
  }

  el("tab-structures").hidden = !report.structures;
  if (report.structures) {
    renderStructures(report.structures);
  }
  renderStructureLoss(report.jspace, report.lastStructureLoss, report.previousStructureLosses || []);

  // the Members tab's rows come from GetMembers, started by the search
  el("tab-roster").hidden = !report.memberList;
  if (report.memberList) renderRosterPage();

  renderGlance(report);
}

/*
  showProfileOnly shows the report area with only At a glance, holding the target's Profile and why there is nothing else,
  for a search that ended without a report, so the target's details are still shown somewhere.
*/
function showProfileOnly(reason) {
  for (const tab of el("tab-list").querySelectorAll(".tab")) tab.hidden = true;
  const note = document.createElement("p");
  note.className = "muted";
  note.textContent = reason;
  el("glance-cards").replaceChildren(note);
  selectTab("tab-glance");
  el("intel-loading").hidden = true;
  el("intel-tabs").hidden = false;
  el("intel").hidden = false;
}

// setTabCount shows how many rows a tab has beside its name in the sidebar, or nothing for a count of 0 or none
function setTabCount(tabId, count) {
  const shown = el(tabId).querySelector(".tab-count");
  shown.textContent = count ? shortCount(count) : "";
  // in full for the collapsed sidebar's tooltip
  shown.dataset.count = count ? count.toLocaleString("en-US") : "";
}

// shortCount fits a count in the collapsed sidebar's badge: as it is up to 999, then in thousands, ie 1.2k under 10,000 and 200k above
function shortCount(count) {
  if (count < 1000) return String(count);
  const thousands = count / 1000;
  return `${thousands < 10 ? Math.floor(thousands * 10) / 10 : Math.floor(thousands)}k`;
}

// ACTIVITY_DAYS are the histogram's rows, Monday first, as [name, Date.getDay() value]
const ACTIVITY_DAYS = [["Monday", 1], ["Tuesday", 2], ["Wednesday", 3], ["Thursday", 4], ["Friday", 5], ["Saturday", 6], ["Sunday", 0]];

// activityCounts counts times by the weekday and hour they fall in, as [weekday][hour] with 0 as Sunday, in EVE or local time as chosen in Preferences
function activityCounts(times) {
  const counts = Array.from({ length: 7 }, () => Array(24).fill(0));
  for (const time of times) {
    const parts = timeParts(time);
    counts[parts.weekday][parts.hour]++;
  }
  return counts;
}

// timeRange is the oldest and newest of ISO times, as [oldest, newest], in one pass rather than a sort
function timeRange(times) {
  let oldest = times[0];
  let newest = times[0];
  for (const time of times) {
    if (time < oldest) oldest = time;
    if (time > newest) newest = time;
  }
  return [oldest, newest];
}

/*
  renderActivity draws the Activity tab: a grid of the killmails seen in each hour of each day of the week, in the time zone chosen in Preferences.
  The busier an hour, the stronger its accent color, and each hour shows its count. The last row and column total each hour and day.
*/
function renderActivity(kms) {
  const counts = activityCounts(kms.map((km) => km.time));
  const busiest = Math.max(0, ...counts.flat());
  const zone = preferences.timeZone === "local" ? "local" : "EVE";

  const cell = (className, content = "") => {
    const div = document.createElement("div");
    div.className = className;
    div.textContent = content;
    return div;
  };
  const cells = [cell("activity-corner"), ...Array.from({ length: 24 }, (_, hour) => cell("activity-hour", pad2(hour))), cell("activity-hour activity-total", "Total")];

  for (const [dayName, weekday] of ACTIVITY_DAYS) {
    cells.push(cell("activity-day", dayName.slice(0, 3)));
    counts[weekday].forEach((count, hour) => {
      const block = cell("activity-cell", count || "");
      block.title = `${dayName} ${pad2(hour)}:00–${pad2(hour)}:59 ${zone}: ${count} ${count === 1 ? "killmail" : "killmails"}`;
      if (count) {
        const share = count / busiest;
        // the quietest hour still shows a tint, the busiest is the full accent
        block.style.background = `color-mix(in srgb, var(--accent) ${Math.round(15 + 85 * share)}%, var(--panel-raised))`;
        if (share >= 0.55) block.classList.add("activity-strong");
      }
      cells.push(block);
    });
    cells.push(cell("activity-cell activity-total", counts[weekday].reduce((a, b) => a + b, 0) || ""));
  }

  cells.push(cell("activity-day", "Total"));
  for (let hour = 0; hour < 24; hour++) {
    cells.push(cell("activity-cell activity-total", counts.reduce((sum, day) => sum + day[hour], 0) || ""));
  }
  cells.push(cell("activity-cell activity-total", kms.length || ""));

  el("activity-grid").replaceChildren(...cells);
  el("activity-note").textContent = `${kms.length} ${kms.length === 1 ? "killmail" : "killmails"} by the hour they happened, in ${zone === "EVE" ? "EVE time (UTC)" : "local time"}.`;
  el("activity-grid").hidden = kms.length === 0;
  el("activity-note").hidden = kms.length === 0;

  // the dates the chart covers, from the oldest killmail to the newest
  el("activity-span").textContent = kms.length ? killmailSpan(...timeRange(kms.map((km) => km.time)), true) : "";
  el("activity-span").hidden = kms.length === 0;
  el("activity-empty").hidden = kms.length > 0;
}

/*
  renderCapitals lists each capital ship type a corp or alliance's members flew, with its image, then the pilots who flew it in a collapsible list.
  Each pilot shows their current corp and alliance, and is struck through if they have left the target's alliance. Each list starts open. Clicking a pilot offers a search for them.
*/
function renderCapitals(capitals) {
  el("capitals-list").replaceChildren(...capitals.map((capital) => {
    const ship = document.createElement("details");
    ship.className = "capital";
    ship.open = true;

    const summary = document.createElement("summary");
    const image = document.createElement("img");
    image.src = capital.imageUrl;
    image.alt = "";
    const name = document.createElement("span");
    name.className = "capital-name";
    name.textContent = capital.name;
    const count = document.createElement("span");
    count.className = "capital-count";
    count.textContent = `${capital.pilots.length} ${capital.pilots.length === 1 ? "pilot" : "pilots"}`;
    summary.append(image, name, count);

    const pilots = document.createElement("ul");
    pilots.className = "capital-pilots";
    pilots.append(...capital.pilots.map((pilot) => {
      const item = document.createElement("li");
      const button = document.createElement("button");
      button.type = "button";
      button.className = pilot.left ? "link-button capital-left" : "link-button";
      button.textContent = pilot.name;
      button.title = pilot.left ? "No longer in this alliance. Search for this" : "Search for this";
      button.addEventListener("click", () => offerSearch({ type: "PILOT", name: pilot.name, id: pilot.id }));
      item.append(button);

      const group = groupLine(pilot.corporation, pilot.alliance);
      if (group) item.append(group);
      // only the pilots under each ship have the menu, not the ship
      return withMenu(item, pilotSections(pilot));
    }));

    ship.append(summary, pilots);
    return ship;
  }));
  el("capitals-empty").hidden = capitals.length > 0;
}

// fitsShown is the Fits tab's list as drawn, kept so a redraw (ie the time zone changed) and Show fit keep the chosen fit and the folded hulls
let fitsShown = noFitsShown();

/*
  noFitsShown is fitsShown before any Fits tab is drawn. byKillmail is each loss's fit, by the loss's and its capsule's killmail ids.
  filter lists only the fits the filter box matches, searchText is what it matches each fit against (fitSearchText), and filterOpened is the hulls a filter has
  unfolded, each with whether it was folded before, so clearing the box folds them again; null while the box is empty.
*/
function noFitsShown() {
  return { fits: null, selected: 0, folded: new Set(), select: null, byKillmail: new Map(), filter: null, searchText: new Map(), filterOpened: null, ruleTags: [] };
}

// FITS_FOLD_OVER is how many fits the tab lists before it starts with every hull folded, but the chosen one's
const FITS_FOLD_OVER = 12;

/*
  renderFits fills the Fits tab of a pilot, corp, or alliance search: the target's fits, identical fits merged with a count, under a heading for each ship that folds.
  Selecting one draws it in the fitting window on the right, with its losses to page through. Copy EFT puts the chosen loss on the clipboard for the in-game Import from clipboard.
  The first fit listed is selected to start, and a redraw keeps the one chosen.
*/
function renderFits(fits) {
  if (fitsShown.fits !== fits) {
    fitsShown = { ...noFitsShown(), fits };
    // a new search starts unfiltered
    el("fits-filter").value = "";
    if (fits.length > FITS_FOLD_OVER) fits.forEach((fit) => fitsShown.folded.add(fit.shipName));
    for (const fit of fits) {
      for (const loss of fit.losses) {
        fitsShown.byKillmail.set(loss.killmailId, fit);
        if (loss.podKillmailId) fitsShown.byKillmail.set(loss.podKillmailId, fit);
      }
      fitsShown.searchText.set(fit, fitSearchText(fit));
    }
    fitsShown.ruleTags = fitRuleTags(fits);
    if (fitKeyCollapsedBefore !== null) fillFitKey();
  }
  const rows = new Map();
  const hulls = new Map();
  // a fit picked from the list opens on its first loss, the one with the most implants, the newest of those (buildFits)
  const select = (fit, killmailId) => {
    const loss = fit.losses.find((each) => isFitLoss(each, killmailId)) || fit.losses[0];
    fitsShown.selected = loss.killmailId;
    for (const [shown, row] of rows) row.setAttribute("aria-selected", String(shown === fit));
    if (fitsShown.folded.delete(fit.shipName)) hulls.get(fit.shipName).unfold();
    renderFitWindow(fit, loss, (other) => select(fit, other));
    return rows.get(fit);
  };
  // Show fit: a fit the filter hides is shown by clearing the filter
  fitsShown.select = (killmailId) => {
    const fit = fitsShown.byKillmail.get(killmailId);
    if (!fit) return null;
    if (rows.get(fit).parentElement.hidden) {
      el("fits-filter").value = "";
      fitsShown.filter();
    }
    return select(fit, killmailId);
  };

  const ofHull = new Map();
  for (const fit of fits) ofHull.set(fit.shipName, [...(ofHull.get(fit.shipName) || []), fit]);
  const items = [];
  for (const fit of fits) {
    if (!hulls.has(fit.shipName)) {
      const hull = fitsHull(fit, ofHull.get(fit.shipName));
      hulls.set(fit.shipName, hull);
      items.push(hull.item);
    }
    const row = fitRow(fit);
    row.addEventListener("click", () => select(fit));
    rows.set(fit, row);
    const item = document.createElement("li");
    item.append(row);
    hulls.get(fit.shipName).rows.append(item);
  }
  el("fits-list").replaceChildren(...items);
  fitsShown.filter = () => filterFits(fits, rows, hulls);
  fitsShown.filter();

  el("fits-layout").hidden = fits.length === 0;
  el("fits-empty").hidden = fits.length > 0;
  if (fits.length === 0) {
    // nothing of the last search's fit is left behind
    el("fit-view-title").textContent = "";
    el("fit-view-loss").replaceChildren();
    el("fit-view-loss").oncontextmenu = null;
    el("fit-view-tags").replaceChildren();
    hideFitTagTip();
    el("fit-view-note").hidden = true;
    el("fit-ring").replaceChildren();
    el("fit-prev").hidden = true;
    el("fit-next").hidden = true;
    el("fit-page-count").textContent = "";
    el("panel-fits").onkeydown = null;
    for (const button of [el("fit-copy"), el("fit-zkill")]) {
      button.disabled = true;
      button.onclick = null;
      button.oncontextmenu = null;
    }
  } else {
    // a redraw keeps the chosen fit, even one the filter hides
    const kept = fitsShown.byKillmail.get(fitsShown.selected);
    if (kept) select(kept, fitsShown.selected);
    else select(fits[0]);
  }
}

/*
  filterFits hides the fits the filter box does not match, and the hull headings left with none.
  Each word in the box must be in the fit's search text (fitSearchText), so "rifter scram" finds Rifters with a warp scrambler.
  The words after "tag:" or "tags:" that name a tag, by its name or a start of three letters or more that fits only it (fitTagNamed), are tags a fit must have, so "rifter tags: abyssal imp" finds Rifters tagged both.
  A tag's name may be several words, ie Fleet Doctrine, so the longest run of words naming one is taken first (fitTagAt).
  Any other word after them is an ordinary word, so typing on after clicking a tag narrows by name too.
  While filtering, the hulls with a match unfold, once each so one folded again stays folded; clearing the box folds those that were folded before, but the chosen fit's.
*/
function filterFits(fits, rows, hulls) {
  const [named, ...tagged] = el("fits-filter").value.toLowerCase().split(/\btags?:/);
  const words = named.split(/\s+/).filter(Boolean);
  const tags = [];
  const after = tagged.join(" ").split(/[\s,]+/).filter(Boolean);
  for (let at = 0; at < after.length; ) {
    const [tag, used] = fitTagAt(after, at);
    if (tag) tags.push(tag);
    else words.push(after[at]);
    at += used;
  }
  const filtering = words.length > 0 || tags.length > 0;
  const matched = new Set();
  for (const fit of fits) {
    const text = fitsShown.searchText.get(fit);
    const match = tags.every((tag) => fitHasTag(fit, tag)) && words.every((word) => text.includes(word));
    rows.get(fit).parentElement.hidden = !match;
    if (match) matched.add(fit.shipName);
  }

  if (filtering) fitsShown.filterOpened ??= new Map();
  const chosen = fitsShown.byKillmail.get(fitsShown.selected)?.shipName;
  for (const [shipName, hull] of hulls) {
    hull.item.hidden = !matched.has(shipName);
    if (filtering && matched.has(shipName) && !fitsShown.filterOpened.has(shipName)) {
      fitsShown.filterOpened.set(shipName, fitsShown.folded.delete(shipName));
      hull.unfold();
    } else if (!filtering && fitsShown.filterOpened?.get(shipName) && shipName !== chosen) {
      fitsShown.folded.add(shipName);
      hull.fold();
    }
  }
  if (!filtering) fitsShown.filterOpened = null;
  el("fits-no-match").hidden = fits.length === 0 || matched.size > 0;
}

/*
  fitSearchText is what the filter box matches a fit against, lowercased: its hull, its modules, and the pilot, corp and alliance of each loss.
  Every loss's modules are read, as a merged fit's can differ (fit_merged_groups, ie its ECM flavors), as does who lost it.
  Names are joined on a line break, which no word holds, so a word matches within one name.
*/
function fitSearchText(fit) {
  const modules = new Set(fit.losses.flatMap((loss) => FIT_RACKS.flatMap((rack) => (loss.fit[rack.key] || []).map((slot) => slot?.module.name))));
  const owners = fit.losses.flatMap((loss) => [loss.pilotName, loss.corpName, loss.allianceName]);
  return [fit.shipName, ...modules, ...owners].filter(Boolean).join("\n").toLowerCase();
}

// fitsHull is a hull's heading in the fit list, with its losses counted, and the list of its fits beneath that it folds away
function fitsHull(fit, ofHull) {
  const item = document.createElement("li");
  item.className = "fits-hull";
  const heading = document.createElement("button");
  heading.type = "button";
  heading.className = "fits-hull-head";
  const image = document.createElement("img");
  image.src = fit.shipImageUrl;
  image.alt = "";
  const name = document.createElement("span");
  name.className = "fits-hull-name";
  name.textContent = fit.shipName;
  const count = document.createElement("span");
  count.className = "fits-hull-count";
  const losses = ofHull.reduce((sum, each) => sum + each.losses.length, 0);
  count.textContent = ofHull.length === losses ? plural(losses, "loss", "losses") : `${plural(losses, "loss", "losses")} · ${plural(ofHull.length, "fit", "fits")}`;
  heading.append(image, name, count);
  const rows = document.createElement("ul");
  rows.className = "fits-hull-rows";
  const show = (folded) => {
    heading.setAttribute("aria-expanded", String(!folded));
    rows.hidden = folded;
  };
  show(fitsShown.folded.has(fit.shipName));
  heading.addEventListener("click", () => {
    const folded = !fitsShown.folded.has(fit.shipName);
    if (folded) fitsShown.folded.add(fit.shipName);
    else fitsShown.folded.delete(fit.shipName);
    show(folded);
  });
  item.append(heading, rows);
  return { item, rows, unfold: () => show(false), fold: () => show(true) };
}

// fitRow is one fit in the list: when and where it was last lost, how many times, and its tags beneath. Who flew it is in the fitting window, and the filter box matches it.
function fitRow(fit) {
  const row = document.createElement("button");
  row.type = "button";
  row.className = "fit-row";
  row.setAttribute("aria-selected", "false");
  const text = document.createElement("span");
  text.className = "fit-row-text";
  // a system that could not be looked up has no name, and is left out
  const systems = new Set(fit.losses.map((loss) => loss.systemName).filter(Boolean));
  const when = document.createElement("span");
  when.textContent = [spokenDate(fit.lastLost), systems.size > 1 ? `${systems.size} systems` : [...systems][0]].filter(Boolean).join(" · ");
  // the fit's first role by priority names the row (retrieval.Fit.Tags orders them)
  const naming = (fit.tags || []).find((tag) => tag.role && !tag.hidden);
  if (naming) {
    const name = document.createElement("span");
    name.className = "fit-row-name";
    name.textContent = naming.name;
    name.title = fitTagReasons(naming);
    when.prepend(name, " · ");
  }
  text.append(when);
  // the naming tag is among them too, so the tag a fit is named for is always where its tags are
  const shown = (fit.tags || []).filter((tag) => !tag.hidden).slice(0, FIT_ROW_TAGS).map(fitRuleTag);
  const tags = [...shown, ...FIT_TAGS.filter((tag) => fit[tag.key])];
  if (tags.length > 0) {
    const line = document.createElement("span");
    line.className = "fit-row-tags";
    line.append(...tags.map((tag) => fitTag(tag, true)));
    text.append(line);
  }
  row.append(text);
  if (fit.losses.length > 1) {
    const count = document.createElement("span");
    count.className = "fit-row-count";
    count.textContent = `×${fit.losses.length}`;
    count.title = `Lost ${fit.losses.length} times with this fit`;
    row.append(count);
  }
  return row;
}

// FIT_TAGS mark a fit in the list as small filled badges, see FIT_KEY. Abyssal is red as it is a warning, so fixed; implants are in the theme's --badge color. The filter box finds a tag by its name.
const FIT_ABYSSAL_TAG = { key: "abyssal", name: "Abyssal", className: "fit-tag fit-tag-abyssal", meaning: "Contains Abyssal Module" };
const FIT_IMPLANTS_TAG = { key: "implants", name: "Implants", className: "fit-tag fit-tag-implants", meaning: "Contains Implants from a related capsule killmail" };
const FIT_TAGS = [FIT_ABYSSAL_TAG, FIT_IMPLANTS_TAG];

// FIT_ROW_TAGS is how many of a fit's tags from fit_tag_rules.json its row shows, the first by priority, the one naming it included; the fitting window shows them all
const FIT_ROW_TAGS = 3;

// fitRuleTag is a tag read by fit_tag_rules.json (retrieval.FitTag) as the fit list shows it, tinted in the theme's highlight so it doesn't read as the solid Abyssal or Implants. A hidden one (the weapons, Close, Long) is a muted outline instead.
function fitRuleTag(tag) {
  return { name: tag.name, meaning: tag.meaning, className: tag.hidden ? "fit-tag fit-tag-rule fit-tag-hidden" : "fit-tag fit-tag-rule", rule: true };
}

// fitRuleTags are the tags any loss of the search's fits has, hidden ones too so the filter finds them, by priority and then name, for the filter box and the key
function fitRuleTags(fits) {
  const byName = new Map();
  for (const fit of fits) {
    for (const loss of fit.losses) {
      for (const tag of loss.tags || []) if (!byName.has(tag.name)) byName.set(tag.name, tag);
    }
  }
  const rank = (tag) => tag.priority || Infinity;
  return [...byName.values()].sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name)).map((tag) => ({ ...fitRuleTag(tag), hidden: tag.hidden }));
}

// fitHasTag reports whether a fit has a tag: Abyssal and Implants by their flag, a rule's tag if any of its losses has it, as drones can differ between them
function fitHasTag(fit, tag) {
  if (!tag.rule) return Boolean(fit[tag.key]);
  return fit.losses.some((loss) => (loss.tags || []).some((each) => each.name === tag.name));
}

// fitTagReasons is a tag's hover text: its name, and the modules that gave it weight, or for Fleet Doctrine the counts it reached
function fitTagReasons(tag) {
  const reasons = (tag.reasons || []).filter((reason) => reason.weight > 0 || reason.needed).map((reason) => (reason.count > 1 ? `${reason.name} ×${reason.count}` : reason.name));
  return reasons.length ? `${tag.name}: ${reasons.join(", ")}` : tag.name;
}

// fitTagWeight writes a tag's weight to two places at most, ie 1.5 or 10.5, with a sign for a reason's
function fitTagWeight(weight, signed = false) {
  const shown = String(Math.round(weight * 100) / 100);
  return signed && weight >= 0 ? `+${shown}` : shown.replace("-", "−");
}

/*
  showFitTagTip fills the fitting window's tag tip with how a tag was reached and puts it under the tag: each module, drone, hull, or other tag with its count and the weight it gave,
  the part a hull bonus added on its own line beneath it so the rows still add up, then the total against the threshold, and the share of the racks against its own threshold for a tag with one (Firewaller).
  A reason counted against a number rather than weighed (Fleet Doctrine's hulls, logi, losses, and pilots) shows its count against the number needed.
  One tip serves every tag, fixed to the page, so the window's scrolling can't clip it.
*/
function showFitTagTip(chip, tag) {
  const tip = el("fit-tag-tip");
  const title = document.createElement("div");
  title.className = "fit-tag-tip-title";
  title.textContent = tag.name;
  const meaning = document.createElement("div");
  meaning.className = "fit-tag-tip-meaning";
  meaning.textContent = tag.meaning;

  const table = document.createElement("table");
  const row = (label, value, className = "") => {
    const line = document.createElement("tr");
    line.className = className;
    const name = document.createElement("td");
    name.textContent = label;
    const weight = document.createElement("td");
    weight.textContent = value;
    line.append(name, weight);
    table.append(line);
  };
  for (const reason of tag.reasons || []) {
    if (reason.needed) {
      row(reason.name, `${reason.count} of ${reason.needed} needed`);
      continue;
    }
    const counted = reason.count > 1 ? `${reason.name} ×${reason.count}` : reason.name;
    const bonus = reason.bonus || 0;
    row(reason.count === 0 ? `${counted} tag` : counted, fitTagWeight(reason.weight - bonus, true), reason.weight < 0 ? "fit-tag-tip-against" : "");
    if (bonus) row("↳ hull bonus", fitTagWeight(bonus, true), bonus < 0 ? "fit-tag-tip-bonus fit-tag-tip-against" : "fit-tag-tip-bonus");
  }
  if (tag.threshold > 0) row("Total", `${fitTagWeight(tag.weight)} of ${fitTagWeight(tag.threshold)} needed`, "fit-tag-tip-total");
  if (tag.shareThreshold > 0) row("Share of its racks", `${Math.round(tag.share * 100)}% of ${Math.round(tag.shareThreshold * 100)}% needed`, "fit-tag-tip-total");

  tip.replaceChildren(title, meaning, table);
  tip.hidden = false;
  // under the tag, or over it when there is no room below, kept inside the window's edges
  const at = chip.getBoundingClientRect();
  const width = tip.offsetWidth;
  const height = tip.offsetHeight;
  tip.style.left = `${Math.max(8, Math.min(at.left, window.innerWidth - width - 8))}px`;
  const below = at.bottom + 6;
  tip.style.top = `${below + height > window.innerHeight - 8 ? Math.max(8, at.top - height - 6) : below}px`;
}

function hideFitTagTip() {
  el("fit-tag-tip").hidden = true;
}

// FIT_TAG_PREFIX is the fewest letters of a tag's name the filter takes for the tag, so a short word after "tags:", ie part of a pilot's name, stays a word
const FIT_TAG_PREFIX = 3;

// fitTagNamed is the tag a filter word names: the one with that name, or else the only one whose name starts with it, if it is FIT_TAG_PREFIX letters or more. null for any other word.
function fitTagNamed(word) {
  const all = [...FIT_TAGS, ...fitsShown.ruleTags];
  const named = all.find((tag) => tag.name.toLowerCase() === word);
  if (named || word.length < FIT_TAG_PREFIX) return named || null;
  const started = all.filter((tag) => tag.name.toLowerCase().startsWith(word));
  return started.length === 1 ? started[0] : null;
}

// fitTagAt is the tag named by the longest run of filter words from at, ie "fleet doctrine" or "fleet doc", with how many words it took. A word naming none is [null, 1].
function fitTagAt(words, at) {
  for (let end = words.length; end > at; end--) {
    const tag = fitTagNamed(words.slice(at, end).join(" "));
    if (tag) return [tag, end - at];
  }
  return [null, 1];
}

// fitTag is a fit's tag, named on hover by the name the filter box finds it by; the key gives its meaning. A clickable one lists only the fits with that tag, in place of the filter box's words.
function fitTag(tag, clickable = false) {
  const mark = document.createElement("span");
  mark.className = tag.className;
  mark.textContent = tag.name;
  mark.title = tag.name;
  if (clickable) {
    mark.classList.add("fit-tag-filter");
    mark.addEventListener("click", (event) => {
      event.stopPropagation(); // filters, without selecting the row it is on
      filterFitsByTag(tag);
    });
  }
  return mark;
}

// filterFitsByTag lists only the fits with a tag, in place of the filter box's words
function filterFitsByTag(tag) {
  el("fits-filter").value = `tags: ${tag.name.toLowerCase()}`;
  fitsShown.filter?.();
}

/*
  filterHelp builds a filter box's round ? button, whose popup shows on hover or keyboard focus: what the box matches, as bullet points under an intro.
  Unlike a tab's help it has no window of its own.
*/
function filterHelp(intro, points) {
  const lead = document.createElement("p");
  lead.textContent = intro;
  return helpPopup("filter-help", "How to filter", [lead, ...helpContent(points, [])]).help;
}

// FITS_FILTER_HELP is the Fits filter box's ? popup
const FITS_FILTER_HELP = [
  "Filter results by:",
  [
    "Hull, pilot, module, corp, or alliance name.",
    'Several words and parts of words, ie "Rifter Web".',
    'The key word "tag:" or "tags:" followed by tag names, or their first three letters or more, ie "tags: neut" or "tags: imp". Clicking a tag does the same. Weapons and their range are tags too, ie "tags: lasers" or "tags: long", though they are never shown.',
    "Any other word after the tags is matched as a name.",
  ],
];

// FIT_RACKS place each rack of slots on the fitting window's ring, as EVE's does: highs over the top, mids down the right, lows along the bottom, and rigs and subsystems inside on the left. Angles are clockwise from the right, in degrees.
// The inner ring sits on the hull render's edge, its slots further apart than the outer ring's, so a T3 cruiser's rigs and subsystems keep a gap between them.
const FIT_RACKS = [
  { key: "highs", label: "High", centre: -90, ring: "outer" },
  { key: "mids", label: "Mid", centre: 0, ring: "outer" },
  { key: "lows", label: "Low", centre: 90, ring: "outer" },
  { key: "rigs", label: "Rig", centre: 145, ring: "inner" },
  { key: "subsystems", label: "Subsystem", centre: 215, ring: "inner" },
];
const FIT_SLOT_STEP = { outer: 11, inner: 16, far: 9.5 };

/*
  FIT_ARCS curve the rest of the fit around the outside of the ring, one icon each, named on hover. Cargo is not shown.
  Implants, in slot order, then boosters run down the left from its top left corner, and drones then fighters up the right from its bottom right.
*/
const FIT_ARCS = [
  { keys: ["implants", "boosters"], start: 225, direction: -1 },
  { keys: ["drones", "fighters"], start: 45, direction: -1 },
];

/*
  renderFitWindow draws a loss of a fit: the hull with its modules around it, loaded charges as a badge on their module, and an empty slot as a hollow socket.
  Arrows beside the ship's name (or ← → anywhere on the tab) page through its losses, wrapping round at either end, the one shown named under it, and its implants, boosters, drones and fighters curve around the outside (FIT_ARCS).
  The hull, each module and charge, and each of those has a right click menu to open it on EVE Ref.
*/
function renderFitWindow(fit, loss, choose) {
  el("fit-view-title").textContent = fit.shipName;
  const index = fit.losses.indexOf(loss);
  const count = fit.losses.length;
  const page = (step) => {
    if (count > 1) choose(fit.losses[(index + step + count) % count].killmailId);
  };
  const paged = count > 1;
  el("fit-prev").hidden = !paged;
  el("fit-next").hidden = !paged;
  el("fit-prev").onclick = () => page(-1);
  el("fit-next").onclick = () => page(1);
  el("fit-page-count").textContent = paged ? `${index + 1} of ${count}` : "";
  // anywhere on the tab, so a fit just picked from the list pages at once; a modified arrow is left to the WebView
  el("panel-fits").onkeydown = (event) => {
    // the arrows move the filter box's caret, not the loss
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey || event.target === el("fits-filter")) return;
    const step = { ArrowLeft: -1, ArrowRight: 1 }[event.key];
    if (!step) return;
    event.preventDefault();
    page(step);
  };

  // the loss shown, and its menu: its killmail, and the capsule whose implants it shows
  const sections = [
    killmailSection(loss.url),
    ...(loss.podUrl ? [{ heading: "Capsule", items: [{ label: "Open on zKillboard", action: () => window.runtime.BrowserOpenURL(loss.podUrl) }] }] : []),
  ];
  const shown = el("fit-view-loss");
  shown.replaceChildren([loss.pilotName, spokenDate(loss.time), loss.systemName].filter(Boolean).join(" · "));
  if (loss.podKillmailId) shown.append(" ", fitTag(FIT_IMPLANTS_TAG, true));
  shown.oncontextmenu = (event) => openSectionsMenu(event, sections);
  // every tag of this loss by name, with how it was reached on hover or focus (showFitTagTip), the hidden ones last and muted, as the list never shows them
  hideFitTagTip();
  const tags = loss.tags || [];
  el("fit-view-tags").replaceChildren(...[...tags.filter((tag) => !tag.hidden), ...tags.filter((tag) => tag.hidden)].map((tag) => {
    const chip = fitTag(fitRuleTag(tag), true);
    chip.removeAttribute("title");
    chip.tabIndex = 0;
    chip.addEventListener("mouseenter", () => showFitTagTip(chip, tag));
    chip.addEventListener("focus", () => showFitTagTip(chip, tag));
    chip.addEventListener("mouseleave", hideFitTagTip);
    chip.addEventListener("blur", hideFitTagTip);
    return chip;
  }));
  const zkill = el("fit-zkill");
  zkill.disabled = false;
  zkill.onclick = () => window.runtime.BrowserOpenURL(loss.url);
  zkill.oncontextmenu = (event) => openSectionsMenu(event, sections);

  const note = el("fit-view-note");
  note.hidden = !loss.missingTypes;
  note.textContent = loss.missingTypes ? `${loss.missingTypes} item ${loss.missingTypes === 1 ? "type is" : "types are"} newer than GoSleuth's item data, so this fit may be missing pieces.` : "";
  el("fit-copy").disabled = false;
  el("fit-copy").onclick = () => window.runtime.ClipboardSetText(loss.eft)
    .then((copied) => tickerPush(copied ? "Fit copied" : "Could not copy the fit"))
    .catch(() => tickerPush("Could not copy the fit"));

  const render = document.createElement("img");
  render.className = "fit-hull-render";
  render.src = fit.shipRenderUrl;
  render.alt = "";
  withMenu(render, [typeSection({ name: fit.shipName, eveRefUrl: fit.shipEveRefUrl })]);
  const slots = [render];
  for (const rack of FIT_RACKS) {
    const rackSlots = loss.fit[rack.key] || [];
    const step = FIT_SLOT_STEP[rack.ring];
    rackSlots.forEach((slot, index) => {
      const socket = document.createElement("span");
      // not .empty, which is the page's padded "nothing here" message
      socket.className = `fit-slot fit-slot-${rack.ring}${slot ? "" : " fit-slot-empty"}`;
      socket.style.setProperty("--angle", `${rack.centre + (index - (rackSlots.length - 1) / 2) * step}deg`);
      if (!slot) {
        socket.title = `Empty ${rack.label.toLowerCase()} slot`;
      } else {
        socket.title = slot.charge ? `${slot.module.name}\n${slot.charge.name}` : slot.module.name;
        socket.append(fitIcon(slot.module, "fit-module"));
        if (slot.charge) socket.append(fitIcon(slot.charge, "fit-charge"));
        withMenu(socket, [typeSection(slot.module), ...(slot.charge ? [typeSection(slot.charge)] : [])]);
      }
      slots.push(socket);
    });
  }
  for (const arc of FIT_ARCS) {
    arc.keys.flatMap((key) => loss.fit[key] || []).forEach((stack, index) => {
      const item = document.createElement("span");
      item.className = "fit-slot fit-slot-far";
      item.style.setProperty("--angle", `${arc.start + arc.direction * index * FIT_SLOT_STEP.far}deg`);
      item.title = stack.quantity > 1 ? `${stack.name} ×${stack.quantity.toLocaleString()}` : stack.name;
      item.append(fitIcon(stack, "fit-module"));
      if (stack.quantity > 1) {
        const count = document.createElement("span");
        count.className = "fit-stack-count";
        count.textContent = stack.quantity.toLocaleString();
        item.append(count);
      }
      withMenu(item, [typeSection(stack)]);
      slots.push(item);
    });
  }
  el("fit-ring").replaceChildren(...slots);
}

// fitIcon is an item type's icon in the fitting window
function fitIcon(part, className) {
  const image = document.createElement("img");
  if (className) image.className = className;
  image.src = part.imageUrl;
  image.alt = "";
  return image;
}

// typeSection is an item type's menu section, ie a module in the fitting window, opening it on EVE Ref
function typeSection(part) {
  return { heading: part.name, items: [{ label: "Open on EVE Ref", action: () => window.runtime.BrowserOpenURL(part.eveRefUrl) }] };
}

// showFit switches to the Fits tab and selects a killmail's fit, from a killmail's right click menu
function showFit(killmailId) {
  if (!fitsShown.select) return;
  const row = fitsShown.select(killmailId);
  if (!row) return;
  selectTab("tab-fits");
  row.scrollIntoView({ block: "nearest" });
}

// hasFit is whether a killmail is one of the target's losses listed on the Fits tab, or the capsule lost with one
function hasFit(killmailId) {
  return fitsShown.byKillmail.has(killmailId);
}

// isFitLoss is whether a killmail is this loss, or the capsule whose implants it shows
function isFitLoss(loss, killmailId) {
  // podKillmailId is left out when there is no capsule, so no id must not match it
  return Boolean(killmailId) && (loss.killmailId === killmailId || loss.podKillmailId === killmailId);
}

// renderPIPilots fills the PI Pilots tab of a system search: each pilot with PI losses, haulers near its planets or any ship with PI aboard, newest first
function renderPIPilots(pilots) {
  el("pi-list").replaceChildren(...overallLossBlock(pilots), ...pilots.map((pilot) => lossPilotBlock(pilot, piLossRow)));
  el("pi-empty").hidden = pilots.length > 0;
}

// renderRatters fills the Ratters tab of a system search: each pilot who lost a ship with NPCs on it, those with only NPCs or smartbombs on a loss first
function renderRatters(ratters) {
  el("ratters-list").replaceChildren(...overallLossBlock(ratters), ...ratters.map((ratter) => lossPilotBlock(ratter, ratLossRow)));
  el("ratters-empty").hidden = ratters.length > 0;
}

// OVERALL_ACTIVITY_MIN is the fewest losses a PI Pilots or Ratters tab needs before it shows the hours of them all
const OVERALL_ACTIVITY_MIN = 3;

// overallLossBlock is the PI Pilots and Ratters tabs' top row: the hours every pilot's losses fell in together. It is left out, an empty list, with too few losses to show a pattern.
function overallLossBlock(pilots) {
  const times = pilots.flatMap((pilot) => pilot.losses.map((loss) => loss.time));
  if (times.length < OVERALL_ACTIVITY_MIN) return [];

  const block = document.createElement("section");
  block.className = "pi-pilot pi-overall";
  const header = document.createElement("div");
  header.className = "pi-pilot-header";

  const who = document.createElement("div");
  who.className = "pi-who";
  const title = document.createElement("span");
  title.className = "pi-name";
  title.textContent = "Overall";
  const pilotCount = document.createElement("span");
  pilotCount.className = "group-line";
  pilotCount.textContent = `${pilots.length} ${pilots.length === 1 ? "pilot" : "pilots"}`;
  who.append(title, pilotCount);

  const count = document.createElement("span");
  count.className = "pi-count";
  count.textContent = `${times.length} losses`;
  header.append(who, hourStrip(times, "loss", "losses"), count);
  block.append(header);
  return [block];
}

// lossPilotBlock is a pilot with their losses in the system, for the PI Pilots and Ratters tabs: a header with who they are and when they lose ships, then a row per loss from lossRow
function lossPilotBlock(pilot, lossRow) {
  const block = document.createElement("section");
  block.className = "pi-pilot";

  const header = document.createElement("div");
  header.className = "pi-pilot-header";
  header.append(siteLinks(pilot.name, pilot, true));
  const portrait = document.createElement("img");
  portrait.src = pilot.imageUrl;
  portrait.alt = "";
  portrait.loading = "lazy";

  const who = document.createElement("div");
  who.className = "pi-who";
  const name = document.createElement("button");
  name.type = "button";
  name.className = "link-button pi-name";
  name.textContent = pilot.name;
  name.title = "Search for this";
  name.addEventListener("click", () => offerSearch({ type: "PILOT", name: pilot.name, id: pilot.id }));
  who.append(name);
  const group = groupLine(pilot.corporation, pilot.alliance);
  if (group) who.append(group);

  const count = document.createElement("span");
  count.className = "pi-count";
  count.textContent = `${pilot.losses.length} ${pilot.losses.length === 1 ? "loss" : "losses"}`;
  header.append(portrait, who);
  header.append(hourStrip(pilot.losses.map((loss) => loss.time), "loss", "losses"), count);
  withMenu(header, pilotSections(pilot));

  const losses = document.createElement("ol");
  losses.className = "pi-losses";
  // a loss's menu leads with the killmail, then the pilot's
  losses.append(...pilot.losses.map((loss) => withMenu(lossRow(loss), [killmailSection(loss.url), ...pilotSections(pilot)])));

  block.append(header, losses);
  return block;
}

/*
  hourStrip is a one row histogram of the hours, 00 to 23, that times fall in, shaded like the Activity tab and in EVE or local time as chosen.
  noun and nouns name what was counted, for each hour's tooltip, ie loss and losses.
*/
function hourStrip(times, noun, nouns) {
  const counts = Array(24).fill(0);
  for (const time of times) counts[timeParts(time).hour]++;
  const busiest = Math.max(0, ...counts);
  const zone = preferences.timeZone === "local" ? "local" : "EVE";

  const strip = document.createElement("div");
  strip.className = "hour-strip";
  counts.forEach((count, hour) => {
    const cell = document.createElement("span");
    cell.className = "hour-cell";
    cell.title = `${pad2(hour)}:00–${pad2(hour)}:59 ${zone}: ${count} ${count === 1 ? noun : nouns}`;
    if (count) cell.style.background = `color-mix(in srgb, var(--accent) ${Math.round(25 + 75 * (count / busiest))}%, var(--panel))`;
    strip.append(cell);
  });
  // a few hour marks under the cells
  for (const hour of [0, 6, 12, 18]) {
    const mark = document.createElement("span");
    mark.className = "hour-mark";
    mark.style.gridColumn = `${hour + 1} / span 6`;
    mark.textContent = pad2(hour);
    strip.append(mark);
  }
  return strip;
}

// piLossRow is one PI loss: when, the ship, which planet if it was near one, its value, and the PI materials aboard. Clicking it opens the killmail on zKillboard.
function piLossRow(loss) {
  const row = document.createElement("li");
  const carried = loss.cargo.length > 0;
  row.className = "pi-loss";
  row.tabIndex = 0;
  // a loss with cargo shows it on hover instead, which would sit under this tooltip
  if (!carried) row.title = "Open on zKillboard";

  const time = document.createElement("span");
  time.className = "pi-time";
  time.textContent = formatKillmailTime(loss.time);

  const ship = document.createElement("span");
  ship.className = "pi-ship";
  const shipImage = document.createElement("img");
  shipImage.src = loss.shipImageUrl;
  shipImage.alt = "";
  shipImage.loading = "lazy";
  ship.append(shipImage, loss.shipName);

  const planet = document.createElement("span");
  planet.className = "pi-planet";
  planet.textContent = loss.planet || "—";
  if (!loss.planet) planet.title = "Not near a planet";

  const value = document.createElement("span");
  value.className = "pi-value";
  value.textContent = formatIsk(loss.totalValue);

  const tag = document.createElement("span");
  tag.className = "pi-tag";
  tag.textContent = carried ? "PI" : ""; // only a loss with PI aboard is marked

  row.append(time, ship, planet, value, tag);

  // the PI materials aboard, shown while hovering over or focused on the row
  if (carried) {
    const cargo = document.createElement("div");
    cargo.className = "pi-cargo";
    cargo.setAttribute("role", "tooltip");
    const heading = document.createElement("div");
    heading.className = "pi-cargo-heading";
    heading.textContent = "PI cargo";
    const items = document.createElement("ul");
    items.append(...loss.cargo.map((item) => {
      const line = document.createElement("li");
      const name = document.createElement("span");
      name.textContent = item.name;
      const quantity = document.createElement("span");
      quantity.className = "pi-cargo-quantity";
      quantity.textContent = `×${item.quantity.toLocaleString("en-US")}`;
      line.append(name, quantity);
      return line;
    }));
    const hint = document.createElement("div");
    hint.className = "pi-cargo-hint";
    hint.textContent = "Click to open on zKillboard";
    cargo.append(heading, items, hint);
    row.append(cargo);
  }

  const open = () => window.runtime.BrowserOpenURL(loss.url);
  row.addEventListener("click", open);
  row.addEventListener("keydown", (event) => {
    if (event.key === "Enter") open();
  });
  return row;
}

// RAT_KINDS label a ratting loss by who was on it, surest first: its tag, and the tag's tooltip
const RAT_KINDS = {
  npc: ["NPC", "Only NPCs were on this killmail"],
  smartbomb: ["SB", "NPCs, and players who only hit it with smartbombs"],
  players: ["PVP", "NPCs, and players with other weapons"],
};

// ratLossRow is one ratting loss: when, the ship, the nearest celestial, its value, and who was on it. The NPCs and players on it show on hover. Clicking it opens the killmail on zKillboard.
function ratLossRow(loss) {
  const row = document.createElement("li");
  row.className = "pi-loss rat-loss";
  row.tabIndex = 0;

  const time = document.createElement("span");
  time.className = "pi-time";
  time.textContent = formatKillmailTime(loss.time);

  const ship = document.createElement("span");
  ship.className = "pi-ship";
  const shipImage = document.createElement("img");
  shipImage.src = loss.shipImageUrl;
  shipImage.alt = "";
  shipImage.loading = "lazy";
  ship.append(shipImage, loss.shipName);

  const value = document.createElement("span");
  value.className = "pi-value";
  value.textContent = formatIsk(loss.totalValue);

  const [label, meaning] = RAT_KINDS[loss.kind] || ["", ""];
  const tag = document.createElement("span");
  tag.className = `pi-tag rat-kind-${loss.kind}`;
  tag.textContent = label;

  row.append(time, ship, value, tag);

  // the NPCs and players on it, shown while hovering over or focused on the row
  const attackers = document.createElement("div");
  attackers.className = "pi-cargo";
  attackers.setAttribute("role", "tooltip");
  const heading = document.createElement("div");
  heading.className = "pi-cargo-heading";
  heading.textContent = meaning;
  const items = document.createElement("ul");
  const line = (text, count) => {
    const item = document.createElement("li");
    const name = document.createElement("span");
    name.textContent = text;
    const quantity = document.createElement("span");
    quantity.className = "pi-cargo-quantity";
    quantity.textContent = `×${count.toLocaleString("en-US")}`;
    item.append(name, quantity);
    return item;
  };
  items.append(...loss.rats.map((rat) => line(rat.name || "Unknown NPC", rat.count)));
  if (loss.players) items.append(line(loss.players === 1 ? "Player" : "Players", loss.players));
  const hint = document.createElement("div");
  hint.className = "pi-cargo-hint";
  hint.textContent = "Click to open on zKillboard";
  attackers.append(heading, items, hint);
  row.append(attackers);

  const open = () => window.runtime.BrowserOpenURL(loss.url);
  row.addEventListener("click", open);
  row.addEventListener("keydown", (event) => {
    if (event.key === "Enter") open();
  });
  return row;
}

// DEFAULT_TAB_ORDER is the tabs' ids as index.html lists them, read before a saved order moves them
const DEFAULT_TAB_ORDER = [...document.querySelectorAll("#tab-list .tab")].map((tab) => tab.id);

/*
  tabOrderFrom turns a saved tab order into the order to show: the saved tabs fill the places they have in the default order, in their saved order,
  so a tab the saved order does not know, ie one added in a later version, keeps its default place. Ids that are not tabs are ignored.
*/
function tabOrderFrom(saved) {
  const known = saved.filter((id, index) => DEFAULT_TAB_ORDER.includes(id) && saved.indexOf(id) === index);
  let next = 0;
  return DEFAULT_TAB_ORDER.map((id) => (known.includes(id) ? known[next++] : id));
}

// arrangeTabs puts the sidebar's tabs in an order, hidden ones included, below At a glance
function arrangeTabs(order) {
  for (const id of order) el("tab-list").append(el(id));
}

// saveTabOrder saves the tabs' order as shown. If it can not be saved it still applies for this session.
function saveTabOrder() {
  backend().SetTabOrder(shownTabOrder()).catch((err) => setStatus(String(err), true));
}

// shownTabOrder is the tabs' ids in the order they are shown
function shownTabOrder() {
  return [...el("tab-list").querySelectorAll(".tab")].map((tab) => tab.id);
}

// showTabOrderReset enables the Preferences window's Reset tab order button only when the tabs have been moved
function showTabOrderReset() {
  el("reset-tab-order").disabled = shownTabOrder().every((id, index) => id === DEFAULT_TAB_ORDER[index]);
}

// resetTabOrder puts the tabs back in their default order and saves that, from the Preferences window
function resetTabOrder() {
  arrangeTabs(DEFAULT_TAB_ORDER);
  showTabOrderReset();
  backend().SetTabOrder([])
    .then(() => tickerPush("Tab order reset"))
    .catch((err) => showPreferencesError(String(err)));
}

// draggedTab is the tab being dragged to a new place, null when none is
let draggedTab = null;

function clearTabDropMarks() {
  for (const tab of el("tab-list").querySelectorAll(".tab")) tab.classList.remove("tab-drop-before", "tab-drop-after");
}

// dropsBefore reports whether a drag over a tab would put the dragged tab above it, from which half of the tab the pointer is over
function dropsBefore(tab, event) {
  const box = tab.getBoundingClientRect();
  return event.clientY < box.top + box.height / 2;
}

/*
  wireTabMoving lets a tab be dragged into a new place among the others, or moved one place with Alt and the up or down arrow.
  Either way the new order is saved.
*/
function wireTabMoving(tab) {
  tab.draggable = true;

  tab.addEventListener("dragstart", (event) => {
    draggedTab = tab;
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("text/plain", tab.id);
    tab.classList.add("tab-dragging");
  });
  tab.addEventListener("dragover", (event) => {
    if (!draggedTab || draggedTab === tab) return;
    event.preventDefault(); // allows the drop
    event.dataTransfer.dropEffect = "move";
    const before = dropsBefore(tab, event);
    clearTabDropMarks();
    tab.classList.add(before ? "tab-drop-before" : "tab-drop-after");
  });
  tab.addEventListener("dragleave", () => tab.classList.remove("tab-drop-before", "tab-drop-after"));
  tab.addEventListener("drop", (event) => {
    if (!draggedTab || draggedTab === tab) return;
    event.preventDefault();
    el("tab-list").insertBefore(draggedTab, dropsBefore(tab, event) ? tab : tab.nextSibling);
    clearTabDropMarks();
    saveTabOrder();
  });
  tab.addEventListener("dragend", () => {
    tab.classList.remove("tab-dragging");
    clearTabDropMarks();
    draggedTab = null;
  });

  tab.addEventListener("keydown", (event) => {
    if (!event.altKey || (event.key !== "ArrowUp" && event.key !== "ArrowDown")) return;
    event.preventDefault();
    // past the next tab that is shown, hidden ones are skipped over
    const shown = [...el("tab-list").querySelectorAll(".tab")].filter((other) => !other.hidden);
    const at = shown.indexOf(tab);
    const neighbour = shown[at + (event.key === "ArrowUp" ? -1 : 1)];
    if (!neighbour) return;
    el("tab-list").insertBefore(tab, event.key === "ArrowUp" ? neighbour : neighbour.nextSibling);
    tab.focus();
    saveTabOrder();
  });
}

// sidebarCollapsed is whether the user shrank the sidebar to its icons, saved for the next run
let sidebarCollapsed = false;
// sidebarSaving is the last save of sidebarCollapsed, the next waiting on it
let sidebarSaving = Promise.resolve();

// NARROW_WINDOW matches a window too narrow for the full sidebar, which then shows only its icons whatever was chosen
const NARROW_WINDOW = window.matchMedia("(max-width: 760px)");

// setSidebarCollapsed shrinks the sidebar to its icons, or shows it in full again
function setSidebarCollapsed(collapsed) {
  sidebarCollapsed = collapsed;
  showSidebar();
}

// showSidebar lays the sidebar out as chosen, or as icons in a narrow window. Icons alone name their tab in a tooltip.
function showSidebar() {
  const collapsed = sidebarCollapsed || NARROW_WINDOW.matches;
  el("intel-tabs").classList.toggle("sidebar-collapsed", collapsed);
  const toggle = el("sidebar-toggle");
  toggle.hidden = NARROW_WINDOW.matches;
  toggle.textContent = collapsed ? "»" : "«";
  toggle.setAttribute("aria-expanded", String(!collapsed));
  const label = collapsed ? "Show the sidebar in full" : "Shrink the sidebar to its icons";
  toggle.title = label;
  toggle.setAttribute("aria-label", label);

  for (const tab of document.querySelectorAll(".tab")) {
    // a tab's own title, ie Allies?' note, is kept to show again in full
    if (tab.dataset.title === undefined) tab.dataset.title = tab.title;
    const name = tab.querySelector(".tab-name").textContent;
    const count = tab.querySelector(".tab-count").dataset.count;
    tab.title = collapsed ? [name + (count ? ` (${count})` : ""), tab.dataset.title].filter(Boolean).join(": ") : tab.dataset.title;
  }
}

// GLANCE_ROWS is how many of a tab's top rows its At a glance card shows
const GLANCE_ROWS = 2;

// GLANCE_ORDER is At a glance's cards in their set order, the tabs' default order whatever order the sidebar is in, with a j-space system's last structure lost after Activity
const GLANCE_ORDER = DEFAULT_TAB_ORDER.flatMap((id) => (id === "tab-activity" ? [id, "glance-structure-loss"] : [id]));

// GLANCE_OPENS is the tab a card that is not a tab's own opens
const GLANCE_OPENS = { "glance-structure-loss": "tab-activity" };

/*
  renderGlance fills At a glance: a card for each tab this search has, in a set order, with its top few rows and a link to open the full tab.
  It reads only the report already fetched, so it costs no API calls. Members has no card, as its rows load page by page.
*/
function renderGlance(report) {
  const cards = [];
  for (const id of GLANCE_ORDER) {
    const tabId = GLANCE_OPENS[id] || id;
    if (el(tabId).hidden || !GLANCE_CARDS[id]) continue;
    const card = GLANCE_CARDS[id](report);
    if (card) cards.push(glanceCard(tabId, card));
  }
  el("glance-cards").replaceChildren(...cards);
  showSidebar(); // the counts in the icons' tooltips
}

/*
  glanceCard builds one tab's card: its icon and name, an Open link to the tab, its rows, and a note under them.

  id: the id of the tab it opens, ie tab-systems
  card: { rows, note, title, iconFrom }
    - rows: [{ name, value, imageUrl, detail, className, menu }], name and value are text, detail is a muted line under the name, className colors the name,
      menu is the row's right-click menu as withMenu takes it, the same as its row in the tab, or none
    - note: a muted line under the rows, empty for none
    - title, iconFrom: for a card that is not a tab's own, its title and the tab whose icon it shows. A tab's own card leaves them out.
*/
function glanceCard(id, card) {
  const section = document.createElement("section");
  section.className = "glance-card";

  const header = document.createElement("header");
  const icon = el(card.iconFrom || id).querySelector(".tab-icon").cloneNode(true);
  const title = document.createElement("h3");
  title.textContent = card.title || el(id).querySelector(".tab-name").textContent;
  const open = document.createElement("button");
  open.type = "button";
  open.className = "link-button glance-open";
  open.textContent = "Open →";
  open.addEventListener("click", () => selectTab(id));
  header.append(icon, title, open);
  section.append(header);

  if (card.rows.length) {
    const list = document.createElement("ul");
    list.className = "glance-rows";
    list.append(...card.rows.map((row) => {
      const item = document.createElement("li");
      if (row.imageUrl) {
        const image = document.createElement("img");
        image.src = row.imageUrl;
        image.alt = "";
        image.loading = "lazy";
        item.append(image);
      }
      const text = document.createElement("div");
      text.className = "glance-text";
      const name = document.createElement("span");
      name.className = `glance-name ${row.className || ""}`.trim();
      name.textContent = row.name;
      text.append(name);
      if (row.detail) {
        const detail = document.createElement("span");
        detail.className = "glance-detail";
        detail.textContent = row.detail;
        text.append(detail);
      }
      const value = document.createElement("span");
      value.className = "glance-value";
      value.textContent = row.value;
      item.append(text, value);
      return row.menu ? withMenu(item, row.menu) : item;
    }));
    section.append(list);
  }
  if (card.note || !card.rows.length) {
    const note = document.createElement("p");
    note.className = "glance-note";
    note.textContent = card.note || "Nothing found.";
    note.title = note.textContent; // in full, if the line is cut short
    section.append(note);
  }
  return section;
}

// glanceGroup is a pilot's or corp's corp and alliance as one muted line, ie Brave Newbies · Brave Collective
function glanceGroup(corp, alliance) {
  return [corp, alliance].filter(Boolean).map((ref) => ref.name).join(" · ");
}

// topBy is the first GLANCE_ROWS of a list, sorted by a count, most first, the list left as it was
function topBy(list, count) {
  return [...list].sort((a, b) => count(b) - count(a)).slice(0, GLANCE_ROWS);
}

// rankedPilots is the card of a top ten pilots tab, as they are ranked
function rankedPilots(pilots) {
  return {
    rows: pilots.slice(0, GLANCE_ROWS).map((pilot) => ({ name: pilot.name, imageUrl: pilot.imageUrl, detail: glanceGroup(pilot.corporation, pilot.alliance), value: plural(pilot.killmailsShared, "killmail", "killmails"), menu: pilotSections(pilot) })),
  };
}

// busiestHour is the hour of the day most of the times fall in, ie 19:00 EVE, in EVE or local time as chosen in Preferences. A tie goes to the earliest hour.
function busiestHour(times) {
  const counts = activityCounts(times);
  const hours = Array.from({ length: 24 }, (_, hour) => counts.reduce((sum, day) => sum + day[hour], 0));
  return `${pad2(hours.indexOf(Math.max(...hours)))}:00 ${preferences.timeZone === "local" ? "local" : "EVE"}`;
}

// lossPilots is the card of the PI Pilots or Ratters tab: the pilots who lost the most ships there, each with about the hour they most often lost one
function lossPilots(pilots) {
  return {
    rows: topBy(pilots, (pilot) => pilot.losses.length).map((pilot) => ({ name: pilot.name, imageUrl: pilot.imageUrl, detail: glanceGroup(pilot.corporation, pilot.alliance), value: `~${busiestHour(pilot.losses.map((loss) => loss.time))}`, menu: pilotSections(pilot) })),
    note: pilots.length > GLANCE_ROWS ? `${plural(pilots.length, "pilot", "pilots")} in all` : "",
  };
}

/*
  GLANCE_CARDS builds each tab's At a glance card from the report, { rows, note } as glanceCard takes, or null for none.
  A tab missing here, ie Members, has no card.
*/
const GLANCE_CARDS = {
  "tab-killmails": (report) => {
    const kms = report.killmails || [];
    if (!kms.length) return { rows: [], note: "No killmails found." };
    const note = killmailSpan(...timeRange(kms.map((km) => km.time)), true);
    // a system has no side, its killmails are only destroyed value
    if (!kms[0].outcome) {
      const destroyed = kms.reduce((sum, km) => sum + km.totalValue, 0);
      return { rows: [{ name: "Killmails", value: kms.length.toLocaleString("en-US") }, { name: "Destroyed", value: formatIsk(destroyed) }], note };
    }
    const kills = kms.filter((km) => km.outcome === "kill");
    const losses = kms.filter((km) => km.outcome !== "kill");
    const value = (list) => formatIsk(list.reduce((sum, km) => sum + km.totalValue, 0));
    return {
      rows: [
        // the count and its isk on one line, keeping the card as short as the others
        { name: "Kills", className: "glance-kill", value: `${kills.length.toLocaleString("en-US")} · ${value(kills)}` },
        { name: "Losses", className: "glance-loss", value: `${losses.length.toLocaleString("en-US")} · ${value(losses)}` },
      ],
      note,
    };
  },
  "tab-activity": (report) => {
    const kms = report.killmails || [];
    if (!kms.length) return null;
    const days = activityCounts(kms.map((km) => km.time)).map((hours) => hours.reduce((sum, count) => sum + count, 0));
    const busiestDay = ACTIVITY_DAYS.find(([, weekday]) => weekday === days.indexOf(Math.max(...days)))[0];
    return {
      rows: [
        { name: "Busiest day", value: busiestDay },
        { name: "Busiest hour", value: busiestHour(kms.map((km) => km.time)) },
      ],
    };
  },
  // a j-space system's newest structure lost, however long ago, as the Activity tab shows it
  "glance-structure-loss": (report) => {
    if (!report.jspace) return null;
    const card = { title: "Last Structure Lost", iconFrom: "tab-structures", rows: [] };
    const loss = report.lastStructureLoss;
    if (!loss) return card;
    const killer = loss.death.killerAlliance || loss.death.killerCorp;
    card.rows.push({ name: loss.type, imageUrl: loss.imageUrl, detail: glanceGroup(loss.owner, loss.ownerAlliance), value: spokenDate(loss.death.time), menu: structureLossSections(loss, false) });
    card.note = [killer && `Killed by ${killer.name}`, plural(loss.death.attackers, "attacker", "attackers")].filter(Boolean).join(" · ");
    return card;
  },
  "tab-fleetmates": (report) => rankedPilots(report.fleetmates),
  "tab-members": (report) => rankedPilots(report.members),
  "tab-pilots": (report) => rankedPilots(report.pilots),
  "tab-corps": (report) => ({
    rows: report.corporations.slice(0, GLANCE_ROWS).map((corp) => ({
      name: corp.name,
      imageUrl: corp.imageUrl,
      detail: glanceGroup(null, corp.alliance),
      value: plural(corp.killmails, "killmail", "killmails"),
      menu: corpSections(corp),
    })),
  }),
  "tab-systems": (report) => ({
    rows: report.systems.slice(0, GLANCE_ROWS).map((system) => ({
      name: system.name,
      detail: [system.class || system.securityStatus.toFixed(1), system.region].filter(Boolean).join(" · "),
      value: plural(system.killmails, "killmail", "killmails"),
      menu: [systemSection(system)],
    })),
  }),
  "tab-allies": (report) => ({
    // built as the menu opens, as in the Allies tab
    rows: report.allies.slice(0, GLANCE_ROWS).map((ally) => ({ name: ally.name, imageUrl: ally.imageUrl, detail: plural(ally.pilots, "pilot", "pilots"), value: plural(ally.killmails, "kill", "kills"), menu: () => allySections(ally) })),
  }),
  "tab-capitals": (report) => ({
    rows: topBy(report.capitals, (capital) => capital.pilots.length).map((capital) => ({ name: capital.name, imageUrl: capital.imageUrl, value: plural(capital.pilots.length, "pilot", "pilots") })),
  }),
  "tab-structures": (report) => ({
    rows: [
      { name: "Used in kills", value: report.structures.used.length.toLocaleString("en-US") },
      { name: "Destroyed", value: report.structures.destroyed.length.toLocaleString("en-US") },
      { name: "Lost", value: report.structures.lost.length.toLocaleString("en-US") },
    ],
  }),
  "tab-fits": (report) => {
    // the hulls lost most often, each with its losses
    const hulls = new Map();
    for (const fit of report.fits) {
      const hull = hulls.get(fit.shipName) || { name: fit.shipName, imageUrl: fit.shipImageUrl, losses: 0 };
      hull.losses += fit.losses.length;
      hulls.set(fit.shipName, hull);
    }
    return {
      rows: topBy([...hulls.values()], (hull) => hull.losses).map((hull) => ({ name: hull.name, imageUrl: hull.imageUrl, value: plural(hull.losses, "loss", "losses") })),
      note: hulls.size > GLANCE_ROWS ? `${plural(report.fits.length, "fit", "fits")} across ${plural(hulls.size, "hull", "hulls")}` : "",
    };
  },
  "tab-pi": (report) => lossPilots(report.piPilots),
  "tab-ratters": (report) => lossPilots(report.ratters),
};

// UNIQUE_DAYS_HELP explains the Unique Days column the ranked tabs share
const UNIQUE_DAYS_HELP = "Unique Days: total days with killmails (days spanned across)";

// PILOT_MENU_HELP and CORP_MENU_HELP close the help of tabs whose right-click menus only search and link, as most do. A tab with other menu items says so in its own words.
const PILOT_MENU_HELP = "Right-click a pilot, corp, or alliance to search it, or open it on an external site.";
const CORP_MENU_HELP = "Right-click a corp or alliance to search it, or open it on an external site.";

/*
  TAB_HELP explains each tab as bullet points, shown with its color key, if any, on hover over the tab's ? button.
  An entry is a list of points, or a function of the target type (PILOT, CORPORATION, ALLIANCE, or SYSTEM) for a tab that differs by target.
  Each point fits in about two lines of the popup, around 90 characters, and a tab has as few as explain it.
*/
const TAB_HELP = {
  "tab-glance": [
    "An overview of this search: the top few rows of each tab. Open goes to the full tab.",
    "The « button at the top of the sidebar shrinks it to its icons.",
    "Drag the tabs, or use Alt+↑/↓, to reorder them. At a glance stays at the top.",
    "The bar under the target's box pulls it shut, its holdings first if it has any.",
    "Right-click a row for the same menu it has in its full tab.",
  ],
  "tab-killmails": [
    "This search's killmails, newest first. Click one to open it on zKillboard.",
    "How many, and how far back, is set in Preferences.",
    "Right-click the victim or final blow to search them or open them on an external site, or a loss to Show fit.",
  ],
  "tab-activity": [
    "When the killmails happened, by weekday and hour, to show active times.",
    "Wormhole systems also show the last structure destroyed there, however long ago.",
  ],
  "tab-fleetmates": [
    "The 10 pilots most often on killmails with this pilot, or in its corp or alliance now.",
    UNIQUE_DAYS_HELP,
    PILOT_MENU_HELP,
  ],
  "tab-members": [
    "The 10 pilots on the most killmails for this corp or alliance, by who they flew for then.",
    UNIQUE_DAYS_HELP,
    PILOT_MENU_HELP,
  ],
  "tab-pilots": [
    "The 10 pilots on the most killmails in this system.",
    UNIQUE_DAYS_HELP,
    PILOT_MENU_HELP,
  ],
  "tab-allies": [
    "The 10 alliances most on this target's kills, then corps with no alliance.",
    "Holdings are listed above the table.",
    "Left out under the table: ignored allies (× restores one) and, by Preference, allies it also killed.",
    UNIQUE_DAYS_HELP,
    "Right-click an ally to add it as a holding, ignore it, search it, or open it on an external site.",
  ],
  "tab-corps": [
    "The 10 corporations on the most killmails in this system.",
    UNIQUE_DAYS_HELP,
    CORP_MENU_HELP,
  ],
  "tab-systems": [
    "The 10 systems with the most of this target's killmails. Kills and Losses are the target's.",
    UNIQUE_DAYS_HELP,
    "Right-click a system to search it, or open it on an external site.",
  ],
  "tab-pi": [
    "Haulers lost near a planet, and any ship lost with PI aboard. Hover a PI tag to see the cargo.",
    "Right-click a pilot to search them, their corp, or alliance, or a loss to open it on an external site.",
  ],
  "tab-ratters": [
    "Ships lost here with rats on the killmail, not gate guns, CONCORD, or police. Hover a loss to see who was on it.",
    "Right-click a pilot to search them, their corp, or alliance, or a loss to open it on an external site.",
  ],
  "tab-structures": [
    "Used in Kills: this corp's and its holdings' citadels and POSes seen on its kills, at any age.",
    "Destroyed: structures it helped kill on this search's killmails.",
    "Lost: every citadel and POS it or its holdings lost, at any age.",
    "Right-click a structure to search its system, owner, or killers, or open it on an external site.",
  ],
  "tab-fits": [
    "Fits by ship. Losses with the same fit are merged into one, whatever slots its modules were in, and whichever ECM flavors it used.",
    "Non-combat ships (pods, shuttles, haulers, mining ships), structures, and losses with nothing fitted are left out.",
    "A merged fit's losses page with the arrows by its name, or ← →, each showing its own layout, charges, and drones.",
    "Right-click the hull, a module, a drone, or the zKillboard button (which has the capsule too) to open it on an external site.",
    "Tag key, beside the filter, explains each tag this search found. Hover a tag in the fitting window to see what earned it.",
  ],
  "tab-capitals": [
    "Capital ships its pilots flew on these killmails, and who flew them. Click a pilot to search.",
    "A struck-through pilot has since left the alliance.",
    PILOT_MENU_HELP,
  ],
  "tab-roster": (type) => type === "ALLIANCE"
    ? ["Current member corporations, their member counts, and how long they've been in.", CORP_MENU_HELP]
    : [
      "Current members: those on these killmails first, latest first, then the rest from EveWho.",
      "Red underline: EveWho still lists them, but the ESI says they've left.",
      PILOT_MENU_HELP,
    ],
};

function selectTab(tabId) {
  if (tabId !== "tab-fits") closeFitKey();
  const points = typeof TAB_HELP[tabId] === "function" ? TAB_HELP[tabId](roster ? roster.type : "") : TAB_HELP[tabId];
  const key = TAB_COLOR_KEYS[tabId];
  const title = el(tabId).querySelector(".tab-name").textContent;
  el("panel-title").textContent = title;
  shownHelp = points || key ? { title, points: points || [], sections: key || [] } : null;
  el("tab-key").replaceChildren(...(shownHelp ? [tabHelp(shownHelp.points, shownHelp.sections)] : []));
  if (el("help-window").open) renderHelpWindow();

  for (const tab of document.querySelectorAll(".tab")) {
    const selected = tab.id === tabId;
    tab.setAttribute("aria-selected", String(selected));
    el(tab.getAttribute("aria-controls")).hidden = !selected;
  }
}

// formatIsk shortens an isk value to the nearest unit, ie 16572571.24 to 16.57M
function formatIsk(value) {
  const units = [[1e12, "T"], [1e9, "B"], [1e6, "M"], [1e3, "K"]];
  for (const [size, unit] of units) {
    if (value >= size) {
      return `${(value / size).toFixed(2)}${unit} ISK`;
    }
  }
  return `${Math.round(value)} ISK`;
}

// timeParts splits a time into the parts shown, in EVE time (UTC) or local time as chosen in Preferences
function timeParts(isoTime) {
  const date = new Date(isoTime);
  const utc = preferences.timeZone !== "local";
  return {
    year: utc ? date.getUTCFullYear() : date.getFullYear(),
    month: utc ? date.getUTCMonth() : date.getMonth(),
    day: utc ? date.getUTCDate() : date.getDate(),
    hour: utc ? date.getUTCHours() : date.getHours(),
    minute: utc ? date.getUTCMinutes() : date.getMinutes(),
    weekday: utc ? date.getUTCDay() : date.getDay(), // 0 is Sunday
    second: utc ? date.getUTCSeconds() : date.getSeconds(),
  };
}

const pad2 = (value) => String(value).padStart(2, "0");

// clockTime shows the time of day, ie 08:58, adding seconds when asked for
function clockTime(parts, withSeconds = false) {
  return `${pad2(parts.hour)}:${pad2(parts.minute)}${withSeconds ? `:${pad2(parts.second)}` : ""}`;
}

// formatKillmailTime shows a killmail's date and time in the chosen time zone, ie Oct 25th, 2026 · 08:58 EVE
function formatKillmailTime(isoTime) {
  return `${spokenDate(isoTime)} · ${clockTime(timeParts(isoTime))} ${preferences.timeZone === "local" ? "local" : "EVE"}`;
}

/*
  tabHelp builds a tab's round ? button, whose popup shows on hover or keyboard focus: what the tab shows, then what each of its colors means.

  points: the tab's explanation, one bullet point each
  sections: its color key, [{ title, items: [{ sample, className, meaning }] }], empty for a tab without colored data
    - sample: example text shown in the color, ie "0.9" or "B274"
    - className: the css class that colors the sample, the same one the data uses
    - meaning: what the color means
*/
function tabHelp(points, sections) {
  const { help, toggle } = helpPopup("tab-help", "About this tab", helpContent(points, sections));
  toggle.title = "Click to keep this open in its own window";
  toggle.setAttribute("aria-expanded", String(el("help-window").open));
  toggle.addEventListener("click", () => toggleHelpWindow(toggle));
  return help;
}

/*
  helpPopup builds a round ? button with a popup shown on hover or keyboard focus, for tabHelp and filterHelp.

  className: the wrapper's class, the button's being it with -toggle after, ie tab-help and tab-help-toggle
  label: the button's name for screen readers
  parts: the popup's contents
*/
function helpPopup(className, label, parts) {
  const help = document.createElement("div");
  help.className = className;

  const toggle = document.createElement("button");
  toggle.type = "button";
  toggle.className = `${className}-toggle`;
  toggle.textContent = "?";
  toggle.setAttribute("aria-label", label);

  const popup = document.createElement("div");
  popup.className = "tab-help-popup help-content";
  popup.setAttribute("role", "tooltip");
  popup.append(...parts);

  help.append(toggle, popup);
  return { help, toggle };
}

// helpContent is a tab's help, the same in the ? popup and the help window: its bullet points, then a line, then its color key
function helpContent(points, sections) {
  const parts = [];
  if (points.length > 0) {
    const list = document.createElement("ul");
    list.append(...points.map((text) => {
      const point = document.createElement("li");
      point.textContent = text;
      return point;
    }));
    parts.push(list);
  }

  for (const section of sections) {
    const title = document.createElement("h3");
    title.textContent = section.title;

    const list = document.createElement("dl");
    for (const item of section.items) {
      const sample = document.createElement("dt");
      sample.className = item.className;
      sample.textContent = item.sample;
      if (item.onClick) {
        sample.classList.add("fit-tag-filter");
        sample.tabIndex = 0;
        sample.title = item.clickTitle || "";
        sample.addEventListener("click", item.onClick);
        sample.addEventListener("keydown", (event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            item.onClick();
          }
        });
      }
      const meaning = document.createElement("dd");
      meaning.textContent = item.meaning;
      list.append(sample, meaning);
    }

    parts.push(title, list);
  }
  return parts;
}

// shownHelp is the selected tab's help, { title, points, sections }, null for a tab with none. The help window shows it.
let shownHelp = null;
// helpWindowPlace is where the help window was last dragged to, kept for this session, null to open it by the ? button
let helpWindowPlace = null;

function renderHelpWindow() {
  el("help-window-title").textContent = shownHelp ? `${shownHelp.title} help` : "Help";
  el("help-window-body").replaceChildren(...(shownHelp ? helpContent(shownHelp.points, shownHelp.sections) : []));
  if (!shownHelp) {
    const none = document.createElement("p");
    none.className = "muted";
    none.textContent = "This tab has no help.";
    el("help-window-body").append(none);
  }
}

// placeHelpWindow moves the help window, keeping its title bar on screen so it can always be dragged back
function placeHelpWindow(left, top) {
  const box = el("help-window").getBoundingClientRect();
  const x = Math.min(Math.max(left, 0), Math.max(window.innerWidth - box.width, 0));
  const y = Math.min(Math.max(top, 0), Math.max(window.innerHeight - 40, 0));
  el("help-window").style.left = `${x}px`;
  el("help-window").style.top = `${y}px`;
  return { left: x, top: y };
}

function setHelpWindowOpen(open, anchor) {
  const help = el("help-window");
  if (open && !help.open) {
    renderHelpWindow();
    help.show(); // not modal, the app stays usable around it
    if (helpWindowPlace) {
      helpWindowPlace = placeHelpWindow(helpWindowPlace.left, helpWindowPlace.top);
    } else if (anchor) {
      // first time, under the ? button, lined up with its right edge
      const button = anchor.getBoundingClientRect();
      placeHelpWindow(button.right - help.getBoundingClientRect().width, button.bottom + 8);
    }
  } else if (!open && help.open) {
    help.close();
  }
  // while it is open, hovering over the ? does not show the same help again
  document.body.classList.toggle("help-popped", open);
  for (const toggle of document.querySelectorAll(".tab-help-toggle")) toggle.setAttribute("aria-expanded", String(open));
}

function toggleHelpWindow(anchor) {
  setHelpWindowOpen(!el("help-window").open, anchor);
}

// dragging the help window by its title bar
function startHelpDrag(event) {
  if (event.button !== 0 || event.target.closest("button")) return;
  const titlebar = el("help-window-titlebar");
  const box = el("help-window").getBoundingClientRect();
  const offset = { x: event.clientX - box.left, y: event.clientY - box.top };
  titlebar.setPointerCapture(event.pointerId);
  event.preventDefault();

  const move = (moveEvent) => {
    helpWindowPlace = placeHelpWindow(moveEvent.clientX - offset.x, moveEvent.clientY - offset.y);
  };
  const stop = () => {
    titlebar.removeEventListener("pointermove", move);
    titlebar.removeEventListener("pointerup", stop);
    titlebar.removeEventListener("pointercancel", stop);
  };
  titlebar.addEventListener("pointermove", move);
  titlebar.addEventListener("pointerup", stop);
  titlebar.addEventListener("pointercancel", stop);
}

const SECURITY_KEY = {
  title: "Security status",
  items: [
    { sample: "0.9", className: "sec-high", meaning: "Highsec, 0.5 to 1.0" },
    { sample: "0.3", className: "sec-low", meaning: "Lowsec, 0.1 to 0.4" },
    { sample: "-1.0", className: "sec-null", meaning: "Nullsec and J-space, 0.0 and below" },
  ],
};

const OUTCOME_KEY = {
  title: "Card edge and tag",
  items: [
    { sample: "KILL", className: "key-kill", meaning: "The target was an attacker" },
    { sample: "LOSS", className: "key-loss", meaning: "The target, or its corp or alliance, was the victim" },
    { sample: "—", className: "key-neutral", meaning: "System search, the system has no side" },
  ],
};

const WORMHOLE_CLASS_KEY = {
  title: "Wormhole class",
  items: [
    { sample: "C3", className: "space space-c1-3", meaning: "C1 to C3" },
    { sample: "C5", className: "space space-c4-5", meaning: "C4 and C5" },
    { sample: "C6", className: "space space-c6", meaning: "C6" },
  ],
};

const WORMHOLE_KEY = {
  title: "Wormhole class and statics",
  items: [
    { sample: "B274", className: "space space-hs", meaning: "Static to Highsec" },
    { sample: "A239", className: "space space-ls", meaning: "Static to Lowsec" },
    { sample: "K346", className: "space space-ns", meaning: "Static to Nullsec" },
    { sample: "C3 · O477", className: "space space-c1-3", meaning: "C1 to C3, or a static to one" },
    { sample: "C5 · H296", className: "space space-c4-5", meaning: "C4 and C5, or a static to one" },
    { sample: "C6 · R474", className: "space space-c6", meaning: "C6, or a static to one" },
  ],
};

const STRUCTURE_STATE_KEY = {
  title: "Status",
  items: [
    { sample: "✕", className: "structure-state structure-state-destroyed", meaning: "Destroyed since its last kill, grayed out with who destroyed it" },
    { sample: "✓", className: "structure-state structure-state-standing", meaning: "Still standing, as far as zKillboard shows" },
  ],
};

// RATTER_KEY is the Ratters tab's loss tags, from RAT_KINDS. Pilots rank by their surest ratting loss, NPC first.
const RATTER_KEY = {
  title: "Loss tag, surest ratting first",
  items: Object.entries(RAT_KINDS).map(([kind, [label, meaning]]) => ({ sample: label, className: `pi-tag rat-kind-${kind}`, meaning })),
};

// FIT_KEY is the Fits tab's tags, on a fit in the list and, for implants, on the loss named under the window's title
const FIT_KEY = {
  title: "Tags",
  // the rules' tags this search found, so the key changes with each search
  get items() {
    const shown = [...fitsShown.ruleTags.filter((tag) => !tag.hidden), ...FIT_TAGS];
    return shown.map((tag) => ({
      sample: tag.name, className: tag.className, meaning: tag.meaning,
      onClick: () => filterFitsByTag(tag), clickTitle: `List only the fits tagged ${tag.name}`,
    }));
  },
};

// fitKeyCollapsedBefore is whether the sidebar was shrunk to its icons before the tag key opened, restored when it closes; null while it is shut
let fitKeyCollapsedBefore = null;

// openFitKey opens the Fits tab's tag key on the left, shrinking the sidebar to its icons for the room, without saving that as the sidebar's choice
function openFitKey() {
  if (fitKeyCollapsedBefore === null) fitKeyCollapsedBefore = sidebarCollapsed;
  sidebarCollapsed = true;
  showSidebar();
  fillFitKey();
  el("fit-key-panel").hidden = false;
  el("intel-tabs").classList.add("fit-key-open");
  el("fit-key-open").setAttribute("aria-expanded", "true");
}

// fillFitKey writes the tags this search found into the open tag key
function fillFitKey() {
  el("fit-key-body").replaceChildren(...helpContent([], [FIT_KEY]));
}

// closeFitKey shuts the tag key and gives the sidebar back as it was
function closeFitKey() {
  if (fitKeyCollapsedBefore === null) return;
  el("fit-key-panel").hidden = true;
  el("intel-tabs").classList.remove("fit-key-open");
  el("fit-key-open").setAttribute("aria-expanded", "false");
  sidebarCollapsed = fitKeyCollapsedBefore;
  fitKeyCollapsedBefore = null;
  showSidebar();
}

// TAB_COLOR_KEYS is the color key shown for each tab, tabs without colored data have none
const TAB_COLOR_KEYS = {
  "tab-ratters": [RATTER_KEY],
  "tab-structures": [STRUCTURE_STATE_KEY],
  "tab-killmails": [OUTCOME_KEY, SECURITY_KEY, WORMHOLE_CLASS_KEY],
  "tab-systems": [SECURITY_KEY, WORMHOLE_KEY],
};

// securityClass colors a security status by its band, high, low, or null, which the backend decides (api.SecurityBand)
function securityClass(band) {
  return band ? `sec-${band}` : "";
}

// cardHeading builds one of a killmail card's bold headings, which share its top line
function cardHeading(text, className) {
  const heading = document.createElement("strong");
  heading.className = `killmail-heading ${className}`;
  heading.textContent = text;
  return heading;
}

// column builds a killmail card column of muted lines, below its heading
function column(lines, className = "") {
  const col = document.createElement("div");
  col.className = `killmail-col ${className}`;
  for (const line of lines) {
    if (line === null) continue;
    const item = document.createElement("span");
    if (line instanceof Node) {
      item.append(line);
    } else {
      item.textContent = line;
    }
    col.append(item);
  }
  return col;
}

// LABEL_TEXT reads zKillboard's killmail tags in plain words. Tags with a prefix (cat:, #:, fw:) are handled in labelText, anything unknown is shown as it is.
const LABEL_TEXT = {
  pvp: "PvP",
  npc: "NPC",
  solo: "Solo",
  awox: "Awox",
  padding: "Padding",
  capital: "Capital",
  capinv: "Capitals involved",
  super: "Supercapital",
  "loc:highsec": "Highsec",
  "loc:lowsec": "Lowsec",
  "loc:nullsec": "Nullsec",
  "loc:w-space": "W-space",
  "loc:abyssal": "Abyssal",
  "loc:pochven": "Pochven",
  "loc:drifter": "Drifter space",
  "tz:au": "AU TZ",
  "tz:eu": "EU TZ",
  "tz:ru": "RU TZ",
  "tz:use": "US East TZ",
  "tz:usw": "US West TZ",
  "isk:under1b": "Under 1b ISK",
  "isk:1b+": "1b+ ISK",
  "isk:5b+": "5b+ ISK",
  "isk:10b+": "10b+ ISK",
  "isk:100b+": "100b+ ISK",
  "isk:1t+": "1t+ ISK",
};

// FACTION_WARZONES names the faction warfare tags, ie fw:calgal
const FACTION_WARZONES = { amarr: "Amarr", caldari: "Caldari", gallente: "Gallente", minmatar: "Minmatar", amamin: "Amarr/Minmatar", calgal: "Caldari/Gallente" };

function labelText(label) {
  if (LABEL_TEXT[label]) return LABEL_TEXT[label];

  const [prefix, value] = label.split(":");
  // the backend looks up category names from the ESI, see IntelReport.LabelNames
  if (prefix === "cat") return (intelReport && intelReport.labelNames && intelReport.labelNames[label]) || `Category ${value}`;
  if (prefix === "#") return value === "1" ? "1 pilot" : `${value} pilots`;
  if (prefix === "fw") return `FW ${FACTION_WARZONES[value] || value}`;
  return label;
}

// labelChip shows one zKillboard tag as a small chip, with the raw tag on hover
function labelChip(label) {
  const chip = document.createElement("span");
  chip.className = "label-chip";
  chip.textContent = labelText(label);
  chip.title = `zKillboard tag: ${label}`;
  return chip;
}

// showKillmailLabels is the Preferences choice to show zKillboard's label chips on killmail cards. The labels are always collected.
let showKillmailLabels = false;

// groupLine shows a corp and alliance, backend EntityRefs, as 16px logos beside their names, ie [logo] URSA PRIMUS · [logo] URSA., or null if there is neither
function groupLine(...groups) {
  const named = groups.filter((group) => group && group.name);
  if (named.length === 0) return null;

  const line = document.createElement("span");
  line.className = "group-line";
  named.forEach((group, index) => {
    if (index > 0) line.append(" · ");
    if (group.iconUrl) {
      const logo = document.createElement("img");
      logo.src = group.iconUrl;
      logo.alt = "";
      line.append(logo);
    }
    line.append(group.name);
  });
  return line;
}

/*
  killmailCardSections are a killmail card's right click menu for the part of it right-clicked, clicked: always the killmail, then on the victim's column
  their pilot, corp, and alliance, on the attackers' column the final blow's the same, and anywhere else the system.
  A structure victim or NPC final blow has no pilot section, only the corp and alliance it was in.
*/
function killmailCardSections(km, clicked) {
  const on = (part) => Boolean(clicked && clicked.closest && clicked.closest(`.${part}`));
  const sections = [killmailSection(km.url)];
  if (hasFit(km.id)) sections[0].items.push({ label: "Show fit", action: () => showFit(km.id) });
  if (on("killmail-victim")) {
    if (km.victim) sections.push(refSection(km.victim, `Victim: ${km.victim.name}`));
    sections.push(...groupSections(km.victimCorp, km.victimAlliance, "Victim"));
  } else if (on("killmail-attackers")) {
    if (km.finalBlow) sections.push(refSection(km.finalBlow, `Final blow: ${km.finalBlow.name}`));
    sections.push(...groupSections(km.finalBlowCorp, km.finalBlowAlliance, "Final blow"));
  } else if (km.systemName) {
    sections.push(systemSection({ id: km.systemId, name: km.systemName, zkillUrl: km.systemZkillUrl, dotlanUrl: km.systemDotlanUrl }));
  }
  return sections;
}

function renderKillmailCard(km) {
  const card = document.createElement("li");
  card.className = `killmail ${km.outcome}`.trim();
  card.tabIndex = 0;
  card.title = "Open on zKillboard";

  const ship = document.createElement("img");
  ship.src = km.shipImageUrl;
  ship.alt = "";

  const victimGroup = groupLine(km.victimCorp, km.victimAlliance);
  const finalBlowGroup = groupLine(km.finalBlowCorp, km.finalBlowAlliance);

  const location = document.createElement("span");
  const security = document.createElement("span");
  security.className = securityClass(km.securityBand);
  security.textContent = km.securityStatus.toFixed(1);
  location.append(`${km.systemName} `, security);

  const finalBlow = km.finalBlowName ? `Final blow: ${km.finalBlowName}` : null;
  const attackers = km.solo ? "Solo" : `${km.attackerCount} attackers`;

  // a system target has no side, so its killmails have no outcome tag
  let outcome = null;
  if (km.outcome) {
    outcome = document.createElement("span");
    outcome.className = "outcome";
    outcome.textContent = km.outcome.toUpperCase();
    if (km.outcome === "internal") outcome.title = "Every attacker and the victim are this target or its holdings";
  }

  // the date goes in the value column directly, not wrapped like its other lines, so it can be its own small label
  const time = document.createElement("span");
  time.className = "killmail-time";
  time.textContent = formatKillmailTime(km.time);
  const value = column([`Dropped ${formatIsk(km.droppedValue || 0)}`, outcome], "killmail-value");
  value.append(time);

  // a killmail collected from one of the target's holdings says which, at the foot of the victim's corp and alliance column on the date's line,
  // as that column's width is fixed and the value column's is not
  const victimLines = column([victimGroup], "killmail-victim");
  if (km.via) {
    const via = document.createElement("span");
    via.className = "killmail-via";
    via.textContent = `via ${km.via}`;
    via.title = via.textContent;
    victimLines.append(via);
  }

  // the headings share the top line, with the ship image and each heading's details on the line below
  card.append(
    cardHeading(km.victimName, "killmail-victim"),
    cardHeading(km.shipName, "killmail-ship"),
    cardHeading(attackers, "killmail-attackers"),
    cardHeading(formatIsk(km.totalValue), "killmail-total"),
    ship,
    victimLines,
    column([location, km.class ? spaceLabel(km.class) : km.region || null], "killmail-ship"),
    column([finalBlow, finalBlowGroup], "killmail-attackers"),
    value,
  );

  if (showKillmailLabels && km.labels && km.labels.length > 0) {
    const tags = document.createElement("div");
    tags.className = "killmail-tags";
    tags.append(...km.labels.map(labelChip));
    card.append(tags);
  }

  withMenu(card, (event) => killmailCardSections(km, event.target));
  const open = () => window.runtime.BrowserOpenURL(km.url);
  card.addEventListener("click", open);
  card.addEventListener("keydown", (event) => {
    if (event.key === "Enter") open();
  });

  return card;
}

// PAGE_LINKS is how many page numbers a pager lists, centered on the current page, once there are more pages than that
const PAGE_LINKS = 5;

/*
  renderPager fills a pager's middle with "Page 3 of 4", or past PAGE_LINKS pages, links to the five pages around the current one, which is highlighted.
  Past PAGE_LINKS pages it also offers the first and last page buttons outside Previous and Next, each only while its page is not among the links.
  label: the element between the Previous and Next buttons. current is from 0. onSelect is called with the page chosen, from 0.
  jumps: { first, last }, the pager's first and last page buttons, whose clicks are wired once by wirePagerJumps.
*/
function renderPager(label, current, pageCount, onSelect, jumps) {
  jumps.first.hidden = jumps.last.hidden = pageCount <= PAGE_LINKS;
  if (pageCount <= PAGE_LINKS) {
    label.replaceChildren(`Page ${current + 1} of ${pageCount}`);
    return;
  }

  const first = Math.min(Math.max(current - Math.floor(PAGE_LINKS / 2), 0), pageCount - PAGE_LINKS);
  // kept in place but unseen while their page is already a link, so Previous and Next do not move
  jumps.first.classList.toggle("page-jump-off", first === 0);
  jumps.last.classList.toggle("page-jump-off", first + PAGE_LINKS >= pageCount);
  jumps.first.disabled = first === 0;
  jumps.last.disabled = first + PAGE_LINKS >= pageCount;
  const links = [];
  for (let page = first; page < first + PAGE_LINKS; page++) {
    const link = document.createElement("button");
    link.type = "button";
    link.className = page === current ? "page-link page-current" : "page-link";
    link.textContent = (page + 1).toLocaleString("en-US");
    if (page === current) {
      link.setAttribute("aria-current", "page");
    } else {
      link.title = `Go to page ${page + 1}`;
      link.addEventListener("click", () => onSelect(page));
    }
    links.push(link);
  }

  const of = document.createElement("span");
  of.className = "page-of";
  of.textContent = `of ${pageCount.toLocaleString("en-US")}`;
  label.replaceChildren(...links, of);
}

// wirePagerJumps makes a pager's first and last page buttons go to page 0 and the last page, which pageCount gives when clicked
function wirePagerJumps(first, last, pageCount, onSelect) {
  first.addEventListener("click", () => onSelect(0));
  last.addEventListener("click", () => onSelect(pageCount() - 1));
}

function renderKillmailPage() {
  const pageCount = Math.ceil(killmails.length / KILLMAILS_PER_PAGE);
  const start = killmailPage * KILLMAILS_PER_PAGE;

  el("killmail-list").replaceChildren(...killmails.slice(start, start + KILLMAILS_PER_PAGE).map(renderKillmailCard));
  el("killmails-empty").hidden = killmails.length > 0;

  el("killmail-pager").hidden = pageCount <= 1;
  renderPager(el("page-label"), killmailPage, pageCount, showKillmailPage, { first: el("page-first"), last: el("page-last") });
  el("page-prev").disabled = killmailPage === 0;
  el("page-next").disabled = killmailPage >= pageCount - 1;
}

function changeKillmailPage(step) {
  showKillmailPage(killmailPage + step);
}

function showKillmailPage(page) {
  killmailPage = page;
  renderKillmailPage();
}

/*
  renderPilotRows fills a ranked pilot table, such as Top Fleetmates or Top Members

  section: the id prefix of the table body and empty message, ie "fleetmates" for #fleetmates-body and #fleetmates-empty
  pilots: the ranked pilots from the intel report
*/
function renderPilotRows(section, pilots) {
  const body = el(`${section}-body`);
  body.replaceChildren();

  for (const mate of pilots) {
    const row = document.createElement("tr");

    const nameCell = document.createElement("td");
    const name = document.createElement("div");
    name.textContent = mate.name;
    const detail = document.createElement("div");
    detail.className = "row-detail";
    const group = groupLine(mate.corporation, mate.alliance);
    detail.append(...(group ? [group, " · "] : []), killmailSpan(mate.firstTogether, mate.lastTogether));
    nameCell.append(name, detail);

    const killmailsCell = document.createElement("td");
    killmailsCell.className = "num";
    killmailsCell.textContent = mate.killmailsShared;

    row.append(avatarCell(mate.name, mate, mate.imageUrl), nameCell, killmailsCell, uniqueDaysCell(mate.uniqueDays, mate.firstTogether, mate.lastTogether));
    body.append(withMenu(row, pilotSections(mate)));
  }

  el(`${section}-empty`).hidden = pilots.length > 0;
}

function renderCorporations(corporations) {
  const rows = corporations.map((corporation) => {
    const row = document.createElement("tr");

    const nameCell = document.createElement("td");
    const name = document.createElement("div");
    name.textContent = corporation.name;
    const detail = document.createElement("div");
    detail.className = "row-detail";
    const alliance = groupLine(corporation.alliance);
    detail.append(...(alliance ? [alliance, " · "] : []), killmailSpan(corporation.firstSeen, corporation.lastSeen));
    nameCell.append(name, detail);

    row.append(avatarCell(corporation.name, corporation, corporation.imageUrl), nameCell, numberCell(corporation.killmails), numberCell(corporation.kills), numberCell(corporation.losses), numberCell(corporation.pilots), uniqueDaysCell(corporation.uniqueDays, corporation.firstSeen, corporation.lastSeen));
    return withMenu(row, corpSections(corporation));
  });

  el("corps-body").replaceChildren(...rows);
  el("corps-empty").hidden = corporations.length > 0;
}

// entityChip is a pilot, corp, or alliance's logo and name. ref is a backend EntityRef, null gives null. Its row's structureMenu covers it.
function entityChip(ref) {
  if (!ref) return null;
  return groupLine(ref);
}

// ownerLine is a structure's owning corp and alliance
function ownerLine(owner, alliance) {
  const line = document.createElement("span");
  line.className = "structure-owner";
  [entityChip(owner), entityChip(alliance)].filter(Boolean).forEach((chip, index) => {
    if (index > 0) line.append(" · ");
    line.append(chip);
  });
  return line;
}

// structureSystemLine is a structure's system, with its security, class, and statics
function structureSystemLine(system) {
  const line = document.createElement("span");
  const name = document.createElement("span");
  const security = document.createElement("span");
  security.className = securityClass(system.securityBand);
  security.textContent = (system.securityStatus || 0).toFixed(1);
  name.append(`${system.name} `, security);
  line.append(name);
  for (const part of systemLocation(system)) line.append(" · ", part);
  return line;
}

// deathLine says who destroyed a structure, with how many attackers, and how long ago, ie "Killed by Hard Knocks Citizens (12) · 3 days ago, Oct 23rd, 2026"
function deathLine(death) {
  const line = document.createElement("span");
  line.className = "structure-death";
  const killer = entityChip(death.killerAlliance) || entityChip(death.killerCorp);
  line.append("Killed by ");
  if (killer) line.append(killer, " ");
  line.append(`(${death.attackers})`);
  const when = document.createElement("span");
  when.textContent = ` · ${daysAgo(death.time)}, ${spokenDate(death.time)}`;
  line.append(when);
  return line;
}

/*
  renderStructureLoss draws the box above a j-space system's Activity: its newest upwell structure or control tower lost, however long ago,
  with its render, owner, killers, and when. jspace false hides it, a j-space system with none says so.
  previous are the structures lost besides it, minor ones such as customs offices too, newest first, in small rows under it, collapsed until opened.
*/
function renderStructureLoss(jspace, loss, previous) {
  const box = el("structure-loss");
  box.hidden = !jspace;
  box.oncontextmenu = null;
  if (!jspace) return;

  const heading = document.createElement("h3");
  heading.textContent = "Last structure destroyed";
  const earlier = previousStructureLosses(previous);
  if (!loss) {
    const none = document.createElement("p");
    none.className = "muted";
    none.textContent = "zKillboard lists no upwell structure or control tower lost in this system.";
    const details = document.createElement("div");
    details.className = "structure-loss-details";
    details.append(heading, none, ...(earlier ? [earlier] : []));
    box.replaceChildren(details);
    return;
  }

  const image = document.createElement("img");
  image.className = "structure-loss-render";
  image.src = loss.renderUrl;
  image.alt = "";
  const details = document.createElement("div");
  details.className = "structure-loss-details";
  const type = document.createElement("div");
  type.className = "structure-loss-type";
  type.textContent = loss.moon ? `${loss.type} · ${loss.moon}` : loss.type;
  const owner = document.createElement("div");
  owner.append("Owner: ", ownerLine(loss.owner, loss.ownerAlliance));
  const death = document.createElement("div");
  death.append(deathLine(loss.death));
  details.append(heading, type, owner, death, ...(earlier ? [earlier] : []));
  const structure = document.createElement("div");
  structure.className = "structure-loss-structure";
  structure.append(siteLinks(loss.type, { zkillUrl: loss.death.zkillUrl }, true), image);
  box.replaceChildren(structure, details);
  // set rather than added, as the box is kept from search to search. The previous rows have their own menus.
  const sections = structureLossSections(loss, false);
  box.oncontextmenu = (event) => openSectionsMenu(event, sections);
}

/*
  structureLossSections are the right click menu sections of a destroyed structure: the structure, with only the zKillboard link to its loss,
  then its system when withSystem, its owner, and its killer, each searchable and with its site links
*/
function structureLossSections(loss, withSystem) {
  return [
    { heading: `Structure: ${loss.type}`, items: [{ label: "Open its loss on zKillboard", action: () => window.runtime.BrowserOpenURL(loss.death.zkillUrl) }] },
    ...(withSystem ? [systemSection(loss.system)] : []),
    ...groupSections(loss.owner, loss.ownerAlliance, "Owner"),
    ...killerSections(loss.death),
  ];
}

function systemSection(system) {
  return entitySection({ type: "SYSTEM", id: system.id, name: system.name, zkillUrl: system.zkillUrl, dotlanUrl: system.dotlanUrl });
}

// entitySection is an entity's menu section, under a heading naming its kind, ie "Corporation: URSA PRIMUS"
function entitySection(entity) {
  return { heading: `${searchTypeLabel(entity.type)}: ${entity.name}`, entity };
}

// killmailSection is the menu section that opens a killmail, or a PI or ratting loss, on zKillboard
function killmailSection(url) {
  return { heading: "Killmail", items: [{ label: "Open on zKillboard", action: () => window.runtime.BrowserOpenURL(url) }] };
}

// pilotSections are a pilot row's menu sections: the pilot, then their corp and alliance for those they have
function pilotSections(pilot) {
  return [
    entitySection({ type: "PILOT", id: pilot.id, name: pilot.name, zkillUrl: pilot.zkillUrl, evewhoUrl: pilot.evewhoUrl }),
    ...groupSections(pilot.corporation, pilot.alliance),
  ];
}

// corpSections are a corp row's menu, ie in Top Corps: the corp, then its alliance
function corpSections(corp) {
  return [
    entitySection({ type: "CORPORATION", id: corp.id, name: corp.name, zkillUrl: corp.zkillUrl, evewhoUrl: corp.evewhoUrl }),
    ...groupSections(null, corp.alliance),
  ];
}

// refSection is a menu section for a backend EntityRef, under the heading given
function refSection(ref, heading) {
  return { heading, entity: { type: ref.type, id: ref.id, name: ref.name, zkillUrl: ref.zkillUrl, evewhoUrl: ref.evewhoUrl } };
}

// groupSections are a corp's and alliance's menu sections, ie "Owner corp: Some Corp" and "Owner alliance: Some Alliance", or without a role "Corporation: Some Corp", for those there are
function groupSections(corp, alliance, role = "") {
  return [
    ...(corp ? [refSection(corp, `${role ? `${role} corp` : "Corporation"}: ${corp.name}`)] : []),
    ...(alliance ? [refSection(alliance, `${role ? `${role} alliance` : "Alliance"}: ${alliance.name}`)] : []),
  ];
}

// killerSections is the menu section of the group that destroyed a structure, the one deathLine names
function killerSections(death) {
  const killer = death.killerAlliance || death.killerCorp;
  return killer ? [refSection(killer, `Killed by: ${killer.name}`)] : [];
}

// previousStructureLosses is the collapsed list of other structures lost in the system, a small row each, or null for none
function previousStructureLosses(previous) {
  if (previous.length === 0) return null;
  const list = document.createElement("details");
  list.className = "structure-previous";
  const summary = document.createElement("summary");
  summary.textContent = `${previous.length} previous ${previous.length === 1 ? "structure" : "structures"} destroyed`;
  // the rows share one grid, so the types line up whatever their length
  const rows = document.createElement("div");
  rows.className = "structure-previous-rows";
  list.append(summary, rows);
  for (const loss of previous) {
    const row = document.createElement("div");
    row.className = "structure-previous-row";
    const icon = document.createElement("img");
    icon.className = "structure-previous-icon";
    icon.src = loss.imageUrl;
    icon.alt = "";
    const type = document.createElement("span");
    type.className = "structure-previous-type";
    type.textContent = loss.moon ? `${loss.type} · ${loss.moon}` : loss.type;
    const who = document.createElement("span");
    who.className = "structure-previous-who";
    const owner = document.createElement("div");
    owner.append(ownerLine(loss.owner, loss.ownerAlliance));
    const death = document.createElement("div");
    death.append(deathLine(loss.death));
    who.append(owner, death);
    row.append(siteLinks(loss.type, { zkillUrl: loss.death.zkillUrl }), icon, type, who);
    rows.append(withMenu(row, structureLossSections(loss, false)));
  }
  return list;
}

// structuresSubtab is the Structures tab's shown list, kept across searches like the tab itself
let structuresSubtab = "subtab-structures-used";

function selectStructuresSubtab(id) {
  structuresSubtab = id;
  for (const subtab of document.querySelectorAll("#panel-structures .subtab")) {
    const selected = subtab.id === id;
    subtab.setAttribute("aria-selected", String(selected));
    el(subtab.getAttribute("aria-controls")).hidden = !selected;
  }
}

// structureState is the Status column's mark: red ✕ for a structure lost since its last kill, green ✓ for one standing as far as zKillboard shows
function structureState(destroyed) {
  const mark = document.createElement("span");
  mark.className = destroyed ? "structure-state structure-state-destroyed" : "structure-state structure-state-standing";
  mark.textContent = destroyed ? "✕" : "✓";
  mark.title = destroyed ? "Destroyed since its last kill" : "Standing, as far as zKillboard shows";
  return mark;
}

// structureNameCell is a structure row's type, system, and any moon, battery, or holding it was seen through
function structureNameCell(type, system, extras) {
  const cell = document.createElement("td");
  const name = document.createElement("div");
  name.textContent = type;
  const detail = document.createElement("div");
  detail.className = "row-detail";
  detail.append(structureSystemLine(system));
  for (const extra of extras.filter(Boolean)) detail.append(" · ", extra);
  cell.append(name, detail);
  return cell;
}

/*
  renderStructures draws the Structures tab of a corp or alliance: Used in Kills, its structures seen on its kills, grayed once destroyed since,
  Destroyed, the structures it took part in destroying, and Lost, every structure it lost. A row's right click menu is its owner's, its system and killers have their own.
*/
function renderStructures(structures) {
  const used = structures.used.map((structure) => {
    const row = document.createElement("tr");
    if (structure.destroyed) row.className = "structure-destroyed";
    const via = structure.via ? `via ${structure.via}` : null;
    const battery = structure.battery ? `seen through its ${structure.battery}` : null;
    const ownerCell = document.createElement("td");
    ownerCell.append(ownerLine(structure.owner, structure.ownerAlliance));
    const seen = document.createElement("div");
    seen.className = "row-detail";
    seen.textContent = killmailSpan(structure.firstSeen, structure.lastSeen);
    ownerCell.append(seen);
    const deathCell = document.createElement("td");
    deathCell.className = "structure-state-cell";
    // a table cell can not be a flex box itself, so the mark and details share a wrapper
    const stateLine = document.createElement("div");
    stateLine.className = "structure-state-line";
    stateLine.append(structureState(Boolean(structure.destroyed)));
    if (structure.destroyed) {
      const death = structure.destroyed;
      const killer = death.killerAlliance || death.killerCorp;
      const details = document.createElement("div");
      details.className = "structure-state-death";
      details.textContent = `Killed${killer ? ` by ${killer.name}` : ""} (${death.attackers}) · ${daysAgo(death.time)}, ${spokenDate(death.time)}`;
      stateLine.append(details);
    }
    deathCell.append(stateLine);
    const avatar = avatarCell(structure.type, { zkillUrl: structure.zkillUrl, dotlanUrl: structure.system.dotlanUrl }, structure.imageUrl);
    // the ship it killed on its newest kill, to the right of the structure
    if (structure.killedShipImageUrl) {
      const killed = document.createElement("img");
      killed.src = structure.killedShipImageUrl;
      killed.alt = "";
      killed.title = `Killed a ${structure.killedShip}`;
      avatar.firstChild.append(killed);
    }
    row.append(avatar,
      structureNameCell(structure.type, structure.system, [structure.moon, battery, via]), ownerCell, numberCell(structure.kills), deathCell);
    // a row is every kill of one structure, so with several it opens the latest
    const structureItems = [{ label: structure.kills > 1 ? "Open latest killmail" : "Open this killmail", action: () => window.runtime.BrowserOpenURL(structure.zkillUrl) }];
    if (structure.destroyed) structureItems.push({ label: "Open its loss on zKillboard", action: () => window.runtime.BrowserOpenURL(structure.destroyed.zkillUrl) });
    return withMenu(row, [
      { heading: `Structure: ${structure.type}`, items: structureItems },
      systemSection(structure.system),
      ...groupSections(structure.owner, structure.ownerAlliance, "Owner"),
      ...(structure.destroyed ? killerSections(structure.destroyed) : []),
    ]);
  });
  el("structures-used-body").replaceChildren(...used);
  el("structures-used-empty").hidden = used.length > 0;
  el("structures-used-count").textContent = `(${used.length})`;

  const destroyed = structures.destroyed.map((loss) => {
    const row = document.createElement("tr");
    const ownerCell = document.createElement("td");
    ownerCell.append(ownerLine(loss.owner, loss.ownerAlliance));
    const deathCell = document.createElement("td");
    deathCell.append(deathLine(loss.death));
    row.append(avatarCell(loss.type, { zkillUrl: loss.death.zkillUrl, dotlanUrl: loss.system.dotlanUrl }, loss.imageUrl),
      structureNameCell(loss.type, loss.system, [loss.moon]), ownerCell, deathCell, numberCell(loss.pilots));
    return withMenu(row, structureLossSections(loss, true));
  });
  el("structures-destroyed-body").replaceChildren(...destroyed);
  el("structures-destroyed-empty").hidden = destroyed.length > 0;
  el("structures-destroyed-count").textContent = `(${destroyed.length})`;

  const lost = structures.lost.map((loss) => {
    const row = document.createElement("tr");
    const ownerCell = document.createElement("td");
    ownerCell.append(ownerLine(loss.owner, loss.ownerAlliance));
    const deathCell = document.createElement("td");
    deathCell.append(deathLine(loss.death));
    row.append(avatarCell(loss.type, { zkillUrl: loss.death.zkillUrl, dotlanUrl: loss.system.dotlanUrl }, loss.imageUrl),
      structureNameCell(loss.type, loss.system, [loss.moon, loss.via ? `via ${loss.via}` : null]), ownerCell, deathCell);
    return withMenu(row, structureLossSections(loss, true));
  });
  el("structures-lost-body").replaceChildren(...lost);
  el("structures-lost-empty").hidden = lost.length > 0;
  el("structures-lost-count").textContent = `(${lost.length})`;
  selectStructuresSubtab(structuresSubtab);
}

// intelTarget is the target whose report is shown, as { id, type }, for actions that save something for it, ie its ignored allies
let intelTarget = null;

// allyKey names an ally, ie ALLIANCE:99000001
const allyKey = (ally) => `${ally.type}:${ally.id}`;
// ignoredAllies are the target's ignored allies as last saved
let ignoredAllies = [];
// showingKilledAllies is true once the user chose to show, for the shown search only, the allies the Hide killed allies preference left out
let showingKilledAllies = false;

/*
  renderAllies lists the alliances, and corps in no alliance, most often on the target's kills outside its own alliance. Clicking a name offers a search for it.
  Right-clicking a row opens its menu, to ignore it for the target or open it. The allies already ignored are listed under the table, with the kills each shared,
  most first, each with an x to bring it back. First among them, while the Hide killed allies preference is on and the target killed any, a toggle hides or shows those for this search.
*/
function renderAllies(allies, ignored, killedAllies, holdings) {
  ignoredAllies = ignored.map((ally) => ({ type: ally.type, id: ally.id, name: ally.name }));
  renderAllyHoldings(holdings);

  const rows = allies.map((ally) => {
    const row = document.createElement("tr");
    row.className = "ally-row";
    // built as it opens, since adding a holding depends on the report shown then
    withMenu(row, () => allySections(ally));

    const nameCell = document.createElement("td");
    const name = document.createElement("button");
    name.type = "button";
    name.className = "link-button";
    name.textContent = ally.name;
    name.title = "Search for this, or right-click for more";
    name.addEventListener("click", () => offerSearch({ type: ally.type, name: ally.name, id: ally.id }));
    const detail = document.createElement("div");
    detail.className = "row-detail";
    detail.append(`${searchTypeLabel(ally.type)} · `, killmailSpan(ally.firstSeen, ally.lastSeen));
    nameCell.append(name, detail);
    // shot alongside the target, and shot by it too
    if (ally.killed) {
      const killed = document.createElement("div");
      killed.className = "row-detail ally-killed";
      killed.textContent = `Also a victim of the target ${ally.killed} ${ally.killed === 1 ? "time" : "times"}`;
      nameCell.append(killed);
    }

    row.append(avatarCell(ally.name, ally, ally.imageUrl), nameCell, numberCell(ally.killmails), numberCell(ally.pilots), uniqueDaysCell(ally.uniqueDays, ally.firstSeen, ally.lastSeen));
    return row;
  });

  el("allies-body").replaceChildren(...rows);
  el("allies-empty").hidden = allies.length > 0;
  el("allies-hint").hidden = allies.length === 0;

  const tags = ignored.map((ally) => leftOutTag(`${ally.name} (${ally.killmails || 0})`, "Stop ignoring", `Stop ignoring ${ally.name} for this target`,
    () => saveIgnoredAllies(ignoredAllies.filter((other) => allyKey(other) !== allyKey(ally)), `No longer ignoring ${ally.name} for this target`)));
  if (preferences.hideKilledAllies && killedAllies > 0) tags.unshift(killedAlliesToggle(killedAllies));
  el("allies-ignored-list").replaceChildren(...tags);
  el("allies-ignored").hidden = tags.length === 0;
}

// renderAllyHoldings fills the line at the top of a corp or alliance's Allies: its holdings, with the kills each shared with it, most first. Each has the right click menu.
function renderAllyHoldings(holdings) {
  const parts = [];
  holdings.forEach((holding, index) => {
    if (index > 0) parts.push(" · ");
    const item = document.createElement("span");
    item.className = "allies-holding";
    const name = document.createElement("button");
    name.type = "button";
    name.className = "link-button";
    name.textContent = holding.name;
    name.title = "Search for this, or right-click for more";
    name.addEventListener("click", () => offerSearch({ type: holding.type, name: holding.name, id: holding.id }));
    const count = document.createElement("span");
    count.className = "row-detail";
    count.textContent = ` (${holding.killmails})`;
    count.title = "Kills with pilots of both the target and this holding on them";
    item.append(name, count);
    parts.push(withMenu(item, [entitySection(holding)]));
  });
  el("allies-holdings-list").replaceChildren(...parts);
  el("allies-holdings").hidden = holdings.length === 0;
}

// killedAlliesToggle is the tag that hides, as the preference says, or for this search shows, the allies the target also killed
function killedAlliesToggle(count) {
  const toggle = document.createElement("button");
  toggle.type = "button";
  toggle.className = "allies-ignored-item allies-killed-tag";
  toggle.setAttribute("aria-pressed", String(!showingKilledAllies));
  const state = document.createElement("span");
  state.className = "allies-killed-state";
  state.textContent = showingKilledAllies ? "shown" : "hidden";
  // groups that flew with the target but also lost a pilot to it
  toggle.append(`“Fair-Weather Friends” (${count})`, state);
  toggle.title = showingKilledAllies
    ? "Groups that flew with the target but also had a pilot killed by it, shown for this search. Click to hide them again, as the Hide allies the target also killed preference does."
    : "Groups that flew with the target but also had a pilot killed by it, hidden by the Hide allies the target also killed preference. Click to show them for this search.";
  toggle.addEventListener("click", () => rankAlliesAgain(!showingKilledAllies));
  return toggle;
}

// leftOutTag is a tag under the Allies table for something left out of them, with an x that brings it back
function leftOutTag(text, removeTitle, removeLabel, remove) {
  const item = document.createElement("span");
  item.className = "allies-ignored-item";
  const button = document.createElement("button");
  button.type = "button";
  button.className = "allies-ignored-remove";
  button.textContent = "×";
  button.title = removeTitle;
  button.setAttribute("aria-label", removeLabel);
  button.addEventListener("click", remove);
  item.append(text, button);
  return item;
}

// entityMenu is the open right click menu: the target shown when it opened, and its items. null while it is closed.
let entityMenu = null;

// ENTITY_LINKS are the site links a right click menu offers, for those the entity has
const ENTITY_LINKS = [["zkillUrl", "Open on zKillboard"], ["evewhoUrl", "Open on EveWho"], ["dotlanUrl", "Open on Dotlan"]];

// entityMenuItems are an entity's Search, when it has a type, and its site links. entity is { type, name, zkillUrl, evewhoUrl, dotlanUrl }, links without a url left out.
function entityMenuItems(entity) {
  const items = [];
  if (entity.type) items.push({ label: `Search for ${entity.name}`, action: () => offerSearch({ type: entity.type, name: entity.name, id: entity.id }) });
  for (const [key, label] of ENTITY_LINKS) {
    if (entity[key]) items.push({ label, action: () => window.runtime.BrowserOpenURL(entity[key]) });
  }
  return items;
}

/*
  openSectionsMenu opens a row's right click menu where it was right-clicked, kept on screen, or under the row when opened from the keyboard.
  It has a section for everything on the row, ie a pilot, their corp, and alliance, or a structure, its system, owner, and killer.
  Each section is { heading, entity } or { heading, items }: a grayed heading naming it, then the entity's Search and site links, or the items given,
  { label, action, disabled, forTarget }: one forTarget does nothing once another target's report is shown.
*/
function openSectionsMenu(event, sections) {
  const items = [];
  sections.forEach((section, index) => {
    if (index > 0) items.push(null);
    items.push({ heading: section.heading }, ...(section.items || entityMenuItems(section.entity)));
  });
  showContextMenu(event, items);
}

// showContextMenu shows the right click menu's items where it was right-clicked: null is a line, { heading } a grayed heading, the rest buttons
function showContextMenu(event, items) {
  event.preventDefault();
  event.stopPropagation();

  const menu = el("entity-menu");
  menu.replaceChildren(...items.map((item) => {
    if (!item) {
      const line = document.createElement("div");
      line.className = "context-menu-separator";
      line.setAttribute("role", "separator");
      return line;
    }
    if (item.heading) {
      const heading = document.createElement("div");
      heading.className = "context-menu-heading";
      heading.textContent = item.heading;
      return heading;
    }
    const button = document.createElement("button");
    button.type = "button";
    button.className = "context-menu-item";
    button.setAttribute("role", "menuitem");
    button.textContent = item.label;
    button.disabled = Boolean(item.disabled);
    button.addEventListener("click", () => runEntityMenuItem(item));
    return button;
  }));
  entityMenu = { target: intelTarget };

  menu.hidden = false;
  let x = event.clientX;
  let y = event.clientY;
  if (!x && !y) {
    const rect = event.currentTarget.getBoundingClientRect();
    x = rect.left + 24;
    y = rect.bottom;
  }
  menu.style.left = `${Math.max(4, Math.min(x, window.innerWidth - menu.offsetWidth - 4))}px`;
  menu.style.top = `${Math.max(4, Math.min(y, window.innerHeight - menu.offsetHeight - 4))}px`;
  const first = menu.querySelector(".context-menu-item:not(:disabled)");
  if (first) first.focus();
}

function closeEntityMenu() {
  el("entity-menu").hidden = true;
  entityMenu = null;
}

// runEntityMenuItem closes the menu and runs the item, unless it is for a target no longer shown
function runEntityMenuItem(item) {
  const shown = entityMenu && entityMenu.target === intelTarget;
  closeEntityMenu();
  if (shown || !item.forTarget) item.action();
}

// withMenu gives an element, ie a tab's row, its right click menu, see openSectionsMenu. sections can be a function of the event, to build them as the menu opens.
function withMenu(element, sections) {
  element.addEventListener("contextmenu", (event) => openSectionsMenu(event, typeof sections === "function" ? sections(event) : sections));
  return element;
}

/*
  allySections are an Allies row's menu: the ally, with its own items, adding it as a holding and ignoring it, above its Search and links,
  then its killmails with the target and its holdings on zKillboard, the kills against it too
*/
function allySections(ally) {
  const items = [...allyMenuExtras(ally), null, ...entityMenuItems(ally)]; // a line between its own items and the rest
  if (ally.togetherUrl) items.push({ label: "Open shared killmails on zKillboard", action: () => window.runtime.BrowserOpenURL(ally.togetherUrl) });
  return [{ ...entitySection(ally), items }];
}

// allyMenuExtras are an Allies row's own menu items: adding it as a holding of a corp or alliance target, and ignoring it for the target
function allyMenuExtras(ally) {
  const extras = [];
  const holdingTarget = intelTarget && shownTarget && intelTarget.id === shownTarget.id && intelTarget.type === shownTarget.type
    && (intelTarget.type === "CORPORATION" || intelTarget.type === "ALLIANCE");
  if (holdingTarget) {
    extras.push({ label: "Add to Holding", forTarget: true, action: () => openHoldingDialog(null, { type: ally.type, name: ally.name }) });
  }
  extras.push({
    label: `Ignore ${ally.name} for this target`,
    forTarget: true,
    disabled: !intelTarget,
    action: () => {
      if (ignoredAllies.some((other) => allyKey(other) === allyKey(ally))) return;
      saveIgnoredAllies([...ignoredAllies, { type: ally.type, id: ally.id, name: ally.name }], `Ignoring ${ally.name} for this target`);
    },
  });
  return extras;
}

// rankAlliesAgain shows the target's allies ranked again by the backend, ie after the hide killed allies preference changed. showKilled shows, for this search only, those it left out.
async function rankAlliesAgain(showKilled = showingKilledAllies) {
  if (!intelTarget || !intelReport || !intelReport.allies) return;
  const target = intelTarget;
  const wasShowing = showingKilledAllies;
  const update = await runBackend(() => backend().RankAllies(target.id, target.type, showKilled));
  // a new search may have started meanwhile
  if (!update || !update.allies || intelTarget !== target) return;
  showingKilledAllies = showKilled;
  intelReport.allies = update.allies;
  intelReport.ignoredAllies = update.ignoredAllies;
  intelReport.killedAllies = update.killedAllies;
  intelReport.holdings = update.holdings;
  renderAllies(intelReport.allies, intelReport.ignoredAllies, intelReport.killedAllies, intelReport.holdings || []);
  renderGlance(intelReport);
  if (showKilled !== wasShowing) tickerPush(showKilled ? "Showing the “Fair-Weather Friends” for this search" : "Hiding the “Fair-Weather Friends” again");
}

// saveIgnoredAllies saves the target's ignored allies at once, then shows its allies ranked again with them left out
async function saveIgnoredAllies(ignored, message) {
  if (!intelTarget || !intelReport) return;
  const target = intelTarget;
  const update = await runBackend(() => backend().SaveIgnoredAllies(target.id, target.type, ignored));
  // a new search may have started while saving
  if (!update || intelTarget !== target) return;
  if (update.allies) {
    intelReport.allies = update.allies;
    intelReport.killedAllies = update.killedAllies;
    intelReport.holdings = update.holdings;
  }
  intelReport.ignoredAllies = update.ignoredAllies;
  renderAllies(intelReport.allies, intelReport.ignoredAllies, intelReport.killedAllies || 0, intelReport.holdings || []);
  renderGlance(intelReport);
  tickerPush(message);
}

const MONTH_ABBREVIATIONS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

// spokenDate reads a time as an abbreviated month, ordinal day, and year in the chosen time zone, ie 2026-10-12T08:00:00Z is Oct 12th, 2026 in EVE time. Every date in the gui is written this way.
function spokenDate(isoTime) {
  const { year, month, day } = timeParts(isoTime);
  const lastTwo = day % 100;
  const suffix = lastTwo >= 11 && lastTwo <= 13 ? "th" : { 1: "st", 2: "nd", 3: "rd" }[day % 10] || "th";
  return `${MONTH_ABBREVIATIONS[month]} ${day}${suffix}, ${year}`;
}

/*
  killmailSpan describes the first and last killmail dates, ie "Killmails span from Oct 12th, 2026 to Oct 25th, 2026", or "Killmails on Oct 25th, 2026" when both are the same day.
  withDays adds how many days that covers, both included, ie "(14 days)", for the Activity tab. The tables show it in their Unique Days column instead.
  The days are counted on the dates shown, so in the time zone chosen in Preferences.
*/
function killmailSpan(firstTime, lastTime, withDays = false) {
  const first = spokenDate(firstTime);
  const last = spokenDate(lastTime);
  if (first === last) return `Killmails on ${first}`;
  if (!withDays) return `Killmails span from ${first} to ${last}`;
  const day = (isoTime) => { const parts = timeParts(isoTime); return Date.UTC(parts.year, parts.month, parts.day); };
  const days = Math.round((day(lastTime) - day(firstTime)) / 86400000) + 1;
  return `Killmails span from ${first} to ${last} (${days.toLocaleString("en-US")} days)`;
}

// daySpan returns how many calendar days (EVE time) the killmails cover, counting both the first and last day, ie Sep 2 to Oct 24 is 53
function daySpan(firstTime, lastTime) {
  const day = (isoTime) => Date.UTC(Number(isoTime.slice(0, 4)), Number(isoTime.slice(5, 7)) - 1, Number(isoTime.slice(8, 10)));
  return Math.round((day(lastTime) - day(firstTime)) / 86400000) + 1;
}

// uniqueDaysCell shows the unique days active, followed by the days the killmails span in parentheses when that is more than one day, ie 4 (53)
function uniqueDaysCell(uniqueDays, firstTime, lastTime) {
  const cell = numberCell(uniqueDays);
  const span = daySpan(firstTime, lastTime);
  if (span > 1) {
    cell.textContent = `${uniqueDays} (${span})`;
    cell.title = `Active on ${uniqueDays} of the ${span} days from the first killmail to the last`;
  }
  return cell;
}

// numberCell builds a right aligned table cell for a count
function numberCell(value) {
  const cell = document.createElement("td");
  cell.className = "num";
  cell.textContent = value;
  return cell;
}

const SPACE_ABBREVIATIONS = { Highsec: "HS", Lowsec: "LS", Nullsec: "NS" };

// SPACE_COLORS maps a wormhole class or static destination to its color: k-space by security, j-space classes in three shades of blue
const SPACE_COLORS = {
  Highsec: "space-hs", Lowsec: "space-ls", Nullsec: "space-ns",
  C1: "space-c1-3", C2: "space-c1-3", C3: "space-c1-3",
  C4: "space-c4-5", C5: "space-c4-5",
  C6: "space-c6",
};

// spaceLabel builds a colored label for a wormhole class or static destination, ie C5 or HS
function spaceLabel(space) {
  const label = document.createElement("span");
  label.className = `space ${SPACE_COLORS[space] || ""}`;
  label.textContent = SPACE_ABBREVIATIONS[space] || space;
  return label;
}

// SPACE_TAG_COLORS colors a system's kind of space by security, matching its security status. Wormhole classes use SPACE_COLORS.
const SPACE_TAG_COLORS = { "High Sec": "sec-high", "Low Sec": "sec-low", "Null Sec": "sec-null", Triglavian: "sec-trig" };

// spaceTag builds the square tag shown beside a system's name: its wormhole class, or High Sec, Low Sec, Null Sec, or Triglavian
function spaceTag(space) {
  const tag = document.createElement("span");
  tag.className = `space-tag ${SPACE_TAG_COLORS[space] || SPACE_COLORS[space] || ""}`;
  tag.textContent = space;
  return tag;
}

// staticLabel builds a static's wormhole code, ie B274, colored by where it leads
function staticLabel(static_) {
  const label = document.createElement("span");
  label.className = `space ${SPACE_COLORS[static_.destination] || ""}`;
  label.textContent = static_.code;
  label.title = `${static_.code} → ${SPACE_ABBREVIATIONS[static_.destination] || static_.destination}`;
  return label;
}

// systemLocation describes where a system is: its class and statics for j-space, otherwise its region
function systemLocation(system) {
  if (!system.class) {
    return system.region ? [system.region] : [];
  }

  const parts = [spaceLabel(system.class)];
  if (system.statics.length > 0) {
    const statics = document.createElement("span");
    statics.append("Statics: ");
    system.statics.forEach((static_, index) => {
      if (index > 0) statics.append(", ");
      statics.append(staticLabel(static_));
    });
    parts.push(statics);
  }
  return parts;
}

function renderSystems(systems) {
  const rows = systems.map((system) => {
    const row = document.createElement("tr");

    const nameCell = document.createElement("td");
    const name = document.createElement("div");
    const security = document.createElement("span");
    security.className = securityClass(system.securityBand);
    security.textContent = system.securityStatus.toFixed(1);
    name.append(`${system.name} `, security);
    const detail = document.createElement("div");
    detail.className = "row-detail";
    const parts = [...systemLocation(system), killmailSpan(system.firstSeen, system.lastSeen)];
    parts.forEach((part, index) => {
      if (index > 0) detail.append(" · ");
      detail.append(part);
    });
    nameCell.append(name, detail);

    row.append(avatarCell(system.name, system, ""), nameCell, numberCell(system.killmails), numberCell(system.kills), numberCell(system.losses), uniqueDaysCell(system.uniqueDays, system.firstSeen, system.lastSeen));
    return withMenu(row, [systemSection(system)]);
  });

  el("systems-body").replaceChildren(...rows);
  el("systems-empty").hidden = systems.length > 0;
}

function clearIntel() {
  intelReport = null;
  intelTarget = null;
  roster = null;
  el("report-warning").hidden = true;
  el("intel").hidden = true;
  el("intel-tabs").hidden = true;
}

// runBackend wraps a backend call so errors are reported by the status line and ticker, returning null on failure
async function runBackend(call) {
  try {
    return await call();
  } catch (err) {
    setStatus(String(err), true);
    tickerPush(`Error: ${err}`);
    return null;
  }
}

// recentSearches are the targets past searches found, newest first, as the backend's RecentSearch: { name, type }
let recentSearches = [];
// recentShown are the recent searches in the open list, and recentActive the index of the one the arrow keys are on, -1 for none
let recentShown = [];
let recentActive = -1;
// recentAll is true while the list shows every recent search, from the toggle or the down arrow, rather than those matching the typed name
let recentAll = false;

// loadRecentSearches reads the recent searches from the backend's cache, keeping the last list if that fails
function loadRecentSearches() {
  backend().GetRecentSearches().then((recent) => { recentSearches = recent || []; }).catch(() => {});
}

// matchingRecent returns the recent searches whose name starts with the typed text, then those that contain it, ignoring case. No text matches them all.
function matchingRecent(typed) {
  if (!typed) return recentSearches;
  const starting = [];
  const containing = [];
  for (const entry of recentSearches) {
    const at = entry.name.toLowerCase().indexOf(typed);
    // a ticker typed finds its corp or alliance too
    if (at === 0 || (entry.ticker && entry.ticker.toLowerCase().startsWith(typed))) starting.push(entry);
    else if (at > 0) containing.push(entry);
  }
  return [...starting, ...containing];
}

function setRecentOpen(open) {
  el("recent-list").hidden = !open;
  el("search-input").setAttribute("aria-expanded", String(open));
  el("recent-toggle").setAttribute("aria-expanded", String(open));
  if (!open) setRecentActive(-1);
}

// recentOption is one row of the list: the name with the typed part highlighted and a corp's or alliance's ticker, then its type
function recentOption(entry, index, typed) {
  const option = document.createElement("li");
  option.id = `recent-${index}`;
  option.className = "recent-option";
  option.setAttribute("role", "option");
  option.setAttribute("aria-selected", "false");

  const name = document.createElement("span");
  name.className = "recent-name";
  const at = typed ? entry.name.toLowerCase().indexOf(typed) : -1;
  if (at >= 0) {
    const match = document.createElement("span");
    match.className = "recent-match";
    match.textContent = entry.name.slice(at, at + typed.length);
    name.append(entry.name.slice(0, at), match, entry.name.slice(at + typed.length));
  } else {
    name.textContent = entry.name;
  }

  if (entry.ticker) {
    const ticker = document.createElement("span");
    ticker.className = "recent-ticker";
    ticker.textContent = ` ${formatTicker(entry.type, entry.ticker)}`;
    name.append(ticker);
  }

  const type = document.createElement("span");
  type.className = "recent-type";
  type.textContent = searchTypeLabel(entry.type);

  const remove = document.createElement("button");
  remove.type = "button";
  remove.className = "recent-remove";
  remove.tabIndex = -1;
  remove.textContent = "×";
  remove.title = "Remove from recent searches";
  remove.setAttribute("aria-label", `Remove ${entry.name} from recent searches`);
  remove.addEventListener("click", (event) => {
    event.stopPropagation(); // not a pick of the row
    removeRecent(entry);
  });

  option.append(name, type, remove);
  // mousedown would take focus from the search box, whose blur closes the list before the click lands
  option.addEventListener("mousedown", (event) => event.preventDefault());
  option.addEventListener("click", () => pickRecent(entry));
  return option;
}

// removeRecent forgets a recent search, from its × or the Delete key, keeping the list open on what is left
async function removeRecent(entry) {
  try {
    recentSearches = (await backend().RemoveRecentSearch(entry.name, entry.type)) || [];
  } catch (err) {
    setStatus(String(err), true);
    return;
  }
  const active = recentActive;
  showRecent(recentAll);
  if (active >= 0 && recentShown.length > 0) setRecentActive(Math.min(active, recentShown.length - 1));
}

/*
  showRecent opens the recent searches list: all of them from the toggle or the down arrow, or as a name is typed, those that match it.
  Typing that matches none, or clearing the box, closes it.
*/
function showRecent(all) {
  recentAll = all;
  const typed = all ? "" : el("search-input").value.trim().toLowerCase();
  recentShown = matchingRecent(typed);
  if (!all && (!typed || recentShown.length === 0)) {
    setRecentOpen(false);
    return;
  }

  const list = el("recent-list");
  if (recentShown.length === 0) {
    const empty = document.createElement("li");
    empty.className = "recent-empty";
    empty.textContent = "No recent searches yet";
    list.replaceChildren(empty);
  } else {
    list.replaceChildren(...recentShown.map((entry, index) => recentOption(entry, index, typed)));
  }
  setRecentOpen(true);
  setRecentActive(-1);
}

function setRecentActive(index) {
  recentActive = index;
  const options = el("recent-list").querySelectorAll(".recent-option");
  options.forEach((option, at) => option.setAttribute("aria-selected", String(at === index)));
  if (index >= 0 && options[index]) {
    el("search-input").setAttribute("aria-activedescendant", options[index].id);
    options[index].scrollIntoView({ block: "nearest" });
  } else {
    el("search-input").removeAttribute("aria-activedescendant");
  }
}

// moveRecent moves the arrow keys' place in the list by step, wrapping around, opening the full list first if it is closed
function moveRecent(step) {
  if (el("recent-list").hidden) showRecent(true);
  const count = recentShown.length;
  if (count === 0) return;
  setRecentActive(recentActive < 0 ? (step > 0 ? 0 : count - 1) : (recentActive + step + count) % count);
}

// pickRecent fills the search bar with a recent search and runs it
function pickRecent(entry) {
  setRecentOpen(false);
  searchTypeHint = entry.type;
  searchIdHint = entry.id || 0;
  el("search-input").value = entry.name;
  el("search-form").requestSubmit();
}

function recentKeydown(event) {
  const open = !el("recent-list").hidden;
  if (event.key === "ArrowDown" || (event.key === "ArrowUp" && open)) {
    event.preventDefault();
    moveRecent(event.key === "ArrowDown" ? 1 : -1);
  } else if (event.key === "Enter" && open && recentActive >= 0) {
    event.preventDefault();
    pickRecent(recentShown[recentActive]);
  } else if (event.key === "Delete" && open && recentActive >= 0) {
    // only with a row picked by the arrow keys, otherwise Delete edits the typed name as usual
    event.preventDefault();
    removeRecent(recentShown[recentActive]);
  } else if (event.key === "Escape" && open) {
    setRecentOpen(false);
  }
}

// showCaveatWarning opens the caveat warning as the app launches, while its preference is on
function showCaveatWarning() {
  if (!preferences.showCaveatWarning) return;
  el("caveat-dont-show").checked = false;
  el("caveat-warning").showModal();
}

// dismissCaveatWarning turns the caveat warning off from its Don't show this again box
function dismissCaveatWarning() {
  preferences.showCaveatWarning = false;
  backend().DismissCaveatWarning().catch((err) => tickerPush(String(err)));
}

// search looks up the target, then immediately pulls its killmails, since every intel section is built from them
async function search(event) {
  event.preventDefault();

  const name = el("search-input").value.trim();
  if (!name) {
    return;
  }
  const searchType = searchTypeHint;
  const searchId = searchIdHint;
  // a change in the search limits applies to this search
  closeLimitsMenu();
  await limitsPending;

  stopRequested = false;
  setBusy(true);
  setStatus("");
  clearIntel();
  const started = performance.now();
  let retrievedName = ""; // the target's name as EVE spells it, set once its intel is in
  let target = null;
  let report = null;
  let cancelled = false; // the user closed the choice between the name's matches

  try {
    target = await runBackend(() => backend().SearchTarget(name, searchType, searchId));
    // a name more than one thing has, or some ticker: the search is asked again for the one chosen, as EVE spells it
    if (target && target.choices) {
      const chosen = await chooseTarget(name, target.choices);
      cancelled = !chosen;
      if (chosen) {
        el("search-input").value = chosen.name;
        searchTypeHint = chosen.type;
        searchIdHint = chosen.id;
        target = await runBackend(() => backend().SearchTarget(chosen.name, chosen.type, chosen.id));
      } else {
        target = null;
      }
    }
    if (!target) {
      return;
    }
    renderTarget(target);
    // a holding is searched as its main, with the main's holdings
    if (target.searchedHolding) setStatus(`${target.searchedHolding} is a holding of ${target.display.Name}, so this is ${target.display.Name}'s search, with its holdings.`);
    // SearchTarget cached the found target as a recent search
    loadRecentSearches();
    if (stopRequested) {
      showProfileOnly("The search was stopped before any killmails were collected.");
      return;
    }

    el("intel-loading").hidden = false;
    el("intel").hidden = false;
    report = await runBackend(() => backend().CollectIntel(target.id, target.type));
    if (report) intelTarget = { id: target.id, type: target.type };

    if (report) {
      renderIntel(report);
      showReportWarning(report);
      retrievedName = target.display.Name;
    } else {
      showProfileOnly("No report: the search stopped or failed before its killmails were collected.");
    }
  } finally {
    const took = formatDuration(performance.now() - started);
    if (retrievedName) tickerPush(searchSummary(retrievedName, took, report));
    else tickerPush(cancelled ? `Search for ${name} cancelled` : `Search for ${name} stopped after ${took}`);
    setBusy(false);
  }

  // the Members tab looks up its first pages in the background, after the summary, so its progress only goes in the history
  if (report && report.memberList) startRoster(target.id, target.type);
}

/*
  chooseTarget asks which of a name's matches, the backend's TargetChoice, to search for, resolving with the one chosen, or null if closed without choosing.
  Those with the name come first, then the corps and alliances with it as their ticker, under their own heading when there are both.
*/
function chooseTarget(name, choices) {
  const dialog = el("target-choice");
  const named = choices.filter((choice) => !choice.byTicker).length;
  el("target-choice-title").textContent = named > 0 ? "More than one thing matches this. Which did you mean?" : "Nothing has exactly this name, but these have it as their ticker. Which did you mean?";
  el("target-choice-name").textContent = name;

  const rows = [];
  choices.forEach((choice, index) => {
    if (choice.byTicker && index === named && named > 0) {
      const heading = document.createElement("p");
      heading.className = "target-choice-heading";
      heading.textContent = "By ticker";
      rows.push(heading);
    }
    const button = document.createElement("button");
    button.className = "target-choice";
    button.value = String(index);
    let image;
    if (choice.imageUrl) {
      image = document.createElement("img");
      image.src = choice.imageUrl;
      image.alt = "";
    } else {
      image = document.createElement("span"); // a system has no portrait or logo
      image.className = "target-choice-blank";
    }
    const kind = document.createElement("span");
    kind.className = "target-choice-kind";
    kind.textContent = searchTypeLabel(choice.type);
    const choiceName = document.createElement("span");
    choiceName.textContent = choice.name;
    button.append(image, kind, choiceName);
    rows.push(button);
  });
  el("target-choice-list").replaceChildren(...rows);

  dialog.returnValue = "";
  return new Promise((resolve) => {
    // Cancel and Escape leave no index
    dialog.addEventListener("close", () => resolve(/^\d+$/.test(dialog.returnValue) ? choices[Number(dialog.returnValue)] : null), { once: true });
    dialog.showModal();
  });
}

// showReportWarning explains, between the target and its report, that the report was built from part of its data
function showReportWarning(report) {
  const count = (report.killmails || []).length.toLocaleString("en-US");
  const killmails = (report.killmails || []).length === 1 ? "killmail" : "killmails";
  let message = "";
  if (report.stopped) {
    message = `This report was stopped before data collection finished. It is built from the ${count} ${killmails} collected, and some details may be missing.`;
  } else if (report.rateLimited) {
    message = `The ESI rate limited this search, so some killmails or details may be missing from this report of ${count} ${killmails}.`;
  }
  el("report-warning").textContent = message;
  el("report-warning").hidden = !message;
}

// searchSummary is a finished search's ticker line, ie Info Retrieved on URSA. in 26.1s from 100 killmails (9748 hits from cache, 1763 from the ESI; zKillboard live)
function searchSummary(targetName, took, report) {
  const count = (report.killmails || []).length;
  const sources = report.sources ? ` (${report.sources})` : "";
  return `Info Retrieved on ${targetName} in ${took} from ${count} ${count === 1 ? "killmail" : "killmails"}${sources}`;
}

// formatDuration writes a search's run time, ie 8.4s or 2m 05s
function formatDuration(milliseconds) {
  const seconds = milliseconds / 1000;
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const whole = Math.round(seconds);
  return `${Math.floor(whole / 60)}m ${String(whole % 60).padStart(2, "0")}s`;
}

/*
  THEMES are the Theme menu presets, after EVE's UI themes plus Triglavian and Synthwave. Each is its two theme colors and its accent, the button color, as shown in the menu.
  The default ("") keeps the colors in style.css, the others derive their dark backgrounds from the first color. All are dark.
  Colors with a meaning (kill, loss, security, wormhole class) are not themed.
  A theme with an icon shows it in place of the app icon at the top left (BRAND_ICON); see ATTRIBUTION.md for where each came from.
*/
const THEMES = [
  { key: "", name: "GoSleuth (default)", swatches: ["#263040", "#7d8899", "#e0a526"] },
  { key: "amarr", name: "Amarr", swatches: ["#a68a64", "#f3e08c", "#f0884a"], icon: "icons/amarr.png" },
  { key: "caldari", name: "Caldari", swatches: ["#8ecbe6", "#3ec8c2", "#4aa8e0"], icon: "icons/caldari.png" },
  { key: "carbon", name: "Carbon", swatches: ["#d0d0d0", "#ececec", "#a3a8b0"], icon: "icons/carbon.png" },
  { key: "gallente", name: "Gallente", swatches: ["#4fb592", "#6fb9a7", "#3fae7a"], icon: "icons/gallente.png" },
  { key: "minmatar", name: "Minmatar", swatches: ["#c4573a", "#a0472f", "#d0452f"], icon: "icons/minmatar.png" },
  { key: "ore", name: "ORE", swatches: ["#d8aa22", "#3e8c8f", "#d98468"], icon: "icons/ore.png" },
  { key: "photon", name: "Photon", swatches: ["#4aa3b8", "#5cabc0", "#f0884a"], icon: "icons/photon.svg" },
  { key: "sisters", name: "Sisters of EVE", swatches: ["#e04a4a", "#9fe3e3", "#e8eaee"], icon: "icons/sisters.png" },
  { key: "triglavian", name: "Triglavian", swatches: ["#8c1d24", "#d6403a", "#e0453c"], icon: "icons/triglavian.png" },
  { key: "synthwave", name: "Synthwave", swatches: ["#b967ff", "#01cdfe", "#ff71ce"], icon: "icons/synthwave.png" },
];

// BRAND_ICON is the app icon at the top left, for the themes without one of their own
const BRAND_ICON = "icons/gosleuth.png";

// THEMED_PROPERTIES are the css variables a theme sets, cleared again for the default theme
const THEMED_PROPERTIES = ["--bg", "--panel", "--panel-raised", "--border", "--muted", "--accent", "--accent-text", "--highlight", "--badge", "--badge-text"];

// mixColor blends a little of color into base, ie mixColor("#a68a64", "#07090c", 0.06) is a near black tinted tan
function mixColor(color, base, amount) {
  const channels = (hex) => [1, 3, 5].map((start) => parseInt(hex.slice(start, start + 2), 16));
  const [a, b] = [channels(color), channels(base)];
  return `#${a.map((value, index) => Math.round(value * amount + b[index] * (1 - amount)).toString(16).padStart(2, "0")).join("")}`;
}

// contrastRatio is the WCAG contrast between two colors, 4.5 or more reads well as text
function contrastRatio(first, second) {
  const luminance = (hex) => {
    const [r, g, b] = [1, 3, 5].map((start) => {
      const value = parseInt(hex.slice(start, start + 2), 16) / 255;
      return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };
  const [lighter, darker] = [luminance(first), luminance(second)].sort((a, b) => b - a);
  return (lighter + 0.05) / (darker + 0.05);
}

// readableOn lightens a color just enough to read as text on background, so dark theme colors (ie Minmatar's) can still color badges and tabs
function readableOn(color, background) {
  for (let amount = 0; amount <= 1; amount += 0.05) {
    const lightened = mixColor("#ffffff", color, amount);
    if (contrastRatio(lightened, background) >= 4.5) return lightened;
  }
  return "#ffffff";
}

// themeColors works out a theme's css variables from its swatches
function themeColors([primary, secondary, accent]) {
  const panelRaised = mixColor(primary, "#11151c", 0.14);
  return {
    "--bg": mixColor(primary, "#07090c", 0.06),
    "--panel": mixColor(primary, "#0c0f14", 0.1),
    "--panel-raised": panelRaised,
    "--border": mixColor(primary, "#1a1f27", 0.3),
    "--muted": mixColor(primary, "#7d8899", 0.2),
    "--accent": accent,
    // button text: dark on a light accent such as Sisters' white, white on a dark one such as Minmatar's red
    "--accent-text": contrastRatio("#0b0e13", accent) >= contrastRatio("#ffffff", accent) ? "#0b0e13" : "#ffffff",
    "--highlight": readableOn(secondary, panelRaised),
    // a filled badge, ie the Fits tab's C, in the accent so it stands apart from the highlight's counts
    "--badge": accent,
    "--badge-text": contrastRatio("#0b0e13", accent) >= contrastRatio("#ffffff", accent) ? "#0b0e13" : "#ffffff",
  };
}

let currentTheme = "";

// applyTheme switches the app's colors to a preset, an unknown key falls back to the default
function applyTheme(key) {
  const theme = THEMES.find((preset) => preset.key === key) || THEMES[0];
  currentTheme = theme.key;

  const root = document.documentElement;
  if (theme.key === "") {
    THEMED_PROPERTIES.forEach((property) => root.style.removeProperty(property));
  } else {
    Object.entries(themeColors(theme.swatches)).forEach(([property, value]) => root.style.setProperty(property, value));
  }
  document.querySelector(".brand-icon").src = theme.icon || BRAND_ICON;

  for (const option of el("theme-list").children) {
    option.setAttribute("aria-checked", String(option.dataset.theme === currentTheme));
  }
}

// chooseTheme applies a theme from the menu and saves it for next time
async function chooseTheme(key) {
  applyTheme(key);
  try {
    await backend().SetTheme(key);
  } catch (err) {
    setStatus(String(err), true);
  }
}

function renderThemeList() {
  el("theme-list").replaceChildren(...THEMES.map((theme) => {
    const option = document.createElement("button");
    option.type = "button";
    option.className = "theme-option";
    option.setAttribute("role", "menuitemradio");
    option.dataset.theme = theme.key;

    theme.swatches.forEach((color, index) => {
      const swatch = document.createElement("span");
      swatch.className = index === 2 ? "theme-swatch theme-accent" : "theme-swatch";
      swatch.style.background = color;
      option.append(swatch);
    });
    const name = document.createElement("span");
    name.textContent = theme.name;
    option.append(name);

    option.addEventListener("click", () => chooseTheme(theme.key));
    return option;
  }));
}

// setMenuOpen shows or hides a menu and keeps its button's aria-expanded in step
function setMenuOpen(button, menu, open) {
  menu.hidden = !open;
  button.setAttribute("aria-expanded", String(open));
}

function closeThemesMenu() {
  setMenuOpen(el("themes-button"), el("themes-menu"), false);
}

// searchLimits are the search limits button's choices for this session, as the backend's SearchLimits, or null while searches use the saved Preferences
let searchLimits = null;
// limitsPending is the last change of search limits on its way to the backend, which a search waits for
let limitsPending = Promise.resolve();

// AGE_LABELS name the Killmails newer than choices, in days, for the search limits button
const AGE_LABELS = { 0: "any age", 7: "1 week", 30: "30 days", 90: "90 days" };

// savedLimits are the Preferences' search limits, the defaults
function savedLimits() {
  return {
    maxKillmails: preferences.maxKillmails,
    killmailPages: preferences.killmailPages,
    killmailsNewerThanDays: preferences.killmailsNewerThanDays,
    useCachedKillmails: preferences.useCachedKillmails,
    includeHoldings: preferences.includeHoldings,
  };
}

const limitsDiffer = (first, second) => Object.keys(first).some((key) => first[key] !== second[key]);

// showSearchLimits names the limits the next search uses on its button, ie 20 killmails · any age, in the accent while they are not the defaults
function showSearchLimits() {
  const limits = searchLimits || savedLimits();
  const count = limits.maxKillmails === ALL_KILLMAILS ? `All on ${limits.killmailPages} ${limits.killmailPages === 1 ? "page" : "pages"}` : `${limits.maxKillmails.toLocaleString("en-US")} killmails`;
  const button = el("limits-button");
  button.textContent = `${count} · ${AGE_LABELS[limits.killmailsNewerThanDays] || "any age"}`;
  const changed = searchLimits !== null;
  button.classList.toggle("limits-changed", changed);
  button.title = changed ? "Search limits for this session, not your defaults. Click to change them." : "Search limits, your defaults from Preferences. Click to change them for this session.";
}

function fillLimitsForm(limits) {
  el("limit-max").value = String(limits.maxKillmails);
  el("limit-pages").value = String(limits.killmailPages);
  el("limit-pages").disabled = limits.maxKillmails !== ALL_KILLMAILS;
  el("limit-age").value = String(limits.killmailsNewerThanDays);
  el("limit-cached").checked = limits.useCachedKillmails;
  el("limit-holdings").checked = limits.includeHoldings;
}

// limitsForm reads the search limits popup, the numbers as typed (NaN if not a number)
function limitsForm() {
  return {
    maxKillmails: Number(el("limit-max").value),
    killmailPages: Number(el("limit-pages").value),
    killmailsNewerThanDays: Number(el("limit-age").value),
    useCachedKillmails: el("limit-cached").checked,
    includeHoldings: el("limit-holdings").checked,
  };
}

function showLimitsError(message) {
  el("limits-error").textContent = message;
  el("limits-error").hidden = !message;
}

// changeSearchLimits applies the popup's limits to the next searches as they are changed, once they are valid. The defaults again are no limits of its own.
function changeSearchLimits() {
  const limits = limitsForm();
  el("limit-pages").disabled = limits.maxKillmails !== ALL_KILLMAILS;
  const problem = killmailLimitError(limits.maxKillmails, limits.killmailPages);
  showLimitsError(problem);
  if (problem) return;

  searchLimits = limitsDiffer(limits, savedLimits()) ? limits : null;
  const sent = searchLimits ? backend().SetSearchLimits(limits) : backend().ResetSearchLimits();
  limitsPending = sent.catch((err) => showLimitsError(String(err)));
  showSearchLimits();
}

function resetSearchLimits() {
  searchLimits = null;
  fillLimitsForm(savedLimits());
  showLimitsError("");
  limitsPending = backend().ResetSearchLimits().catch(() => {});
  showSearchLimits();
}

// saveSearchLimits makes the popup's limits the Preferences' defaults
async function saveSearchLimits() {
  const limits = limitsForm();
  const problem = killmailLimitError(limits.maxKillmails, limits.killmailPages);
  if (problem) {
    showLimitsError(problem);
    return;
  }
  try {
    await backend().SaveSearchLimits(limits);
  } catch (err) {
    showLimitsError(String(err));
    return;
  }
  Object.assign(preferences, limits);
  searchLimits = null;
  showSearchLimits();
  closeLimitsMenu();
  tickerPush("Search limits saved as your defaults");
}

function closeLimitsMenu() {
  setMenuOpen(el("limits-button"), el("limits-menu"), false);
}

function toggleLimitsMenu() {
  const open = el("limits-menu").hidden;
  if (open) {
    fillLimitsForm(searchLimits || savedLimits());
    showLimitsError("");
  }
  setMenuOpen(el("limits-button"), el("limits-menu"), open);
}

// MAX_KILLMAILS is the most killmails zKillboard lists, 100 pages of 200 (api.ZKILL_MAX_KILLMAILS)
const MAX_KILLMAILS = 20000;
// MAX_KILLMAIL_PAGES is the most zKillboard pages a search for all killmails reads (api.ZKILL_MAX_PAGES)
const MAX_KILLMAIL_PAGES = 100;
// ALL_KILLMAILS as the max killmail count collects every killmail zKillboard lists (retrieval.AllKillmails)
const ALL_KILLMAILS = -1;
// MAX_RECENT_SEARCHES is the most recent searches the search bar can keep (retrieval.MaxRecentSearches)
const MAX_RECENT_SEARCHES = 25;

// preferences are the saved Preferences window choices, as the backend's Preferences
let preferences = { includeZkillLabels: false, maxKillmails: 20, killmailPages: 10, killmailsNewerThanDays: 0, useCachedKillmails: false, timeZone: "eve", maxRecentSearches: 5, hideKilledAllies: false, includeHoldings: true, collapseTarget: false, showCaveatWarning: true, debugMode: false };

// applySettings sets the saved theme and preferences on the app
function applySettings(settings) {
  applyTheme(settings.theme);
  arrangeTabs(tabOrderFrom(settings.tabOrder || []));
  setSidebarCollapsed(Boolean(settings.sidebarCollapsed));
  preferences = {
    includeZkillLabels: Boolean(settings.includeZkillLabels),
    maxKillmails: settings.maxKillmails || 20,
    killmailPages: settings.killmailPages || 10,
    killmailsNewerThanDays: settings.killmailsNewerThanDays || 0,
    useCachedKillmails: Boolean(settings.useCachedKillmails),
    timeZone: settings.timeZone === "local" ? "local" : "eve",
    maxRecentSearches: settings.maxRecentSearches || 5,
    hideKilledAllies: Boolean(settings.hideKilledAllies),
    includeHoldings: settings.includeHoldings !== false,
    collapseTarget: Boolean(settings.collapseTarget),
    showCaveatWarning: settings.showCaveatWarning !== false,
    debugMode: Boolean(settings.debugMode),
  };
  showKillmailLabels = preferences.includeZkillLabels;
  showSearchLimits();
  showDebugMode();
  redrawIntel();
}

// showDebugMode puts the orange border and its name round the window while debug mode is on
function showDebugMode() {
  document.body.classList.toggle("debug-mode", preferences.debugMode);
  el("debug-tag").hidden = !preferences.debugMode;
}

// redrawIntel draws the shown report again after a preference that changes how it looks
function redrawIntel() {
  if (intelReport) renderIntelSections(intelReport);
  else renderKillmailPage();
}

function fillPreferencesForm(chosen) {
  el("pref-zkill-labels").checked = chosen.includeZkillLabels;
  el("pref-max-killmails").value = String(chosen.maxKillmails);
  el("pref-killmail-pages").value = String(chosen.killmailPages);
  showKillmailPagesUsed();
  el("pref-killmail-age").value = String(chosen.killmailsNewerThanDays);
  el("pref-use-cached").checked = chosen.useCachedKillmails;
  el("pref-time-zone").value = chosen.timeZone;
  el("pref-max-recent").value = String(chosen.maxRecentSearches);
  el("pref-hide-killed-allies").checked = chosen.hideKilledAllies;
  el("pref-include-holdings").checked = chosen.includeHoldings;
  el("pref-collapse-target").checked = chosen.collapseTarget;
  el("pref-caveat-warning").checked = chosen.showCaveatWarning;
  el("pref-debug-mode").checked = chosen.debugMode;
}

// showKillmailPagesUsed enables the pages box only while the max killmail count is -1, the only time it is used
function showKillmailPagesUsed() {
  el("pref-killmail-pages").disabled = Number(el("pref-max-killmails").value) !== ALL_KILLMAILS;
}

// preferencesForm reads the Preferences window, the numbers as typed (NaN if not a number)
function preferencesForm() {
  return {
    includeZkillLabels: el("pref-zkill-labels").checked,
    maxKillmails: Number(el("pref-max-killmails").value),
    killmailPages: Number(el("pref-killmail-pages").value),
    killmailsNewerThanDays: Number(el("pref-killmail-age").value),
    useCachedKillmails: el("pref-use-cached").checked,
    timeZone: el("pref-time-zone").value,
    maxRecentSearches: Number(el("pref-max-recent").value),
    hideKilledAllies: el("pref-hide-killed-allies").checked,
    includeHoldings: el("pref-include-holdings").checked,
    collapseTarget: el("pref-collapse-target").checked,
    showCaveatWarning: el("pref-caveat-warning").checked,
    debugMode: el("pref-debug-mode").checked,
  };
}

function preferencesChanged() {
  const form = preferencesForm();
  return Object.keys(preferences).some((key) => !Object.is(form[key], preferences[key]));
}

function showPreferencesError(message) {
  el("preferences-error").textContent = message;
  el("preferences-error").hidden = !message;
}

// what Export game data does, beside its button until an export says where it went
const STATIC_EXPORT_HINT = "Every game data file as GoSleuth reads it, your changes merged in";

// the user guide to the static_data folder, linked from Game data's description in Preferences > Advanced
const STATIC_DATA_GUIDE_URL = "https://github.com/lynkfox/eve-gosleuth/blob/main/docs/static-data-overrides.md";

function openPreferences() {
  closeThemesMenu();
  fillPreferencesForm(preferences);
  showPreferencesError("");
  showTabOrderReset();
  el("preferences-advanced").open = false;
  el("preferences").showModal();
  backend().GetCacheInfo().then(showCacheInfo).catch(() => {});
  backend().GetDebugInfo().then(showDebugInfo).catch(() => {});
  el("static-export-result").textContent = STATIC_EXPORT_HINT;
}

// showDebugInfo enables Open debug folder once there is a debug log to send, naming the newest
function showDebugInfo(info) {
  const button = el("debug-open");
  button.disabled = !info.latest;
  el("debug-latest").textContent = info.latest ? `Newest: ${info.latest}` : "No debug logs yet. Turn on Debug mode, save, and search.";
}

// exportStaticData writes the game data in use to a folder and opens it, naming the folder under the button
function exportStaticData() {
  const button = el("static-export");
  button.disabled = true;
  backend().ExportStaticData()
    .then((dir) => { el("static-export-result").textContent = `Exported to ${dir}`; })
    .catch((err) => tickerPush(`Error: ${err}`))
    .finally(() => { button.disabled = false; });
}

// formatBytes shortens a file size, ie 7208766 to 6.9 MB
function formatBytes(bytes) {
  const units = [[1024 ** 3, "GB"], [1024 ** 2, "MB"], [1024, "KB"]];
  for (const [size, unit] of units) {
    if (bytes >= size) return `${(bytes / size).toFixed(1)} ${unit}`;
  }
  return `${bytes} bytes`;
}

// cacheInfo is the cache as last shown in the Preferences window, from GetCacheInfo
let cacheInfo = null;

// showCacheInfo shows where the cache is and how much it holds, in the Preferences window
function showCacheInfo(info) {
  cacheInfo = info;
  el("cache-path").textContent = info.path;
  el("cache-path").title = info.path;
  const size = info.sizeBytes ? `, ${formatBytes(info.sizeBytes)} on disk` : "";
  el("cache-summary").textContent = `${info.entries.toLocaleString("en-US")} entries${size}${info.isDefault ? " · default location" : ""}`;
  el("cache-default").disabled = info.isDefault;
}

/*
  cacheAction runs one of the cache buttons' backend calls, disabling the buttons while it runs.
  The result is shown in the window, or the error if it failed, ie while a search is running.
*/
async function cacheAction(call) {
  const buttons = ["cache-move", "cache-default", "cache-clear"].map(el);
  buttons.forEach((button) => { button.disabled = true; });
  showPreferencesError("");
  try {
    showCacheInfo(await call());
  } catch (err) {
    showPreferencesError(String(err));
    backend().GetCacheInfo().then(showCacheInfo).catch(() => {});
  } finally {
    buttons.forEach((button) => { button.disabled = false; });
    el("cache-default").disabled = Boolean(cacheInfo && cacheInfo.isDefault);
  }
}

function openClearCache() {
  el("clear-cache-confirm").returnValue = "";
  // what the user set by hand is only cleared when asked each time
  el("clear-user-data").checked = false;
  el("clear-cache-confirm").showModal();
}

// clearCacheScope reads which clear cache choice is selected
function clearCacheScope() {
  const chosen = [...document.querySelectorAll('input[name="clear-scope"]')].find((input) => input.checked);
  return chosen ? chosen.value : "older";
}

// closePreferences closes the window, first asking if there are changes that were not saved
function closePreferences() {
  if (preferencesChanged()) {
    el("discard-confirm").returnValue = "";
    el("discard-confirm").showModal();
    return;
  }
  el("preferences").close();
}

// savePreferences checks and saves the window, applies the labels choice straight away, and closes it
// killmailLimitError says what is wrong with a max killmail count and pages for all as typed, NaN if not a number, or "" if nothing
function killmailLimitError(maxKillmails, killmailPages) {
  const killmailCountOk = maxKillmails === ALL_KILLMAILS || (Number.isInteger(maxKillmails) && maxKillmails >= 1 && maxKillmails <= MAX_KILLMAILS);
  if (!killmailCountOk) return `Max killmail count must be a whole number from 1 to ${MAX_KILLMAILS.toLocaleString("en-US")}, or -1 for all`;
  if (!Number.isInteger(killmailPages) || killmailPages < 1 || killmailPages > MAX_KILLMAIL_PAGES) return `zKillboard pages for all must be a whole number from 1 to ${MAX_KILLMAIL_PAGES}`;
  return "";
}

async function savePreferences() {
  const chosen = preferencesForm();
  const limitError = killmailLimitError(chosen.maxKillmails, chosen.killmailPages);
  if (limitError) {
    showPreferencesError(limitError);
    return;
  }
  if (!Number.isInteger(chosen.maxRecentSearches) || chosen.maxRecentSearches < 1 || chosen.maxRecentSearches > MAX_RECENT_SEARCHES) {
    showPreferencesError(`Recent searches kept must be a whole number from 1 to ${MAX_RECENT_SEARCHES}`);
    return;
  }

  try {
    await backend().SavePreferences(chosen);
  } catch (err) {
    showPreferencesError(String(err));
    // the backend may still be using the new preferences for this session, so match what it has
    backend().GetSettings().then(applySettings).catch(() => {});
    loadRecentSearches();
    return;
  }
  // a lower number of recent searches kept forgets the oldest
  loadRecentSearches();

  const alliesChanged = chosen.hideKilledAllies !== preferences.hideKilledAllies;
  preferences = chosen;
  showSearchLimits(); // new defaults, which the button shows unless it has limits of its own
  showDebugMode();
  showKillmailLabels = chosen.includeZkillLabels;
  redrawIntel();
  // a changed preference applies again, hiding the allies the target killed if it is on
  if (alliesChanged) rankAlliesAgain(false);
  el("preferences").close();
  tickerPush("Preferences saved, killmail limits apply from the next search");
}

el("themes-button").addEventListener("click", () => setMenuOpen(el("themes-button"), el("themes-menu"), el("themes-menu").hidden));
el("limits-button").addEventListener("click", toggleLimitsMenu);
el("myintel-button").addEventListener("click", openMyIntel);
el("myintel-close").addEventListener("click", () => el("myintel").close());
el("myintel-filter").addEventListener("input", renderMyIntel);
el("fits-filter").addEventListener("input", () => fitsShown.filter?.());
el("fits-filter-help").replaceWith(filterHelp(...FITS_FILTER_HELP));
el("myintel-add").addEventListener("submit", addMyIntelCard);
el("myintel-undo-button").addEventListener("click", undoMyIntelRemove);
el("myintel-export-button").addEventListener("click", exportMyIntel);
el("myintel-pick-all").addEventListener("click", pickAllShown);
el("myintel-delete-button").addEventListener("click", () => removeMyIntelCards(pickedCards()));
el("myintel-import-button").addEventListener("click", importMyIntel);
el("debug-open").addEventListener("click", () => backend().OpenDebugFolder().catch((err) => tickerPush(`Error: ${err}`)));
el("static-export").addEventListener("click", exportStaticData);
// a Preferences group's i opens its description, copied from the template its data-more names
for (const button of document.querySelectorAll(".preference-more")) {
  button.addEventListener("click", () => {
    const more = el(`more-${button.dataset.more}`);
    el("preference-more-title").textContent = more.dataset.title;
    el("preference-more-body").replaceChildren(more.content.cloneNode(true));
    for (const link of el("preference-more-body").querySelectorAll("[data-guide]")) link.addEventListener("click", () => window.runtime.BrowserOpenURL(STATIC_DATA_GUIDE_URL));
    el("preference-more").showModal();
  });
}
el("preference-more-close").addEventListener("click", () => el("preference-more").close());
// opening Advanced at the foot of Preferences brings it into view
el("preferences-advanced").addEventListener("toggle", (event) => {
  if (event.currentTarget.open) event.currentTarget.scrollIntoView({ block: "nearest", behavior: "smooth" });
});
el("limits-menu").addEventListener("input", changeSearchLimits);
el("limits-menu").addEventListener("change", changeSearchLimits);
// Enter closes the popup rather than starting a search
el("limits-menu").addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    event.preventDefault();
    closeLimitsMenu();
    el("limits-button").focus();
  }
});
el("limits-reset").addEventListener("click", resetSearchLimits);
el("limits-save").addEventListener("click", saveSearchLimits);
showSearchLimits(); // the built-in defaults until the saved settings arrive
// the theme list stays open while picking themes, so they can be compared, and closes on a click elsewhere or Escape
document.addEventListener("click", (event) => {
  if (!event.target.closest || !event.target.closest(".menu-anchor")) closeThemesMenu();
  if (!event.target.closest || !event.target.closest(".limits-anchor")) closeLimitsMenu();
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    closeThemesMenu();
    closeLimitsMenu();
    setHelpWindowOpen(false);
  }
});

renderThemeList();
applyTheme("");
// the WebView's user agent names its version, for debug logs
backend().SetFrontendInfo(navigator.userAgent).catch(() => {});
backend().GetSettings().then((settings) => {
  applySettings(settings);
  showCaveatWarning();
}).catch(() => applyTheme(""));
loadRecentSearches();
el("preferences-button").addEventListener("click", openPreferences);
el("preferences-close").addEventListener("click", closePreferences);
el("preferences-save").addEventListener("click", savePreferences);
// Escape asks too, rather than closing the window straight away
el("preferences").addEventListener("cancel", (event) => {
  event.preventDefault();
  closePreferences();
});
el("cache-move").addEventListener("click", () => cacheAction(() => backend().ChooseCacheFolder()));
el("cache-default").addEventListener("click", () => cacheAction(() => backend().ResetCacheFolder()));
el("cache-clear").addEventListener("click", openClearCache);
el("reset-tab-order").addEventListener("click", resetTabOrder);
el("clear-cache-confirm").addEventListener("close", () => {
  if (el("clear-cache-confirm").returnValue === "clear") cacheAction(() => backend().ClearCache(clearCacheScope(), el("clear-user-data").checked));
});
el("discard-confirm").addEventListener("close", () => {
  if (el("discard-confirm").returnValue === "discard") el("preferences").close();
});

el("help-window-close").addEventListener("click", () => setHelpWindowOpen(false));
el("help-window-titlebar").addEventListener("pointerdown", startHelpDrag);
// a smaller window can leave the help window off screen, so it is moved back on
window.addEventListener("resize", () => {
  if (el("help-window").open) helpWindowPlace = placeHelpWindow(parseFloat(el("help-window").style.left) || 0, parseFloat(el("help-window").style.top) || 0);
});
el("search-form").addEventListener("submit", search);
el("search-input").addEventListener("input", () => {
  // a typed name could be anything
  searchTypeHint = "";
  searchIdHint = 0;
  showRecent(false);
});
el("search-input").addEventListener("keydown", recentKeydown);
el("search-input").addEventListener("blur", () => setRecentOpen(false));
// the toggle keeps focus in the search box, so its blur does not close the list the toggle is opening
el("recent-toggle").addEventListener("mousedown", (event) => event.preventDefault());
el("recent-toggle").addEventListener("click", () => {
  if (el("recent-list").hidden) showRecent(true);
  else setRecentOpen(false);
  el("search-input").focus();
});
el("ticker-bar").addEventListener("click", toggleTickerHistory);
el("ticker-history").addEventListener("click", () => (el("ticker-history").hidden = true));
// About opens at What it is, the other sections shut, its Killmail Bias a copy of the caveat warning's text
function openAbout() {
  for (const section of el("about").querySelectorAll(".about-section")) section.open = section.id === "about-what";
  el("about-body").scrollTop = 0;
  el("about").showModal();
}
el("about-caveats").replaceChildren(...[...document.querySelector("#caveat-warning .caveat-body").children].map((child) => child.cloneNode(true)));
backend().Version().then((version) => {
  el("about-version").textContent = version ? `Version ${version}` : "";
  el("brand-version").textContent = version ? `Version ${version}` : "";
}).catch((error) => reportFrontendError(`Could not read the version: ${error}`, error));
el("brand").addEventListener("click", openAbout);
// GoSleuth's GitHub issues, opened by About's Report a bug once its reminder about debug logs is past
const BUG_REPORT_URL = "https://github.com/lynkfox/eve-gosleuth/issues";
el("about-report-bug").addEventListener("click", () => {
  const dialog = el("bug-report-confirm");
  dialog.returnValue = "";
  dialog.showModal();
});
el("bug-report-confirm").addEventListener("close", () => {
  if (el("bug-report-confirm").returnValue === "continue") window.runtime.BrowserOpenURL(BUG_REPORT_URL);
});
el("about-close").addEventListener("click", () => el("about").close());
// every About link is a button with data-url, so one handler covers them all; each shows its address on mouseover
for (const link of el("about-body").querySelectorAll("[data-url]")) link.title = link.dataset.url;
el("about-body").addEventListener("click", (event) => {
  const link = event.target.closest("[data-url]");
  if (link) window.runtime.BrowserOpenURL(link.dataset.url);
});
// the donation name is copied as the button shows it, so it lives only in index.html
el("about-copy-name").addEventListener("click", () => {
  const name = el("about-copy-name").textContent;
  window.runtime.ClipboardSetText(name)
    .then((copied) => tickerPush(copied ? `${name} copied` : "Could not copy the name"))
    .catch(() => tickerPush("Could not copy the name"));
});
el("caveat-understood").addEventListener("click", () => el("caveat-warning").close());
el("caveat-warning-close").addEventListener("click", () => el("caveat-warning").close());
el("caveat-warning").addEventListener("close", () => {
  if (el("caveat-dont-show").checked) dismissCaveatWarning();
});
el("search-confirm").addEventListener("close", () => {
  if (el("search-confirm").returnValue === "yes") {
    el("search-form").requestSubmit();
  }
});
el("page-prev").addEventListener("click", () => changeKillmailPage(-1));
el("page-next").addEventListener("click", () => changeKillmailPage(1));
el("roster-prev").addEventListener("click", () => changeRosterPage(-1));
wirePagerJumps(el("page-first"), el("page-last"), () => Math.ceil(killmails.length / KILLMAILS_PER_PAGE), showKillmailPage);
wirePagerJumps(el("roster-first"), el("roster-last"), () => (roster ? rosterPageCount() : 1), showRosterPage);
el("roster-next").addEventListener("click", () => changeRosterPage(1));
for (const tab of document.querySelectorAll(".tab")) tab.addEventListener("click", () => selectTab(tab.id));
// At a glance stays first, only the tabs below it move
for (const tab of el("tab-list").querySelectorAll(".tab")) wireTabMoving(tab);
el("sidebar-toggle").addEventListener("click", () => {
  // while the tag key is open the sidebar is shrunk for it, so the toggle shuts the key and gives the sidebar back
  if (fitKeyCollapsedBefore !== null) {
    closeFitKey();
    return;
  }
  setSidebarCollapsed(!sidebarCollapsed);
  // one save at a time, in the order clicked, so a quick double click can not save the older state last
  const collapsed = sidebarCollapsed;
  sidebarSaving = sidebarSaving.then(() => backend().SetSidebarCollapsed(collapsed)).catch((err) => setStatus(String(err), true));
});
NARROW_WINDOW.addEventListener("change", showSidebar);
el("fit-key-open").addEventListener("click", () => (fitKeyCollapsedBefore === null ? openFitKey() : closeFitKey()));
el("fit-key-close").addEventListener("click", closeFitKey);
// the tag tip is fixed to the page, so any scroll would leave it behind its tag
document.addEventListener("scroll", hideFitTagTip, true);
for (const subtab of document.querySelectorAll("#panel-structures .subtab")) {
  subtab.addEventListener("click", () => selectStructuresSubtab(subtab.id));
}

// progress events come from comm.ReportProgress in the Go backend
window.runtime.EventsOn("progress", (message) => tickerPush(message, true));

// the newest killmail is older than the date limit in Preferences, so the search would collect none: ignore the limit for this search, or keep it
window.runtime.EventsOn("date-limit", ({ newest, newerThan }) => {
  el("date-limit-detail").textContent = `The newest killmail for this target is from ${spokenDate(newest)}, older than the date limit of ${spokenDate(newerThan)} set in Preferences, `
    + `so this search would find none. Ignoring the limit collects them for this search only, still up to the max killmail count.`;
  el("date-limit-confirm").returnValue = "";
  el("date-limit-confirm").showModal();
});

// SEARCH_PROMPTS are the questions a running search waits on, each answering the backend with the value of its go ahead button
const SEARCH_PROMPTS = ["date-limit-confirm"];
const PROMPT_YES = { "date-limit-confirm": "ignore" };
for (const id of SEARCH_PROMPTS) {
  el(id).addEventListener("close", () => {
    // Stop has already answered it
    if (el(id).returnValue === "stop") return;
    backend().AnswerPrompt(el(id).returnValue === PROMPT_YES[id]).catch(() => {});
  });
}
el("ticker-stop").addEventListener("click", stopSearch);
wireTargetPull();
el("pref-max-killmails").addEventListener("input", showKillmailPagesUsed);
// the right click menu closes on a click or right click anywhere else, Escape, scrolling, or the window losing focus
for (const type of ["mousedown", "contextmenu"]) {
  document.addEventListener(type, (event) => {
    if (entityMenu && !el("entity-menu").contains(event.target)) closeEntityMenu();
  }, true);
}
document.addEventListener("scroll", () => { if (entityMenu) closeEntityMenu(); }, true);
window.addEventListener("blur", () => { if (entityMenu) closeEntityMenu(); });
window.addEventListener("resize", () => { if (entityMenu) closeEntityMenu(); });
el("entity-menu").addEventListener("keydown", (event) => {
  const items = [...el("entity-menu").querySelectorAll(".context-menu-item:not(:disabled)")];
  const at = items.indexOf(document.activeElement);
  if (event.key === "Escape") {
    event.preventDefault();
    closeEntityMenu();
  } else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    event.preventDefault();
    const step = event.key === "ArrowDown" ? 1 : -1;
    items[(at + step + items.length) % items.length].focus();
  } else if (event.key === "Tab") {
    closeEntityMenu();
  }
});
el("holding-form").addEventListener("submit", (event) => {
  // Save looks the holding up before closing, Cancel and Escape just close
  if (event.submitter && event.submitter.value === "save") {
    event.preventDefault();
    saveHolding();
  }
});
// Enter in a box saves, rather than the form's first button, Cancel
el("holding-form").addEventListener("keydown", (event) => {
  if (event.key === "Enter" && event.target.tagName === "INPUT") {
    event.preventDefault();
    if (!el("holding-save").disabled) saveHolding();
  }
});

// the ESI answered a 429: its requests are held until the time sent, and the search goes on without them
window.runtime.EventsOn("rate-limited", (until) => {
  const zone = preferences.timeZone === "local" ? "local" : "EVE";
  el("rate-limited-detail").textContent = `You will need to wait 15 minutes before continuing, until ${clockTime(timeParts(until))} ${zone}. Killmails this search needed from the ESI are skipped, and names it could not look up are shown as unknown.`;
  if (!el("rate-limited").open) el("rate-limited").showModal();
});

