/**
 * Flappy Bird — Node.js Web Server
 * Express + static files + leaderboard REST API
 */
'use strict';

const express = require('express');
const path    = require('path');

const app  = express();
const PORT = process.env.PORT || 3000;

// ── Middleware ────────────────────────────────────────────────────────────────
app.use(express.json());
app.use(express.static(path.join(__dirname, 'public')));

// ── In-memory leaderboard (top 10) ───────────────────────────────────────────
const leaderboard = [
  { name: 'ACE',  score: 42 },
  { name: 'ZAP',  score: 35 },
  { name: 'NXS',  score: 28 },
  { name: 'REX',  score: 21 },
  { name: 'VXL',  score: 17 },
];

// GET /api/leaderboard
app.get('/api/leaderboard', (req, res) => {
  const sorted = [...leaderboard]
    .sort((a, b) => b.score - a.score)
    .slice(0, 10);
  res.json(sorted);
});

// POST /api/leaderboard  { name: "AAA", score: 99 }
app.post('/api/leaderboard', (req, res) => {
  const { name, score } = req.body;
  if (!name || typeof score !== 'number') {
    return res.status(400).json({ error: 'name and score required' });
  }
  const entry = {
    name: String(name).toUpperCase().slice(0, 3).padEnd(3, '_'),
    score: Math.floor(score),
    date: new Date().toISOString(),
  };
  leaderboard.push(entry);
  leaderboard.sort((a, b) => b.score - a.score);
  if (leaderboard.length > 50) leaderboard.length = 50; // cap memory
  res.json({ ok: true, entry });
});

// GET /api/status — health check
app.get('/api/status', (req, res) => {
  res.json({
    status: 'online',
    uptime: Math.floor(process.uptime()),
    node: process.version,
    players: leaderboard.length,
  });
});

// ── Start ─────────────────────────────────────────────────────────────────────
app.listen(PORT, () => {
  console.log('');
  console.log('  ██████╗ ██╗      █████╗ ██████╗ ██████╗ ██╗   ██╗');
  console.log(' ██╔════╝ ██║     ██╔══██╗██╔══██╗██╔══██╗╚██╗ ██╔╝');
  console.log(' █████╗   ██║     ███████║██████╔╝██████╔╝  ╚████╔╝ ');
  console.log(' ██╔══╝   ██║     ██╔══██║██╔═══╝ ██╔═══╝    ╚██╔╝  ');
  console.log(' ██║      ███████╗██║  ██║██║     ██║         ██║   ');
  console.log(' ╚═╝      ╚══════╝╚═╝  ╚═╝╚═╝     ╚═╝         ╚═╝   ');
  console.log('');
  console.log(`  🐦  Server running → http://localhost:${PORT}`);
  console.log('');
});
