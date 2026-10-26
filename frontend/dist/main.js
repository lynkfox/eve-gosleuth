// Bound Go methods are injected by the Wails runtime as window.go.<package>.<struct>.<method>, each returning a Promise
const backend = () => window.go.gui.App;

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
  el("search-button").disabled = busy;
  el("search-input").disabled = busy;
  el("search-type").disabled = busy;
  el("recent-toggle").disabled = busy;
  if (busy) setRecentOpen(false);
  el("ticker").classList.toggle("busy", busy);
}

// searching is true while a search runs, see setBusy
let searching = false;

/*
  tickerPush shows a status update in the ticker bar and adds it, timestamped, to the top of the history list.
  The bar itself has no time, since it would only change when a new update arrives.
  A background update, ie the backend's progress, only reaches the bar while a search runs, so a finished search's summary stays shown. Otherwise it only goes in the history.
*/
function tickerPush(message, background = false) {
  const time = clockTime(timeParts(new Date().toISOString()), true);

  if (!background || searching) el("ticker-message").textContent = message;

  const entry = document.createElement("li");
  const stamp = document.createElement("time");
  stamp.textContent = time;
  const text = document.createElement("span");
  text.textContent = message;
  entry.append(stamp, text);

  const history = el("ticker-history");
  history.prepend(entry);
  while (history.children.length > TICKER_HISTORY_LIMIT) {
    history.lastElementChild.remove();
  }
}

function toggleTickerHistory() {
  const history = el("ticker-history");
  history.hidden = !history.hidden || history.children.length === 0;
}

// isSearchable returns true if the search type dropdown has an option for the entity type, ie PILOT
function isSearchable(type) {
  return [...el("search-type").options].some((option) => option.value === type);
}

// searchTypeLabel is the search type dropdown's name for an entity type, ie Pilot for PILOT
function searchTypeLabel(type) {
  const option = [...el("search-type").options].find((choice) => choice.value === type);
  return option ? option.textContent : type;
}

// offerSearch fills the search bar with an entity from the target panel, then asks before searching for it
function offerSearch(entity) {
  if (el("search-button").disabled) {
    return; // a search is already running
  }

  el("search-type").value = entity.type;
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

  side.append(siteLinks(entity.name, entity));

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
  if (entity.spaceLabel) {
    const nameLine = document.createElement("div");
    nameLine.className = "entity-name-line";
    nameLine.append(name, spaceTag(entity.spaceLabel));
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
    name.addEventListener("click", () => offerSearch({ type: "SYSTEM", name: gate.Name }));

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
    name.addEventListener("click", () => offerSearch({ type: "CORPORATION", name: corp.Name }));

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

  const target = { label: result.type, name: display.Name, type: result.type, imageUrl: display.TargetImageUrl, zkillUrl: display.ZkillUrl, evewhoUrl: display.EveWhoUrl, dotlanUrl: display.DotlanUrl, spaceLabel: display.SpaceLabel };
  const related = (display.Related || []).map((entity) => ({
    label: entity.Title, name: entity.Name, type: entity.TypeName, imageUrl: entity.ImageUrl, zkillUrl: entity.ZkillUrl, evewhoUrl: entity.EveWhoUrl,
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

  el("empty").hidden = true;
  el("target").hidden = false;
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
  name.addEventListener("click", () => offerSearch({ type: "PILOT", name: member.name }));
  nameCell.append(name);

  const lastSeen = member.lastSeen ? `${spokenDate(member.lastSeen)} (${daysAgo(member.lastSeen)})` : "Not seen";
  row.append(avatarCell(member.name, member, member.imageUrl, true), nameCell, textCell(member.born), textCell(member.timeInCorp), textCell(member.npcTime), textCell(lastSeen));
  // EveWho still lists them, but the ESI has them out of the corp
  if (member.left) {
    row.className = "member-left";
    row.title = "Has left the corp";
  }
  return row;
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
  name.addEventListener("click", () => offerSearch({ type: "CORPORATION", name: corp.name }));
  nameCell.append(name);

  row.append(avatarCell(corp.name, corp, corp.iconUrl, true), nameCell, textCell(corp.memberCount.toLocaleString("en-US")), textCell(corp.timeInAlliance));
  return row;
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
  el("killmail-count").textContent = `(${killmails.length})`;
  renderIntelSections(report);

  selectTab("tab-killmails");
  el("intel-loading").hidden = true;
  el("intel-tabs").hidden = false;
}

// renderIntelSections draws the killmail page and each tab of a report, keeping the page and tab the user is on
function renderIntelSections(report) {
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

  // the Members tab's rows come from GetMembers, started by the search
  el("tab-roster").hidden = !report.memberList;
  if (report.memberList) renderRosterPage();
}

// ACTIVITY_DAYS are the histogram's rows, Monday first, as [name, Date.getDay() value]
const ACTIVITY_DAYS = [["Monday", 1], ["Tuesday", 2], ["Wednesday", 3], ["Thursday", 4], ["Friday", 5], ["Saturday", 6], ["Sunday", 0]];

/*
  renderActivity draws the Activity tab: a grid of the killmails seen in each hour of each day of the week, in the time zone chosen in Preferences.
  The busier an hour, the stronger its accent color, and each hour shows its count. The last row and column total each hour and day.
*/
function renderActivity(kms) {
  const counts = Array.from({ length: 7 }, () => Array(24).fill(0)); // [weekday][hour]
  for (const km of kms) {
    const parts = timeParts(km.time);
    counts[parts.weekday][parts.hour]++;
  }
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
  const times = kms.map((km) => km.time).sort();
  el("activity-span").textContent = times.length ? killmailSpan(times[0], times[times.length - 1], true) : "";
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
      button.addEventListener("click", () => offerSearch({ type: "PILOT", name: pilot.name }));
      item.append(button);

      const group = groupLine([pilot.corporationIcon, pilot.corporation], [pilot.allianceIcon, pilot.alliance]);
      if (group) item.append(group);
      return item;
    }));

    ship.append(summary, pilots);
    return ship;
  }));
  el("capitals-empty").hidden = capitals.length > 0;
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
  name.addEventListener("click", () => offerSearch({ type: "PILOT", name: pilot.name }));
  who.append(name);
  const group = groupLine([pilot.corporationIcon, pilot.corporation], [pilot.allianceIcon, pilot.alliance]);
  if (group) who.append(group);

  const count = document.createElement("span");
  count.className = "pi-count";
  count.textContent = `${pilot.losses.length} ${pilot.losses.length === 1 ? "loss" : "losses"}`;
  header.append(portrait, who);
  header.append(hourStrip(pilot.losses.map((loss) => loss.time), "loss", "losses"), count);

  const losses = document.createElement("ol");
  losses.className = "pi-losses";
  losses.append(...pilot.losses.map(lossRow));

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

  const location = document.createElement("span");
  location.className = "pi-planet";
  location.textContent = loss.location || "—";
  if (!loss.location) location.title = "No nearest celestial";

  const value = document.createElement("span");
  value.className = "pi-value";
  value.textContent = formatIsk(loss.totalValue);

  const [label, meaning] = RAT_KINDS[loss.kind] || ["", ""];
  const tag = document.createElement("span");
  tag.className = `pi-tag rat-kind-${loss.kind}`;
  tag.textContent = label;

  row.append(time, ship, location, value, tag);

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

// arrangeTabs puts the tabs in an order, hidden ones included, before the help button at the end of the row
function arrangeTabs(order) {
  for (const id of order) el("tab-list").insertBefore(el(id), el("tab-key"));
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

// dropsBefore reports whether a drag over a tab would put the dragged tab before it, from which half of the tab the pointer is over
function dropsBefore(tab, event) {
  const box = tab.getBoundingClientRect();
  return event.clientX < box.left + box.width / 2;
}

/*
  wireTabMoving lets a tab be dragged into a new place among the others, or moved one place with Alt and the left or right arrow.
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
    if (!event.altKey || (event.key !== "ArrowLeft" && event.key !== "ArrowRight")) return;
    event.preventDefault();
    // past the next tab that is shown, hidden ones are skipped over
    const shown = [...el("tab-list").querySelectorAll(".tab")].filter((other) => !other.hidden);
    const at = shown.indexOf(tab);
    const neighbour = shown[at + (event.key === "ArrowLeft" ? -1 : 1)];
    if (!neighbour) return;
    el("tab-list").insertBefore(tab, event.key === "ArrowLeft" ? neighbour : neighbour.nextSibling);
    tab.focus();
    saveTabOrder();
  });
}

// UNIQUE_DAYS_HELP explains the Unique Days column the ranked tabs share
const UNIQUE_DAYS_HELP = "The number in parentheses on Unique Days is the number of days from their first to last killmail reviewed.";

/*
  TAB_HELP explains each tab as bullet points, shown with its color key, if any, on hover over the tab's ? button.
  An entry is a list of points, or a function of the target type (PILOT, CORPORATION, ALLIANCE, or SYSTEM) for a tab that differs by target.
*/
const TAB_HELP = {
  "tab-killmails": [
    "Every killmail collected for this search, newest first. Click one to open it on zKillboard.",
    "How many are collected, and how far back, is set in Preferences.",
  ],
  "tab-activity": [
    "A display, zKillboard style, of when killmails were recorded, for potential activity times.",
    "Only uses the killmails that were analyzed during this search.",
  ],
  "tab-fleetmates": [
    "The 10 pilots on the most killmails with this pilot, who are in the same corporation or alliance now or attacked alongside them, ties going to the most unique days.",
    UNIQUE_DAYS_HELP,
  ],
  "tab-members": [
    "The 10 pilots on the most killmails flying for this corporation or alliance, counted by who they flew for at the time, ties going to the most unique days.",
    UNIQUE_DAYS_HELP,
  ],
  "tab-pilots": [
    "A list of the 10 pilots with the most killmails over the most unique days.",
    UNIQUE_DAYS_HELP,
  ],
  "tab-corps": [
    "A list of the 10 corporations with the most killmails over the most unique days.",
    UNIQUE_DAYS_HELP,
    "Pilots is the number of different pilots seen with that corp on reviewed killmails.",
  ],
  "tab-systems": [
    "The 10 systems with the most of this target's killmails, ties going to the most unique days. Kills and Losses are from the target's side.",
    UNIQUE_DAYS_HELP,
  ],
  "tab-pi": [
    "Any hauler lost near a planet, or any ship carrying PI materials lost anywhere in this system.",
    "Losses marked PI had PI materials aboard: hover over one to see them.",
    "A — for the planet means the loss was not near one.",
    "With 3 or more losses, the Overall row at the top shows the hours of them all.",
  ],
  "tab-ratters": [
    "Any ship lost in this system with NPCs on the killmail.",
    "Pilots with a loss tagged NPC, only NPCs on it, come first, then SB, NPCs and players who only used smartbombs, then PVP, NPCs and players with other weapons.",
    "Hover over a loss to see the NPCs and players on it. A — for the location means it had no nearest celestial.",
    "With 3 or more losses, the Overall row at the top shows the hours of them all.",
  ],
  "tab-capitals": [
    "Every capital ship flown by this corporation's or alliance's pilots on the reviewed killmails, and who flew it. Click a pilot to search for them.",
    "A struck through pilot has since left the alliance.",
  ],
  "tab-roster": (type) => type === "ALLIANCE"
    ? ["This alliance's current member corporations, with their member counts and how long they have been in the alliance."]
    : [
      "This corporation's current members: first those on the reviewed killmails, most recently seen first, then the rest from EveWho's records.",
      "A red underline means EveWho still lists them, but the ESI has them out of the corporation.",
    ],
};

function selectTab(tabId) {
  const help = typeof TAB_HELP[tabId] === "function" ? TAB_HELP[tabId](roster ? roster.type : "") : TAB_HELP[tabId];
  const key = TAB_COLOR_KEYS[tabId];
  shownHelp = help || key ? { title: el(tabId).firstChild.textContent.trim(), points: help || [], sections: key || [] } : null;
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
  const help = document.createElement("div");
  help.className = "tab-help";

  const toggle = document.createElement("button");
  toggle.type = "button";
  toggle.className = "tab-help-toggle";
  toggle.textContent = "?";
  toggle.title = "Click to keep this open in its own window";
  toggle.setAttribute("aria-label", "About this tab");
  toggle.setAttribute("aria-expanded", String(el("help-window").open));
  toggle.addEventListener("click", () => toggleHelpWindow(toggle));

  const popup = document.createElement("div");
  popup.className = "tab-help-popup help-content";
  popup.setAttribute("role", "tooltip");
  popup.append(...helpContent(points, sections));

  help.append(toggle, popup);
  return help;
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

// TAB_COLOR_KEYS is the color key shown for each tab, tabs without colored data have none
const TAB_COLOR_KEYS = {
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

// groupLine shows a corp and alliance as 16px logos beside their names, ie [logo] URSA PRIMUS · [logo] URSA., or null if there is neither
function groupLine(...groups) {
  const named = groups.filter(([, name]) => name);
  if (named.length === 0) return null;

  const line = document.createElement("span");
  line.className = "group-line";
  named.forEach(([icon, name], index) => {
    if (index > 0) line.append(" · ");
    if (icon) {
      const logo = document.createElement("img");
      logo.src = icon;
      logo.alt = "";
      line.append(logo);
    }
    line.append(name);
  });
  return line;
}

function renderKillmailCard(km) {
  const card = document.createElement("li");
  card.className = `killmail ${km.outcome}`.trim();
  card.tabIndex = 0;
  card.title = "Open on zKillboard";

  const ship = document.createElement("img");
  ship.src = km.shipImageUrl;
  ship.alt = "";

  const victimGroup = groupLine([km.victimCorpIcon, km.victimCorp], [km.victimAllianceIcon, km.victimAlliance]);
  const finalBlowGroup = groupLine([km.finalBlowCorpIcon, km.finalBlowCorp], [km.finalBlowAllianceIcon, km.finalBlowAlliance]);

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
  }

  // the date goes in the value column directly, not wrapped like its other lines, so it can be its own small label
  const time = document.createElement("span");
  time.className = "killmail-time";
  time.textContent = formatKillmailTime(km.time);
  const value = column([`Dropped ${formatIsk(km.droppedValue || 0)}`, outcome], "killmail-value");
  value.append(time);

  // the headings share the top line, with the ship image and each heading's details on the line below
  card.append(
    cardHeading(km.victimName, "killmail-victim"),
    cardHeading(km.shipName, "killmail-ship"),
    cardHeading(attackers, "killmail-attackers"),
    cardHeading(formatIsk(km.totalValue), "killmail-total"),
    ship,
    column([victimGroup], "killmail-victim"),
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
    const group = groupLine([mate.corporationIcon, mate.corporation], [mate.allianceIcon, mate.alliance]);
    detail.append(...(group ? [group, " · "] : []), killmailSpan(mate.firstTogether, mate.lastTogether));
    nameCell.append(name, detail);

    const killmailsCell = document.createElement("td");
    killmailsCell.className = "num";
    killmailsCell.textContent = mate.killmailsShared;

    row.append(avatarCell(mate.name, mate, mate.imageUrl), nameCell, killmailsCell, uniqueDaysCell(mate.uniqueDays, mate.firstTogether, mate.lastTogether));
    body.append(row);
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
    const alliance = groupLine([corporation.allianceIcon, corporation.alliance]);
    detail.append(...(alliance ? [alliance, " · "] : []), killmailSpan(corporation.firstSeen, corporation.lastSeen));
    nameCell.append(name, detail);

    row.append(avatarCell(corporation.name, corporation, corporation.imageUrl), nameCell, numberCell(corporation.killmails), numberCell(corporation.kills), numberCell(corporation.losses), numberCell(corporation.pilots), uniqueDaysCell(corporation.uniqueDays, corporation.firstSeen, corporation.lastSeen));
    return row;
  });

  el("corps-body").replaceChildren(...rows);
  el("corps-empty").hidden = corporations.length > 0;
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
    return row;
  });

  el("systems-body").replaceChildren(...rows);
  el("systems-empty").hidden = systems.length > 0;
}

function clearIntel() {
  intelReport = null;
  roster = null;
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
    if (at === 0) starting.push(entry);
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

// recentOption is one row of the list: the name with the typed part highlighted, then its type
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
  el("search-type").value = entry.type;
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

// search looks up the target, then immediately pulls its killmails, since every intel section is built from them
async function search(event) {
  event.preventDefault();

  const name = el("search-input").value.trim();
  if (!name) {
    return;
  }
  const searchType = el("search-type").value;

  setBusy(true);
  setStatus("");
  clearIntel();
  const started = performance.now();
  let retrievedName = ""; // the target's name as EVE spells it, set once its intel is in
  let target = null;
  let report = null;

  try {
    target = await runBackend(() => backend().SearchTarget(name, searchType));
    if (!target) {
      return;
    }
    renderTarget(target);
    // SearchTarget cached the found target as a recent search
    loadRecentSearches();

    el("intel-loading").hidden = false;
    el("intel").hidden = false;
    report = await runBackend(() => backend().CollectIntel(target.id, target.type));

    if (report) {
      renderIntel(report);
      retrievedName = target.display.Name;
    } else {
      el("intel").hidden = true;
    }
  } finally {
    const took = formatDuration(performance.now() - started);
    tickerPush(retrievedName ? searchSummary(retrievedName, took, report) : `Search for ${name} stopped after ${took}`);
    setBusy(false);
  }

  // the Members tab looks up its first pages in the background, after the summary, so its progress only goes in the history
  if (report && report.memberList) startRoster(target.id, target.type);
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
  THEMES are the Theme menu presets, after EVE's UI themes plus Triglavian. Each is its two theme colors and its accent, the button color, as shown in the menu.
  The default ("") keeps the colors in style.css, the others derive their dark backgrounds from the first color. All are dark.
  Colors with a meaning (kill, loss, security, wormhole class) are not themed.
*/
const THEMES = [
  { key: "", name: "GoSleuth (default)", swatches: ["#263040", "#7d8899", "#e0a526"] },
  { key: "amarr", name: "Amarr", swatches: ["#a68a64", "#f3e08c", "#f0884a"] },
  { key: "caldari", name: "Caldari", swatches: ["#8ecbe6", "#3ec8c2", "#4aa8e0"] },
  { key: "carbon", name: "Carbon", swatches: ["#d0d0d0", "#ececec", "#a3a8b0"] },
  { key: "gallente", name: "Gallente", swatches: ["#4fb592", "#6fb9a7", "#3fae7a"] },
  { key: "minmatar", name: "Minmatar", swatches: ["#c4573a", "#a0472f", "#d0452f"] },
  { key: "ore", name: "ORE", swatches: ["#d8aa22", "#3e8c8f", "#d98468"] },
  { key: "photon", name: "Photon", swatches: ["#4aa3b8", "#5cabc0", "#f0884a"] },
  { key: "sisters", name: "Sisters of EVE", swatches: ["#e04a4a", "#9fe3e3", "#e8eaee"] },
  { key: "triglavian", name: "Triglavian", swatches: ["#8c1d24", "#d6403a", "#e0453c"] },
];

// THEMED_PROPERTIES are the css variables a theme sets, cleared again for the default theme
const THEMED_PROPERTIES = ["--bg", "--panel", "--panel-raised", "--border", "--muted", "--accent", "--accent-text", "--highlight"];

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

// MAX_KILLMAILS is the most killmails zKillboard lists, 10 pages of 200 (api.ZKILL_MAX_KILLMAILS)
const MAX_KILLMAILS = 2000;
// ALL_KILLMAILS as the max killmail count collects every killmail zKillboard lists (retrieval.AllKillmails)
const ALL_KILLMAILS = -1;
// MAX_RECENT_SEARCHES is the most recent searches the search bar can keep (retrieval.MaxRecentSearches)
const MAX_RECENT_SEARCHES = 25;

// preferences are the saved Preferences window choices, as the backend's Preferences
let preferences = { includeZkillLabels: false, maxKillmails: 20, killmailsNewerThanDays: 0, useCachedKillmails: false, timeZone: "eve", maxRecentSearches: 5 };

// applySettings sets the saved theme and preferences on the app
function applySettings(settings) {
  applyTheme(settings.theme);
  arrangeTabs(tabOrderFrom(settings.tabOrder || []));
  preferences = {
    includeZkillLabels: Boolean(settings.includeZkillLabels),
    maxKillmails: settings.maxKillmails || 20,
    killmailsNewerThanDays: settings.killmailsNewerThanDays || 0,
    useCachedKillmails: Boolean(settings.useCachedKillmails),
    timeZone: settings.timeZone === "local" ? "local" : "eve",
    maxRecentSearches: settings.maxRecentSearches || 5,
  };
  showKillmailLabels = preferences.includeZkillLabels;
  redrawIntel();
}

// redrawIntel draws the shown report again after a preference that changes how it looks
function redrawIntel() {
  if (intelReport) renderIntelSections(intelReport);
  else renderKillmailPage();
}

function fillPreferencesForm(chosen) {
  el("pref-zkill-labels").checked = chosen.includeZkillLabels;
  el("pref-max-killmails").value = String(chosen.maxKillmails);
  el("pref-killmail-age").value = String(chosen.killmailsNewerThanDays);
  el("pref-use-cached").checked = chosen.useCachedKillmails;
  el("pref-time-zone").value = chosen.timeZone;
  el("pref-max-recent").value = String(chosen.maxRecentSearches);
}

// preferencesForm reads the Preferences window, the max count as typed (NaN if it is not a number)
function preferencesForm() {
  return {
    includeZkillLabels: el("pref-zkill-labels").checked,
    maxKillmails: Number(el("pref-max-killmails").value),
    killmailsNewerThanDays: Number(el("pref-killmail-age").value),
    useCachedKillmails: el("pref-use-cached").checked,
    timeZone: el("pref-time-zone").value,
    maxRecentSearches: Number(el("pref-max-recent").value),
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

function openPreferences() {
  closeThemesMenu();
  fillPreferencesForm(preferences);
  showPreferencesError("");
  showTabOrderReset();
  el("preferences").showModal();
  backend().GetCacheInfo().then(showCacheInfo).catch(() => {});
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
async function savePreferences() {
  const chosen = preferencesForm();
  const killmailCountOk = chosen.maxKillmails === ALL_KILLMAILS || (Number.isInteger(chosen.maxKillmails) && chosen.maxKillmails >= 1 && chosen.maxKillmails <= MAX_KILLMAILS);
  if (!killmailCountOk) {
    showPreferencesError(`Max killmail count must be a whole number from 1 to ${MAX_KILLMAILS.toLocaleString("en-US")}, or -1 for all`);
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

  preferences = chosen;
  showKillmailLabels = chosen.includeZkillLabels;
  redrawIntel();
  el("preferences").close();
  tickerPush("Preferences saved, killmail limits apply from the next search");
}

el("themes-button").addEventListener("click", () => setMenuOpen(el("themes-button"), el("themes-menu"), el("themes-menu").hidden));
// the theme list stays open while picking themes, so they can be compared, and closes on a click elsewhere or Escape
document.addEventListener("click", (event) => {
  if (!event.target.closest || !event.target.closest(".menu-anchor")) closeThemesMenu();
});
document.addEventListener("keydown", (event) => {
  if (event.key === "Escape") {
    closeThemesMenu();
    setHelpWindowOpen(false);
  }
});

renderThemeList();
applyTheme("");
backend().GetSettings().then(applySettings).catch(() => applyTheme(""));
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
  if (el("clear-cache-confirm").returnValue === "clear") cacheAction(() => backend().ClearCache(clearCacheScope()));
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
el("search-input").addEventListener("input", () => showRecent(false));
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
for (const tab of document.querySelectorAll(".tab")) {
  tab.addEventListener("click", () => selectTab(tab.id));
  wireTabMoving(tab);
}

// progress events come from comm.ReportProgress in the Go backend
window.runtime.EventsOn("progress", (message) => tickerPush(message, true));
