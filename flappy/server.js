/**
 * Flappy Bird — Node.js Web Server
 * Express + static files + SQLite leaderboard REST API
 *
 * DB path: /data/leaderboard.db  (PVC mount from Knative YAML)
 * Falls back to ./leaderboard.db for local dev when /data is not mounted.
 */
"use strict";

const express = require("express");
const path = require("path");
const fs = require("fs");
const Database = require("better-sqlite3");

const app = express();
const PORT = process.env.PORT || 3000;

// ── SQLite setup ──────────────────────────────────────────────────────────────
// Use /data (PVC) in production, local fallback for dev
const DATA_DIR = fs.existsSync("/data") ? "/data" : __dirname + "/data";
const DB_PATH = path.join(DATA_DIR, "leaderboard.db");

console.log(`  📂  Database → ${DB_PATH}`);

const db = new Database(DB_PATH);

// WAL mode for better concurrent read performance
db.pragma("journal_mode = WAL");
db.pragma("synchronous = NORMAL");

// Schema
db.exec(`
  CREATE TABLE IF NOT EXISTS scores (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    name      TEXT    NOT NULL,
    score     INTEGER NOT NULL,
    created_at TEXT   NOT NULL DEFAULT (datetime('now'))
  );
  CREATE INDEX IF NOT EXISTS idx_score ON scores (score DESC);
`);

// Seed default scores only if table is empty
const count = db.prepare("SELECT COUNT(*) as n FROM scores").get();
if (count.n === 0) {
  const insert = db.prepare("INSERT INTO scores (name, score) VALUES (?, ?)");
  const seed = db.transaction(() => {
    [
      ["ACE", 42],
      ["ZAP", 35],
      ["NXS", 28],
      ["REX", 21],
      ["VXL", 17],
    ].forEach(([name, score]) => insert.run(name, score));
  });
  seed();
}

// Prepared statements
const stmtTop = db.prepare(
  "SELECT name, score, created_at as date FROM scores ORDER BY score DESC LIMIT 10",
);
const stmtInsert = db.prepare(
  "INSERT INTO scores (name, score) VALUES (?, ?) RETURNING name, score, created_at as date",
);
const stmtCount = db.prepare("SELECT COUNT(*) as n FROM scores");

// ── Middleware ────────────────────────────────────────────────────────────────
app.use(express.json());
app.use(express.static(path.join(__dirname, "public")));

// ── GET /api/leaderboard ──────────────────────────────────────────────────────
app.get("/api/leaderboard", (req, res) => {
  try {
    const rows = stmtTop.all();
    res.json(rows);
  } catch (err) {
    console.error("leaderboard read error:", err);
    res.status(500).json({ error: "db error" });
  }
});

// ── POST /api/leaderboard  { name: "AAA", score: 99 } ─────────────────────────
app.post("/api/leaderboard", (req, res) => {
  const { name, score } = req.body;
  if (!name || typeof score !== "number") {
    return res.status(400).json({ error: "name and score required" });
  }
  const safeName = String(name)
    .toUpperCase()
    .replace(/[^A-Z0-9_]/g, "_")
    .slice(0, 3)
    .padEnd(3, "_");
  const safeScore = Math.max(0, Math.floor(score));
  try {
    const entry = stmtInsert.get(safeName, safeScore);
    res.json({ ok: true, entry });
  } catch (err) {
    console.error("leaderboard insert error:", err);
    res.status(500).json({ error: "db error" });
  }
});

// ── GET /api/status ───────────────────────────────────────────────────────────
app.get("/api/status", (req, res) => {
  try {
    const { n } = stmtCount.get();
    res.json({
      status: "online",
      uptime: Math.floor(process.uptime()),
      node: process.version,
      db: DB_PATH,
      players: n,
    });
  } catch (err) {
    res.status(500).json({ error: "db error" });
  }
});

// ── Graceful shutdown (important for SQLite WAL flush) ────────────────────────
function shutdown(signal) {
  console.log(`\n  [${signal}] Closing SQLite…`);
  db.close();
  process.exit(0);
}
process.on("SIGTERM", () => shutdown("SIGTERM"));
process.on("SIGINT", () => shutdown("SIGINT"));

// ── Start ─────────────────────────────────────────────────────────────────────
app.listen(PORT, () => {
  console.log("");
  console.log("  ██████╗ ██╗      █████╗ ██████╗ ██████╗ ██╗   ██╗");
  console.log(" ██╔════╝ ██║     ██╔══██╗██╔══██╗██╔══██╗╚██╗ ██╔╝");
  console.log(" █████╗   ██║     ███████║██████╔╝██████╔╝  ╚████╔╝ ");
  console.log(" ██╔══╝   ██║     ██╔══██║██╔═══╝ ██╔═══╝    ╚██╔╝  ");
  console.log(" ██║      ███████╗██║  ██║██║     ██║         ██║   ");
  console.log(" ╚═╝      ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝         ╚═╝   ");
  console.log("");
  console.log(`  🐦  Server running → http://localhost:${PORT}`);
  console.log(`  🗄️   SQLite        → ${DB_PATH}`);
  console.log("");
});
