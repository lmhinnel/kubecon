# 🐦 Flappy Bird — Node.js Retro Arcade Website

A full retro arcade website serving a playable Flappy Bird game, built with **Node.js + Express**.

```
  ┌─────────────────────────────────────────────────────┐
  │  ★ INSERT COIN ★  FLAPPY BIRD  ★ HIGH SCORE ★       │  ← scrolling marquee
  ├──────────────┬──────────────────┬────────────────────┤
  │              │   ┌──────────┐   │                    │
  │  LEADERBOARD │   │  GAME    │   │  HOW TO PLAY       │
  │              │   │  CANVAS  │   │                    │
  │  SERVER INFO │   │          │   │  MEDALS            │
  │              │   └──────────┘   │                    │
  │              │   SCORE: 000     │  POWERED BY        │
  └──────────────┴──────────────────┴────────────────────┘
  │  © 2025 FLAPPY BIRD — NODE.JS RETRO ARCADE EDITION   │
  └───────────────────────────────────────────────────────┘
```

## Stack

| Layer    | Technology |
|----------|-----------|
| Backend  | Node.js + Express |
| Frontend | Vanilla JS + Canvas API |
| Styling  | CSS (Press Start 2P pixel font) |
| API      | REST `/api/leaderboard` + `/api/status` |
| Storage  | In-memory (no DB needed) |

## Setup

```bash
npm install
npm start
```

Then open → **http://localhost:3000**

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/api/leaderboard` | Top 10 scores |
| `POST` | `/api/leaderboard` | Submit `{ name, score }` |
| `GET`  | `/api/status` | Server health check |

## Controls

| Key | Action |
|-----|--------|
| `SPACE` / `↑` | Flap |
| `← →` | Navigate name entry |
| `↑ ↓` | Change letter in name entry |
| `ENTER` | Save name to leaderboard |

## Features

- 🎮 Pixel-art retro arcade aesthetic with CRT scanline overlay
- 🏆 Live leaderboard with name entry (3-character arcade style)
- 🌐 Server status panel (Node version, uptime)
- 💥 Particle burst on death
- ⭐ Parallax star field background
- 📱 Mobile touch support
- 🔊 Score flash animation on points
