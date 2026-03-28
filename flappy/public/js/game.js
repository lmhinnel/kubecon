/**
 * FLAPPY BIRD — Canvas Game Engine
 * Retro pixel-art style renderer
 */
'use strict';

(function () {

const canvas = document.getElementById('game');
const ctx    = canvas.getContext('2d');
const W = canvas.width;   // 320
const H = canvas.height;  // 480

// ─── Palette (CGA/NES inspired) ───────────────────────────────────────────────
const PAL = {
  skyTop:    '#020d18',
  skyBot:    '#041c30',
  starA:     '#c8ffc8',
  starB:     '#39ff14',
  ground:    '#0d2a08',
  groundTop: '#1a4a10',
  grassA:    '#226614',
  grassB:    '#2d8a1a',
  pipeBody:  '#0a3a10',
  pipeLit:   '#39ff14',
  pipeDark:  '#052008',
  birdY:     '#ffe600',
  birdO:     '#ff8c00',
  birdW:     '#ffffff',
  birdE:     '#000000',
  birdB:     '#ff4500',
  scoreGlow: '#39ff14',
};

// ─── Config ───────────────────────────────────────────────────────────────────
const GND_H   = 48;
const PLAY_H  = H - GND_H;
const GRAVITY = 0.35;
const FLAP    = -6.5;
const BIRD_X  = 70;
const PIPE_W  = 36;
const PIPE_GAP= 110;
const PIPE_SPD= 2.2;
const PIPE_INT= 120; // frames between spawns

// ─── State ────────────────────────────────────────────────────────────────────
let phase  = 'title'; // title | playing | dead
let bird   = {};
let pipes  = [];
let score  = 0;
let best   = parseInt(localStorage.getItem('flappy_best') || '0', 10);
let frame  = 0;
let ptimer = 0;
let stars  = [];
let clouds = [];
let particles = [];

// ─── Init ─────────────────────────────────────────────────────────────────────
function initStars() {
  stars = Array.from({ length: 35 }, () => ({
    x: Math.random() * W,
    y: Math.random() * (PLAY_H * 0.7),
    r: Math.random() < 0.3 ? 1.5 : 1,
    phase: Math.random() * Math.PI * 2,
  }));
}

function initClouds() {
  clouds = Array.from({ length: 4 }, (_, i) => ({
    x: i * 90 + 20,
    y: 20 + Math.random() * 80,
    w: 60 + Math.random() * 50,
    spd: 0.2 + Math.random() * 0.2,
  }));
}

function resetBird() {
  bird = { y: PLAY_H / 2, vy: 0, angle: 0, wingPhase: 0, hitFlash: 0 };
}

function resetGame() {
  resetBird();
  pipes  = [];
  score  = 0;
  frame  = 0;
  ptimer = 0;
  particles = [];
  spawnPipe();
  // notify UI
  window.dispatchEvent(new CustomEvent('flappy:score', { detail: 0 }));
}

function spawnPipe() {
  const gapY = 40 + Math.random() * (PLAY_H - PIPE_GAP - 60);
  pipes.push({ x: W + PIPE_W, gapY, passed: false });
}

// ─── Particles ────────────────────────────────────────────────────────────────
function burst(x, y) {
  for (let i = 0; i < 14; i++) {
    const angle = (Math.PI * 2 * i) / 14 + Math.random() * 0.3;
    particles.push({
      x, y,
      vx: Math.cos(angle) * (2 + Math.random() * 3),
      vy: Math.sin(angle) * (2 + Math.random() * 3),
      life: 1,
      color: [PAL.birdY, PAL.birdO, PAL.birdB, '#fff'][Math.floor(Math.random() * 4)],
    });
  }
}

// ─── Draw helpers ─────────────────────────────────────────────────────────────
function px(x, y, size, color) {
  ctx.fillStyle = color;
  ctx.fillRect(Math.floor(x), Math.floor(y), size, size);
}

function pixelRect(x, y, w, h, color) {
  ctx.fillStyle = color;
  ctx.fillRect(Math.floor(x), Math.floor(y), w, h);
}

function glowText(text, x, y, size, color, glowColor) {
  ctx.save();
  ctx.font = `${size}px "Press Start 2P", monospace`;
  ctx.textAlign = 'center';
  ctx.shadowColor = glowColor || color;
  ctx.shadowBlur = 12;
  ctx.fillStyle = color;
  ctx.fillText(text, x, y);
  ctx.shadowBlur = 0;
  ctx.restore();
}

// ─── Draw Sky + Stars ─────────────────────────────────────────────────────────
function drawSky() {
  const grad = ctx.createLinearGradient(0, 0, 0, PLAY_H);
  grad.addColorStop(0, PAL.skyTop);
  grad.addColorStop(1, PAL.skyBot);
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, W, PLAY_H);

  // Stars
  stars.forEach(s => {
    s.phase += 0.02;
    const alpha = 0.4 + 0.6 * Math.abs(Math.sin(s.phase));
    ctx.globalAlpha = alpha;
    ctx.fillStyle = s.r > 1 ? PAL.starB : PAL.starA;
    ctx.fillRect(Math.floor(s.x), Math.floor(s.y), s.r > 1 ? 2 : 1, s.r > 1 ? 2 : 1);
    ctx.globalAlpha = 1;
  });
}

// ─── Draw Clouds ──────────────────────────────────────────────────────────────
function drawClouds() {
  clouds.forEach(c => {
    if (phase === 'playing') c.x -= c.spd;
    if (c.x + c.w < 0) { c.x = W + c.w; c.y = 20 + Math.random() * 80; }

    ctx.globalAlpha = 0.12;
    ctx.fillStyle = '#88ffcc';
    // simple 3-bubble cloud
    ctx.beginPath();
    ctx.arc(c.x + c.w * 0.3, c.y + 10, c.w * 0.22, 0, Math.PI * 2);
    ctx.arc(c.x + c.w * 0.6, c.y + 6, c.w * 0.28, 0, Math.PI * 2);
    ctx.arc(c.x + c.w * 0.85, c.y + 11, c.w * 0.18, 0, Math.PI * 2);
    ctx.fill();
    ctx.globalAlpha = 1;
  });
}

// ─── Draw Pipe ────────────────────────────────────────────────────────────────
function drawPipe(p) {
  const topH  = p.gapY;
  const botY  = p.gapY + PIPE_GAP;
  const botH  = PLAY_H - botY;
  const capW  = PIPE_W + 8;
  const capH  = 16;
  const capX  = p.x - 4;

  function drawBody(x, y, w, h) {
    // Dark body
    pixelRect(x, y, w, h, PAL.pipeBody);
    // Left highlight strip
    pixelRect(x + 3, y, 4, h, PAL.pipeLit);
    // Right shadow strip
    pixelRect(x + w - 5, y, 3, h, PAL.pipeDark);
    // Glow edge
    ctx.save();
    ctx.shadowColor = PAL.pipeLit;
    ctx.shadowBlur = 8;
    ctx.strokeStyle = PAL.pipeLit;
    ctx.lineWidth = 1;
    ctx.strokeRect(x + 0.5, y + 0.5, w - 1, h - 1);
    ctx.restore();
  }

  function drawCap(x, y, w, h) {
    pixelRect(x, y, w, h, PAL.pipeBody);
    pixelRect(x + 3, y, 5, h, PAL.pipeLit);
    pixelRect(x + w - 5, y, 4, h, PAL.pipeDark);
    ctx.save();
    ctx.shadowColor = PAL.pipeLit;
    ctx.shadowBlur = 12;
    ctx.strokeStyle = PAL.pipeLit;
    ctx.lineWidth = 1.5;
    ctx.strokeRect(x + 0.5, y + 0.5, w - 1, h - 1);
    ctx.restore();
  }

  // Top pipe
  drawBody(p.x, 0, PIPE_W, topH);
  drawCap(capX, topH - capH, capW, capH);

  // Bottom pipe
  drawCap(capX, botY, capW, capH);
  drawBody(p.x, botY + capH, PIPE_W, botH - capH);
}

// ─── Draw Bird ────────────────────────────────────────────────────────────────
function drawBird() {
  const { y, angle, wingPhase, hitFlash } = bird;
  const bx = BIRD_X, by = Math.floor(y);
  const W2 = 22, H2 = 18;

  ctx.save();
  ctx.translate(bx, by);
  ctx.rotate(Math.max(-0.45, Math.min(1.1, angle)));

  // Glow
  ctx.shadowColor = hitFlash > 0 ? '#ff0000' : PAL.birdY;
  ctx.shadowBlur  = hitFlash > 0 ? 20 : 10;

  // Body (pixelated ellipse via rects)
  const bodyColor = hitFlash > 0 ? '#ff4444' : PAL.birdY;
  ctx.fillStyle = bodyColor;
  // Core body
  ctx.fillRect(-W2/2 + 2, -H2/2 + 2, W2 - 4, H2 - 4);
  ctx.fillRect(-W2/2, -H2/2 + 4, W2, H2 - 8);
  ctx.fillRect(-W2/2 + 4, -H2/2, W2 - 8, H2);

  // Belly highlight
  ctx.fillStyle = '#fff8a0';
  ctx.fillRect(-W2/2 + 4, -1, 6, 5);

  // Wing (flaps up/down)
  const wOff = Math.sin(wingPhase) > 0 ? -3 : 2;
  ctx.fillStyle = PAL.birdO;
  ctx.fillRect(-4, wOff, 10, 5);
  ctx.fillRect(-6, wOff + 2, 6, 3);

  // Eye white
  ctx.shadowBlur = 0;
  ctx.fillStyle = PAL.birdW;
  ctx.fillRect(W2/2 - 9, -H2/2 + 2, 8, 8);
  // Pupil
  ctx.fillStyle = PAL.birdE;
  ctx.fillRect(W2/2 - 6, -H2/2 + 4, 4, 4);
  // Shine
  ctx.fillStyle = '#fff';
  ctx.fillRect(W2/2 - 5, -H2/2 + 5, 2, 2);

  // Beak
  ctx.fillStyle = PAL.birdB;
  ctx.fillRect(W2/2 - 2, -2, 8, 3);
  ctx.fillRect(W2/2 - 2, 1, 7, 3);

  ctx.restore();
}

// ─── Draw Ground ──────────────────────────────────────────────────────────────
function drawGround() {
  const gy = PLAY_H;

  // Main ground
  pixelRect(0, gy, W, GND_H, PAL.ground);

  // Animated grass strips
  for (let x = 0; x < W; x += 4) {
    const h = 3 + Math.sin(x * 0.25 + frame * 0.04) * 1.5;
    ctx.fillStyle = (x / 4 % 2 === 0) ? PAL.grassA : PAL.grassB;
    ctx.fillRect(x, gy, 4, Math.ceil(h));
  }

  // Top border line
  ctx.strokeStyle = PAL.pipeLit;
  ctx.lineWidth = 1;
  ctx.shadowColor = PAL.pipeLit;
  ctx.shadowBlur = 4;
  ctx.beginPath();
  ctx.moveTo(0, gy + 0.5);
  ctx.lineTo(W, gy + 0.5);
  ctx.stroke();
  ctx.shadowBlur = 0;

  // Scrolling dots on ground
  for (let x = (frame * 2) % 12; x < W; x += 12) {
    ctx.fillStyle = PAL.grassB;
    ctx.fillRect(x, gy + 6, 2, 2);
  }
}

// ─── Draw Particles ───────────────────────────────────────────────────────────
function drawParticles() {
  particles.forEach(p => {
    p.x  += p.vx;
    p.y  += p.vy;
    p.vy += 0.2;
    p.life -= 0.04;
    ctx.globalAlpha = Math.max(0, p.life);
    ctx.fillStyle = p.color;
    ctx.fillRect(Math.floor(p.x), Math.floor(p.y), 3, 3);
  });
  ctx.globalAlpha = 1;
  particles = particles.filter(p => p.life > 0);
}

// ─── Draw HUD ─────────────────────────────────────────────────────────────────
function drawHUD() {
  if (phase !== 'playing') return;
  glowText(score.toString(), W / 2, 36, 18, '#ffe600', '#ffe600');
}

// ─── Draw Overlays ────────────────────────────────────────────────────────────
function drawTitleOverlay() {
  // Handled by HTML overlay — just draw a pretty background
}

// ─── Physics & Game Logic ─────────────────────────────────────────────────────
function update() {
  if (phase !== 'playing') return;
  frame++;

  // Bird
  bird.vy += GRAVITY;
  bird.vy = Math.min(bird.vy, 10);
  bird.y += bird.vy;
  bird.angle = bird.vy * 0.09;
  bird.wingPhase += 0.28;
  if (bird.hitFlash > 0) bird.hitFlash--;

  // Ceiling
  if (bird.y < 8) { bird.y = 8; bird.vy = 0; }

  // Ground
  if (bird.y + 12 >= PLAY_H) { bird.y = PLAY_H - 12; die(); return; }

  // Pipes
  ptimer++;
  if (ptimer >= PIPE_INT) { spawnPipe(); ptimer = 0; }

  for (let i = pipes.length - 1; i >= 0; i--) {
    pipes[i].x -= PIPE_SPD;

    // Score when center crosses pipe
    if (!pipes[i].passed && pipes[i].x + PIPE_W < BIRD_X - 10) {
      pipes[i].passed = true;
      score++;
      if (score > best) { best = score; localStorage.setItem('flappy_best', best); }
      window.dispatchEvent(new CustomEvent('flappy:score', { detail: score }));
      window.dispatchEvent(new CustomEvent('flappy:scored', { detail: score }));
    }

    // Remove off-screen
    if (pipes[i].x + PIPE_W + 8 < 0) { pipes.splice(i, 1); continue; }

    // Collision — tight AABB
    const bLeft = BIRD_X - 9, bRight = BIRD_X + 10;
    const bTop  = bird.y - 8,  bBot   = bird.y + 9;
    const pLeft = pipes[i].x - 3, pRight = pipes[i].x + PIPE_W + 3;

    if (bRight > pLeft && bLeft < pRight) {
      if (bTop < pipes[i].gapY || bBot > pipes[i].gapY + PIPE_GAP) {
        die(); return;
      }
    }
  }
}

function die() {
  burst(BIRD_X, bird.y);
  bird.hitFlash = 6;
  phase = 'dead';
  window.dispatchEvent(new CustomEvent('flappy:dead', { detail: { score, best } }));
}

// ─── Render Loop ──────────────────────────────────────────────────────────────
function render() {
  ctx.clearRect(0, 0, W, H);
  drawSky();
  drawClouds();
  pipes.forEach(drawPipe);
  drawParticles();
  drawGround();

  if (phase !== 'title') drawBird();
  drawHUD();

  // Title phase: idle bird bobbing
  if (phase === 'title') {
    bird.y = PLAY_H / 2 + Math.sin(frame * 0.07) * 6;
    bird.wingPhase += 0.15;
    bird.angle = Math.sin(frame * 0.07) * 0.15;
    frame++;
    drawBird();
  }

  requestAnimationFrame(render);
}

// ─── Input ────────────────────────────────────────────────────────────────────
function flap() {
  if (phase === 'playing') {
    bird.vy = FLAP;
    bird.wingPhase = 0;
  } else if (phase === 'title') {
    phase = 'playing';
    resetGame();
    window.dispatchEvent(new CustomEvent('flappy:start'));
  } else if (phase === 'dead') {
    // handled by UI (name entry)
  }
}

document.addEventListener('keydown', e => {
  if (e.code === 'Space' || e.code === 'ArrowUp') { e.preventDefault(); flap(); }
});
canvas.addEventListener('click', flap);
canvas.addEventListener('touchstart', e => { e.preventDefault(); flap(); }, { passive: false });

// External restart trigger
window.addEventListener('flappy:restart', () => {
  phase = 'playing';
  resetGame();
  window.dispatchEvent(new CustomEvent('flappy:start'));
});

// ─── Boot ─────────────────────────────────────────────────────────────────────
initStars();
initClouds();
resetBird();

// Game update loop (separate from render for consistent physics)
setInterval(update, 1000 / 60);

render();

})();
