"use strict";
// Everything from the war is data: it is read as text from the data block and only ever set with textContent.
const DATA = JSON.parse(document.getElementById("war-data").textContent);
const META = DATA.meta;
const COLORS = { a: "#4d9de0", b: "#f08a3c", neutral: "#7d8899", panel: "#131820", border: "#263040", text: "#d8dee9", muted: "#7d8899" };
const CLASS_LABELS = { high: "HS", low: "LS", null: "NS" };

// make builds an element with a class and text
function make(tag, className, text) {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function ordinal(day) {
  const tens = day % 100;
  if (tens >= 11 && tens <= 13) return `${day}th`;
  return `${day}${{ 1: "st", 2: "nd", 3: "rd" }[day % 10] || "th"}`;
}

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
// writtenDate is a date in EVE time, ie Oct 24th, 2026
function writtenDate(value) {
  const at = typeof value === "string" && value.length === 10 ? new Date(`${value}T00:00:00Z`) : new Date(value);
  return `${MONTHS[at.getUTCMonth()]} ${ordinal(at.getUTCDate())}, ${at.getUTCFullYear()}`;
}

function writtenTime(value) {
  return new Date(value).toISOString().slice(11, 16);
}

function isk(value) {
  const size = Math.abs(value);
  if (size >= 1e12) return `${(value / 1e12).toFixed(2)}T`;
  if (size >= 1e9) return `${(value / 1e9).toFixed(1)}B`;
  if (size >= 1e6) return `${(value / 1e6).toFixed(1)}M`;
  if (size >= 1e3) return `${(value / 1e3).toFixed(0)}K`;
  return String(Math.round(value));
}

function count(value) {
  return value.toLocaleString("en-US");
}

// counted is a number with its noun, ie 1 killmail or 12 killmails
function counted(value, noun) {
  return `${count(value)} ${noun}${value === 1 ? "" : "s"}`;
}

function classLabel(cls) {
  return CLASS_LABELS[cls] || cls || "";
}

// classChip is a system's class or security band in the colors the app gives them
function classChip(cls) {
  const label = classLabel(cls);
  return make("span", `class-${label}`, label);
}

function sideName(key) {
  return key === "a" ? META.sideA : key === "b" ? META.sideB : "Others";
}

const MARK_STATES = { destroyed: "Destroyed", reffed: "Ref'd", attacked: "Attacked", unsure: "Attacked?" };

// markLabel is a battle's station chit, ie Citadel Attacked?
function markLabel(mark) {
  return `${mark.title} ${MARK_STATES[mark.state]}`;
}

// markWhy says what a station chit means for this battle, in the words of the app's Battles tab
function markWhy(mark) {
  const name = sideName(mark.side);
  return {
    destroyed: `A ${mark.word} from ${name} was destroyed in this fight`,
    reffed: `A ${mark.word} from ${name} was attacked here - it was destroyed later on ${writtenDate(mark.at)}`,
    attacked: `A ${mark.word} from ${name} was probably attacked here.`,
    unsure: `A ${mark.word} from ${name} may have been attacked - too few killmails to be certain`,
  }[mark.state];
}

// markChip is a station chit in its side's color, what marked it on hover
function markChip(mark) {
  const chip = make("span", `chit mark-${mark.side} mark-${mark.state}`, markLabel(mark));
  chip.title = [markWhy(mark), ...mark.notes].join("\n");
  return chip;
}

// MARK_KEY says what the station chits mean, as the app's own key does
const MARK_KEY = "Destroyed: a side's structure of that kind died in this fight. Ref'd: a side's Standup Fighters or citadel in a fight, then its citadel there died when one of its reinforcement timers would have ended. Attacked: a side's Standup Fighters, or a structure of its on a killmail with 5 or more pilots, with none of them lost there on its timers, so it was probably attacked. Attacked?: a side's structure fired, with no fighters, only on killmails with fewer than 5 pilots, so it may have been attacked, or just been shooting a lone ship.";

// TABS holds what draws each tab, added by the scripts after this one
const TABS = {};

/*
  startPage lays out the header, tabs, and footer, then opens the tab the address names, or the first.
  A tab is drawn the first time it opens, as Plotly can not size a chart in a hidden tab.
*/
function startPage() {
  const app = document.getElementById("app");
  const title = make("div", "title-row");
  title.append(make("h1", "", META.name),
    make("p", "export-notice", `Exported ${writtenDate(META.exported)} with GoSleuth v${META.version}. All times are EVE time.`));
  app.append(title);
  const versus = make("div", "versus");
  versus.append(make("span", "key key-a"), META.sideA, " vs ", make("span", "key key-b"), META.sideB,
    ` · ${META.fromDate ? writtenDate(META.fromDate) : "No start"} → ${META.until ? writtenDate(META.until) : "Still going"}`);
  app.append(versus);

  const nav = make("nav");
  nav.setAttribute("aria-label", "This report");
  const content = make("main");
  const links = {}, sections = {};
  for (const tab of META.tabs) {
    links[tab.id] = make("a", "", tab.label);
    links[tab.id].href = `#${tab.id}`;
    nav.append(links[tab.id]);
    sections[tab.id] = make("section");
    sections[tab.id].hidden = true;
    content.append(sections[tab.id]);
  }
  const json = make("a", "", "JSON");
  json.href = encodeURIComponent(META.jsonFile);
  nav.append(json);
  app.append(nav, content);

  const footer = make("footer");
  footer.append(make("p", "", META.caveat));
  app.append(footer);

  const drawn = new Set();
  const show = () => {
    const wanted = location.hash.slice(1);
    const id = META.tabs.some((tab) => tab.id === wanted) ? wanted : META.tabs[0].id;
    for (const tab of META.tabs) {
      sections[tab.id].hidden = tab.id !== id;
      if (tab.id === id) links[tab.id].setAttribute("aria-current", "page");
      else links[tab.id].removeAttribute("aria-current");
    }
    if (!drawn.has(id)) {
      drawn.add(id);
      TABS[id](sections[id], DATA.body[id]);
    } else {
      // the window may have changed size while it was hidden
      sections[id].querySelectorAll(".js-plotly-plot").forEach((plot) => Plotly.Plots.resize(plot));
    }
  };
  window.addEventListener("hashchange", show);
  show();
}

// icon is a logo or ship from the image server, which falls away if it can not be loaded
function icon(address) {
  const image = make("img", "icon");
  image.alt = "";
  image.loading = "lazy";
  image.referrerPolicy = "no-referrer";
  image.addEventListener("error", () => image.remove());
  if (address) image.src = address;
  return image;
}

// named is a name with its icon before it, when it has one
function named(address, name) {
  const holder = make("span", "named");
  if (address) holder.append(icon(address));
  holder.append(name);
  return holder;
}

// table builds a table from its heads and rows of cells, each a string or a node. A head or cell starting "#" is a number.
function table(heads, rows) {
  const node = make("table");
  const head = make("tr");
  for (const text of heads) head.append(make("th", text.startsWith("#") ? "num" : "", text.replace(/^#/, "")));
  const thead = make("thead");
  thead.append(head);
  const body = make("tbody");
  for (const cells of rows) {
    const row = make("tr");
    cells.forEach((cell, index) => {
      const td = make("td", heads[index].startsWith("#") ? "num" : "");
      td.append(cell);
      row.append(td);
    });
    body.append(row);
  }
  node.append(thead, body);
  const wrap = make("div", "scroll");
  wrap.append(node);
  return wrap;
}

// tableView is a chart's figures as a table, closed until asked for
function tableView(title, node) {
  const details = make("details");
  details.append(make("summary", "", title), node);
  return details;
}

// plotLayout is the charts' shared dark look, with a quiet grid and the legend above the plot
function plotLayout(extra) {
  const axis = { gridcolor: COLORS.border, zerolinecolor: COLORS.border, linecolor: COLORS.border, automargin: true };
  return {
    paper_bgcolor: COLORS.panel,
    plot_bgcolor: COLORS.panel,
    font: { color: COLORS.text, family: '"Segoe UI", system-ui, sans-serif', size: 12 },
    margin: { l: 56, r: 24, t: 56, b: 44 },
    legend: { orientation: "h", x: 0, y: 1.02, yanchor: "bottom" },
    hoverlabel: { align: "left", bgcolor: "#1a212c", bordercolor: COLORS.border, font: { color: COLORS.text } },
    ...extra,
    xaxis: { ...axis, ...extra.xaxis },
    yaxis: { ...axis, ...extra.yaxis },
  };
}

// countAxis keeps an axis of small counts on whole numbers
function countAxis(values) {
  return { rangemode: "tozero", tickformat: ",d", ...(Math.max(...values) < 5 ? { dtick: 1 } : {}) };
}

const PLOT_CONFIG = { displaylogo: false, responsive: true, modeBarButtonsToRemove: ["select2d", "lasso2d", "autoScale2d", "toImage"] };

// chart draws a Plotly chart in a new panel in the holder and returns its node
function chart(holder, traces, layout) {
  const panel = make("div", "panel chart");
  const node = make("div");
  panel.append(node);
  holder.append(panel);
  Plotly.newPlot(node, traces, plotLayout(layout), PLOT_CONFIG);
  return node;
}
