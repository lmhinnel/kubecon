/**
 * FLAPPY BIRD — UI Controller
 * Manages overlays, leaderboard API calls, name entry, server status
 */
'use strict';

(function () {

// ─── Elements ─────────────────────────────────────────────────────────────────
const overlayTitle = document.getElementById('overlayTitle');
const overlayDead  = document.getElementById('overlayDead');
const liveScore    = document.getElementById('liveScore');
const deadScore    = document.getElementById('deadScore');
const deadBest     = document.getElementById('deadBest');
const nameEntry    = document.getElementById('nameEntry');
const retryHint    = document.getElementById('retryHint');
const lbEl         = document.getElementById('leaderboard');
const sStatus      = document.getElementById('sStatus');
const sNode        = document.getElementById('sNode');
const sUptime      = document.getElementById('sUptime');

// ─── Name entry state ─────────────────────────────────────────────────────────
const CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_';
let nameChars = [0, 0, 0]; // indices into CHARS
let activeCursor = 0;
let nameMode = false;
let lastScore = 0;

function getNameStr() {
  return nameChars.map(i => CHARS[i]).join('');
}

function renderNameChars() {
  [0, 1, 2].forEach(i => {
    const el = document.getElementById(`nc${i}`);
    el.textContent = CHARS[nameChars[i]];
    el.classList.toggle('active', i === activeCursor);
  });
}

function enterNameMode(score) {
  lastScore = score;
  nameChars = [0, 0, 0];
  activeCursor = 0;
  nameMode = true;
  nameEntry.classList.remove('hidden');
  retryHint.classList.add('hidden');
  renderNameChars();
}

function exitNameMode() {
  nameMode = false;
  nameEntry.classList.add('hidden');
  retryHint.classList.remove('hidden');
}

async function submitScore(name, score) {
  try {
    await fetch('/api/leaderboard', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, score }),
    });
    await loadLeaderboard(name);
  } catch (e) {
    console.warn('Leaderboard submit failed:', e);
  }
}

// ─── Keyboard handler for name entry ─────────────────────────────────────────
document.addEventListener('keydown', e => {
  if (nameMode) {
    if (e.code === 'ArrowLeft') {
      activeCursor = Math.max(0, activeCursor - 1);
      renderNameChars(); return;
    }
    if (e.code === 'ArrowRight') {
      activeCursor = Math.min(2, activeCursor + 1);
      renderNameChars(); return;
    }
    if (e.code === 'ArrowUp') {
      nameChars[activeCursor] = (nameChars[activeCursor] + 1) % CHARS.length;
      renderNameChars(); return;
    }
    if (e.code === 'ArrowDown') {
      nameChars[activeCursor] = (nameChars[activeCursor] - 1 + CHARS.length) % CHARS.length;
      renderNameChars(); return;
    }
    if (e.code === 'Enter' || e.code === 'Space') {
      e.preventDefault();
      const name = getNameStr();
      submitScore(name, lastScore);
      exitNameMode();
      return;
    }
    // Typing a letter directly
    if (e.key.length === 1) {
      const ch = e.key.toUpperCase();
      const idx = CHARS.indexOf(ch);
      if (idx >= 0) {
        nameChars[activeCursor] = idx;
        activeCursor = Math.min(2, activeCursor + 1);
        renderNameChars();
      }
    }
    return; // block all other keys during name entry
  }

  // Retry from dead screen
  if (e.code === 'Space' || e.code === 'ArrowUp') {
    if (!overlayDead.classList.contains('hidden')) {
      e.preventDefault();
      startRetry();
    }
  }
});

// Clicking name chars
[0,1,2].forEach(i => {
  document.getElementById(`nc${i}`).addEventListener('click', () => {
    activeCursor = i;
    renderNameChars();
  });
});

// ─── Game events ──────────────────────────────────────────────────────────────
window.addEventListener('flappy:start', () => {
  overlayTitle.classList.add('hidden');
  overlayDead.classList.add('hidden');
});

window.addEventListener('flappy:score', e => {
  liveScore.textContent = e.detail;
});

window.addEventListener('flappy:scored', e => {
  // score ping animation
  liveScore.style.transform = 'scale(1.4)';
  setTimeout(() => liveScore.style.transform = '', 200);
});

window.addEventListener('flappy:dead', e => {
  const { score, best } = e.detail;
  deadScore.textContent = score;
  deadBest.textContent  = best;
  overlayDead.classList.remove('hidden');

  // Show name entry for scores > 0
  if (score > 0) {
    setTimeout(() => enterNameMode(score), 600);
  }
});

function startRetry() {
  liveScore.textContent = '0';
  exitNameMode();
  overlayDead.classList.add('hidden');
  window.dispatchEvent(new CustomEvent('flappy:restart'));
}

// Retry button on dead screen (tap/click the retry hint)
retryHint.addEventListener('click', startRetry);

// ─── Background star canvas ───────────────────────────────────────────────────
(function initBgStars() {
  const bg = document.getElementById('bgCanvas');
  const bx = bg.getContext('2d');

  function resize() {
    bg.width  = window.innerWidth;
    bg.height = window.innerHeight;
  }
  resize();
  window.addEventListener('resize', resize);

  const stars = Array.from({ length: 120 }, () => ({
    x: Math.random(),
    y: Math.random(),
    r: Math.random() * 1.5 + 0.3,
    ph: Math.random() * Math.PI * 2,
    spd: 0.005 + Math.random() * 0.015,
  }));

  function drawBg() {
    bx.clearRect(0, 0, bg.width, bg.height);
    stars.forEach(s => {
      s.ph += s.spd;
      const alpha = 0.15 + 0.3 * Math.abs(Math.sin(s.ph));
      bx.globalAlpha = alpha;
      bx.fillStyle = '#39ff14';
      bx.fillRect(
        Math.floor(s.x * bg.width),
        Math.floor(s.y * bg.height),
        s.r > 1 ? 2 : 1,
        s.r > 1 ? 2 : 1
      );
    });
    bx.globalAlpha = 1;
    requestAnimationFrame(drawBg);
  }
  drawBg();
})();

// ─── Leaderboard API ──────────────────────────────────────────────────────────
async function loadLeaderboard(highlightName) {
  try {
    const res  = await fetch('/api/leaderboard');
    const data = await res.json();

    if (!data.length) {
      lbEl.innerHTML = '<div class="lb-loading">NO SCORES YET</div>';
      return;
    }

    lbEl.innerHTML = data.slice(0, 8).map((entry, i) => {
      const isNew = highlightName && entry.name === highlightName;
      const rank  = i === 0 ? '🥇' : i === 1 ? '🥈' : i === 2 ? '🥉' : `${i + 1}.`;
      const cls   = ['top1', 'top2', 'top3'][i] || '';
      return `
        <div class="lb-row ${cls} ${isNew ? 'lb-new' : ''}">
          <span class="lb-rank">${rank}</span>
          <span class="lb-name">${entry.name}</span>
          <span class="lb-score">${entry.score}</span>
        </div>`;
    }).join('');
  } catch (e) {
    lbEl.innerHTML = '<div class="lb-loading">OFFLINE</div>';
  }
}

// ─── Server Status API ────────────────────────────────────────────────────────
async function loadServerStatus() {
  try {
    const res  = await fetch('/api/status');
    const data = await res.json();
    sStatus.textContent = '● ONLINE';
    sStatus.style.color = '#39ff14';
    sNode.textContent   = data.node;
    sUptime.textContent = formatUptime(data.uptime);
  } catch (e) {
    sStatus.textContent = '● OFFLINE';
    sStatus.style.color = '#ff2d2d';
  }
}

function formatUptime(secs) {
  if (secs < 60)  return `${secs}s`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m`;
  return `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`;
}

// ─── Boot ─────────────────────────────────────────────────────────────────────
loadLeaderboard();
loadServerStatus();
setInterval(loadServerStatus, 15000);
setInterval(loadLeaderboard, 30000);

})();
