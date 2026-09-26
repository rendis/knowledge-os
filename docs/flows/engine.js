// knowledge-os flows: a tiny pixel-art engine. Each flow in flows/*.js describes its scene and
// timeline; this file draws the shared cast (agent robots, vault constellation, team cloud,
// status bar) and runs the player UI.
(() => {
  const q = s => document.querySelector(s);
  const qa = s => Array.from(document.querySelectorAll(s));
  const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;

  // ================= canvas and primitives =================
  const W = 224, H = 126, FLOOR_Y = 112;
  const cv = q('#scene'), cx = cv.getContext('2d');
  const off = document.createElement('canvas'); off.width = W; off.height = H;
  const o = off.getContext('2d');
  let P = {}, paletteDirty = true;
  function readPalette() {
    const cs = getComputedStyle(document.documentElement), v = n => cs.getPropertyValue(n).trim();
    P = { K: v('--ink'), W: v('--surface'), B: v('--accent'), b: v('--px-accent2'), S: v('--accent-soft'), R: v('--coral'),
      G: v('--ink-soft'), g: v('--line'), Y: v('--px-gold'), D: v('--px-screen'), C: v('--px-eye'), O: v('--on-accent'),
      bg: v('--px-bg'), grid: v('--grid'),
      // the vault window keeps its night palette in both themes
      N: '#111A2C', L: '#35507A', T: '#DCEBFF' };
  }
  function fit() {
    const w = cv.getBoundingClientRect().width || W;
    const k = Math.max(1, Math.round(w * (devicePixelRatio || 1) / W));
    if (cv.width !== W * k) { cv.width = W * k; cv.height = H * k; }
  }
  const BAYER = [0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5];
  const on = (x, y, a) => a >= 1 || (a > 0 && BAYER[(y & 3) * 4 + (x & 3)] < a * 16);
  function px(x, y, c, a = 1) {
    x = Math.round(x); y = Math.round(y);
    if (x < 0 || y < 0 || x >= W || y >= H || !on(x, y, a)) return;
    o.fillStyle = P[c] || c; o.fillRect(x, y, 1, 1);
  }
  function rect(x, y, w, h, c, a = 1) {
    x = Math.round(x); y = Math.round(y); w = Math.round(w); h = Math.round(h);
    if (w <= 0 || h <= 0) return;
    if (a >= 1) { o.fillStyle = P[c] || c; o.fillRect(x, y, w, h); return; }
    for (let j = 0; j < h; j++) for (let i = 0; i < w; i++) px(x + i, y + j, c, a);
  }
  const box = (x, y, w, h, fill, line = 'K', a = 1) => { rect(x, y, w, h, line, a); rect(x + 1, y + 1, w - 2, h - 2, fill, a); };
  function spr(g, x, y, a = 1, map) {
    if (a <= 0) return;
    x = Math.round(x); y = Math.round(y);
    for (let j = 0; j < g.length; j++) for (let i = 0; i < g[j].length; i++) {
      let c = g[j][i]; if (c === '.') continue;
      if (map && map[c]) c = map[c];
      px(x + i, y + j, c, a);
    }
  }
  function cubic(p0, p1, p2, p3) {
    const out = []; let last;
    for (let i = 0; i <= 240; i++) {
      const t = i / 240, u = 1 - t;
      const x = Math.round(u * u * u * p0[0] + 3 * u * u * t * p1[0] + 3 * u * t * t * p2[0] + t * t * t * p3[0]);
      const y = Math.round(u * u * u * p0[1] + 3 * u * u * t * p1[1] + 3 * u * t * t * p2[1] + t * t * t * p3[1]);
      if (!last || last[0] !== x || last[1] !== y) { out.push([x, y]); last = [x, y]; }
    }
    return out;
  }
  function seg(x0, y0, x1, y1) {
    x0 = Math.round(x0); y0 = Math.round(y0); x1 = Math.round(x1); y1 = Math.round(y1);
    const out = []; let dx = Math.abs(x1 - x0), dy = -Math.abs(y1 - y0), sx = x0 < x1 ? 1 : -1, sy = y0 < y1 ? 1 : -1, e = dx + dy;
    for (;;) { out.push([x0, y0]); if (x0 === x1 && y0 === y1) break; const e2 = 2 * e; if (e2 >= dy) { e += dy; x0 += sx; } if (e2 <= dx) { e += dx; y0 += sy; } }
    return out;
  }
  const link = (a, b, bend = 14) => cubic(a, [a[0] + bend, a[1]], [b[0] - bend, b[1]], b);
  function trace(path, c, prog = 1, a = 1, { dash = 0, shift = 0 } = {}) {
    if (a <= 0 || prog <= 0) return;
    const n = Math.floor(path.length * Math.max(0, Math.min(1, prog)));
    for (let i = 0; i < n; i++) {
      if (dash && ((i + shift) % (dash * 2)) >= dash) continue;
      px(path[i][0], path[i][1], c, a);
    }
  }
  const at = (path, t) => path[Math.min(path.length - 1, Math.max(0, Math.round(t * (path.length - 1))))];

  // ================= 3x5 pixel font =================
  const FONT = {
    A: ['.#.', '#.#', '###', '#.#', '#.#'], B: ['##.', '#.#', '##.', '#.#', '##.'], C: ['.##', '#..', '#..', '#..', '.##'],
    D: ['##.', '#.#', '#.#', '#.#', '##.'], E: ['###', '#..', '##.', '#..', '###'], F: ['###', '#..', '##.', '#..', '#..'],
    G: ['.##', '#..', '#.#', '#.#', '.##'], H: ['#.#', '#.#', '###', '#.#', '#.#'], I: ['###', '.#.', '.#.', '.#.', '###'],
    J: ['..#', '..#', '..#', '#.#', '.#.'], K: ['#.#', '#.#', '##.', '#.#', '#.#'], L: ['#..', '#..', '#..', '#..', '###'],
    M: ['#...#', '##.##', '#.#.#', '#...#', '#...#'], N: ['#..#', '##.#', '#.##', '#..#', '#..#'], O: ['.#.', '#.#', '#.#', '#.#', '.#.'],
    P: ['##.', '#.#', '##.', '#..', '#..'], Q: ['.#.', '#.#', '#.#', '##.', '.##'], R: ['##.', '#.#', '##.', '#.#', '#.#'],
    S: ['.##', '#..', '.#.', '..#', '##.'], T: ['###', '.#.', '.#.', '.#.', '.#.'], U: ['#.#', '#.#', '#.#', '#.#', '###'],
    V: ['#.#', '#.#', '#.#', '#.#', '.#.'], W: ['#...#', '#...#', '#.#.#', '##.##', '#...#'], X: ['#.#', '#.#', '.#.', '#.#', '#.#'],
    Y: ['#.#', '#.#', '.#.', '.#.', '.#.'], Z: ['###', '..#', '.#.', '#..', '###'],
    0: ['###', '#.#', '#.#', '#.#', '###'], 1: ['.#.', '##.', '.#.', '.#.', '###'], 2: ['##.', '..#', '.#.', '#..', '###'],
    3: ['##.', '..#', '.#.', '..#', '##.'], 4: ['#.#', '#.#', '###', '..#', '..#'], 5: ['###', '#..', '##.', '..#', '##.'],
    6: ['.##', '#..', '###', '#.#', '###'], 7: ['###', '..#', '.#.', '.#.', '.#.'], 8: ['###', '#.#', '###', '#.#', '###'], 9: ['###', '#.#', '###', '..#', '##.'],
    ':': ['.', '#', '.', '#', '.'], '!': ['#', '#', '#', '.', '#'], '?': ['##.', '..#', '.#.', '...', '.#.'], '/': ['..#', '..#', '.#.', '#..', '#..'],
    '-': ['...', '...', '###', '...', '...'], '.': ['.', '.', '.', '.', '#'], ',': ['..', '..', '..', '.#', '#.'], '@': ['.##.', '#..#', '#.##', '#...', '.##.'], ' ': ['..', '..', '..', '..', '..'],
    '¿': ['.#.', '...', '.#.', '#..', '.##'], '¡': ['#', '.', '#', '#', '#'],
  };
  // accented capitals: the base letter plus marks in the row above it
  const MARKS = { 'Á': ['A', [[2, -1]]], 'É': ['E', [[2, -1]]], 'Í': ['I', [[2, -1]]], 'Ó': ['O', [[2, -1]]], 'Ú': ['U', [[2, -1]]], 'Ñ': ['N', [[1, -1], [2, -1]]], 'Ü': ['U', [[0, -1], [2, -1]]] };
  const glyph = ch => FONT[MARKS[ch] ? MARKS[ch][0] : ch] || FONT[' '];
  const textW = (s, sc = 1) => [...s].reduce((w, ch) => w + (glyph(ch)[0].length + 1) * sc, 0) - sc;
  function text(s, x, y, c, a = 1, sc = 1) {
    let cx0 = Math.round(x);
    for (const ch of s) {
      const g = glyph(ch);
      for (let r = 0; r < 5; r++) for (let k = 0; k < g[r].length; k++) if (g[r][k] === '#') rect(cx0 + k * sc, y + r * sc, sc, sc, c, a);
      if (MARKS[ch]) MARKS[ch][1].forEach(([mx, my]) => rect(cx0 + mx * sc, y + my * sc, sc, sc, c, a));
      cx0 += (g[0].length + 1) * sc;
    }
  }
  // language: every visible phrase goes through tr(); commands and the request word stay as typed
  let LANG = 'en';
  const COMMON_ES = { VAULT: 'VAULT', TEAM: 'EQUIPO', CODE: 'CÓDIGO', CLOUD: 'NUBE', APPROVED: 'APROBADO', 'READ ONLY': 'SOLO LECTURA' };
  const tr = s => LANG === 'es' ? ((cur && cur.flow.es && cur.flow.es[s]) ?? COMMON_ES[s] ?? s) : s;
  // a flow's own name and request word, before that flow is the current one
  const flowWord = (flow, s) => LANG === 'es' ? ((flow.es && flow.es[s]) ?? s) : s;
  const label = (s, cxm, y, a = 1, c = 'G') => { const t = tr(s); text(t, cxm - textW(t) / 2, y, c, a); };

  // ================= sprites =================
  const disc = (d, ring, fill, marks = []) => {
    const r = d / 2, g = [];
    for (let y = 0; y < d; y++) { const row = []; for (let x = 0; x < d; x++) { const dist = Math.hypot(x + .5 - r, y + .5 - r); row.push(dist <= r - .05 ? (dist > r - 1.5 ? ring : fill) : '.'); } g.push(row); }
    marks.forEach(([x, y, c]) => { g[y][x] = c; });
    return g.map(r => r.join(''));
  };
  const SPR = {
    bot: [
      '.........LL.........', '........LLLL........', '.........KK.........', '...KKKKKKKKKKKKKK...',
      '..KHHHHHHHHHHHHHHK..', '..KHDDDDDDDDDDDDHK..', '..KHDDDDDDDDDDDDHK..', '..KHDDDDDDDDDDDDHK..',
      '..KHDDDDDDDDDDDDHK..', '..KHDDDDDDDDDDDDHK..', '..KHHHHHHHHHHHHHHK..', '...KKKKKKKKKKKKKK...',
      '........KHHK........', '......KKHHHHKK......', '.....KHHHHHHHHK.....', '.....KHHHLLHHHK.....',
      '.....KHHHHHHHHK.....', '......KKKKKKKK......',
    ],
    cloud: [
      '.......KKKK.........', '.....KKWWWWK........', '....KWWWWWWWK.KKK...', '..KKWWWWWWWWWKWWWK..',
      '.KWWWWWWWWWWWWWWWWK.', 'KWWWWWWWWWWWWWWWWWWK', 'KWWWWWWWWWWWWWWWWWWK', '.KWWWWWWWWWWWWWWWWK.', '..KKKKKKKKKKKKKKKK..',
    ],
    ok: ['.BBBBB.', 'BBBBBWB', 'BBBBWBB', 'BWBWBBB', 'BBWBBBB', 'BBBBBBB', '.BBBBB.'],
    bad: ['.RRRRR.', 'RWRRRWR', 'RRWRWRR', 'RRRWRRR', 'RRWRWRR', 'RWRRRWR', '.RRRRR.'],
    warn: ['.RRRRR.', 'RRRWRRR', 'RRRWRRR', 'RRRWRRR', 'RRRRRRR', 'RRRWRRR', '.RRRRR.'],
    changed: ['.BBBBB.', 'BBBWBBB', 'BBWBWBB', 'BWBBBWB', 'BWWWWWB', 'BBBBBBB', '.BBBBB.'],
    fresh: ['.BBBBB.', 'BWWBWWB', 'BWWBWWB', 'BBBBBBB', 'BWWBWWB', 'BWWBWWB', '.BBBBB.'],
    plus: ['SSSSS', 'SSBSS', 'SBBBS', 'SSBSS', 'SSSSS'],
    minus: ['SSSSS', 'SSSSS', 'SRRRS', 'SSSSS', 'SSSSS'],
    lens: ['.KKK...', 'KSSSK..', 'KSWSK..', 'KSSSK..', '.KKK...', '....KK.', '.....KK'],
    lock: ['..KKK..', '.K...K.', '.K...K.', 'KKKKKKK', 'KBBBBBK', 'KBBWBBK', 'KBBBBBK', 'KKKKKKK'],
    ack: ['KKKKKKKKK...', 'KWWWWWWWKK..', 'KWWWWWWWWWK.', 'KWWWWWWWWWWK', 'KWWWWWWWWGWK', 'KWWWWWWWGWWK', 'KWGWWWWGWWWK', 'KWWGWWGWWWWK', 'KWWWGGWWWWWK', 'KKKKKKKKKKKK'],
    iNote: ['KKKKKK.', 'KWWWWK.', 'KWKKWK.', 'KWWWWK.', 'KWKKWK.', 'KWWWWK.', 'KKKKKK.'],
    iTree: ['..KKK..', '..KBK..', '..KKK..', '...K...', '.KKKKK.', '.K...K.', 'KKK.KKK'],
    iClean: ['...K...', '...K...', '..KKK..', 'KKKWKKK', '..KKK..', '...K...', '...K...'],
    iSeal: ['.KKKKK.', 'K.....K', 'K....KK', 'KK..K.K', 'K.KK..K', 'K.....K', '.KKKKK.'],
    iLink: ['.....KK', '....K.K', '...K.K.', '..K.K..', '.K.K...', 'K.K....', 'KK.....'],
    iKey: ['.KKK...', 'K...K..', 'K.K.KKK', 'K...K.K', '.KKK..K', '.......', '.......'],
    db: ['.KKKKKKKKK.', 'KWWWWWWWWWK', 'KKWWWWWWWKK', 'KGKKKKKKKGK', 'KWWWWWWWWWK', 'KKWWWWWWWKK', 'KGKKKKKKKGK', 'KWWWWWWWWWK', 'KKWWWWWWWKK', '.KKKKKKKKK.'],
    ticket: ['KKKKKKKKKKK', 'KWWWWWWWWWK', 'KWBBBWWWWWK', 'KWWWWWWWWWK', '.KWGGGGGWK.', 'KWWWWWWWWWK', 'KWGGGGWWWWK', 'KWWWWWWWWWK', 'KKKKKKKKKKK'],
    laptop: ['..KKKKKKKKKKKKKK..', '..KDDDDDDDDDDDDK..', '..KDCCDDDDDDDDDK..', '..KDDDCCCDDDDDDK..', '..KDCCDDDDDDDDDK..', '..KDDDDDDDDDDDDK..', '..KDDDDDDDDDDDDK..', '..KKKKKKKKKKKKKK..', 'KKKKKKKKKKKKKKKKKK', '.KGGGGGGGGGGGGGGK.'],
    card: ['KKKKKKKKKKKKKKKK', 'KBBBBBBBBBBBBBBK', 'KWWWWWWWWWWWWWWK', 'KWKKKWWGGGGGGWWK', 'KWKKKWWWWWWWWWWK', 'KWKKKWWGGGGWWWWK', 'KWWWWWWWWWWWWWWK', 'KWGGGGGGGGGWWWWK', 'KWWWWWWWWWWWWWWK', 'KKKKKKKKKKKKKKKK'],
    folder: ['KKKKK...........', 'KYYYYK..........', 'KKKKKKKKKKKKKKKK', 'KYYYYYYYYYYYYYYK', 'KYYYYYYYYYYYYYYK', 'KYYYYYYYYYYYYYYK', 'KYYYYYYYYYYYYYYK', 'KYYYYYYYYYYYYYYK', 'KYYYYYYYYYYYYYYK', 'KKKKKKKKKKKKKKKK'],
    box: ['..KKKKKKKKKK..', '.KSSSSSSSSSSK.', 'KKKKKKKKKKKKKK', 'KSSSSSKKSSSSSK', 'KSSSSSKKSSSSSK', 'KSSSSSSSSSSSSK', 'KSSSSSSSSSSSSK', 'KSSSSSSSSSSSSK', 'KKKKKKKKKKKKKK'],
    gear: ['...KKK...', '.K.KBK.K.', '..KBBBK..', 'KKBBWBBKK', 'KBBWWWBBK', 'KKBBWBBKK', '..KBBBK..', '.K.KBK.K.', '...KKK...'],
    file: ['KKKKK..', 'KWWWKK.', 'KWWWWWK', 'KWGGGWK', 'KWWWWWK', 'KWGGGWK', 'KWWWWWK', 'KKKKKKK'],
    cloudS: ['...KKK....', '.KKWWWK.K.', 'KWWWWWWKWK', 'KWWWWWWWWK', '.KKKKKKKK.'],
    iId: ['KKKKKKK', 'KWWWWWK', 'KWKWGGK', 'KWWWWWK', 'KWKWGGK', 'KWWWWWK', 'KKKKKKK'],
    iRepo: ['KKKKKKK', 'KWKWKWK', 'KKKKKKK', 'KWBBWWK', 'KWWGGWK', 'KWBWWWK', 'KKKKKKK'],
    iCloud: ['.......', '..KKK..', '.KWWWK.', 'KWWWWWK', 'KWWWWWK', '.KKKKK.', '.......'],
    iTicket: ['KKKKKKK', 'KWWWWWK', 'KWBBWWK', 'KWWWWWK', 'KWGGGWK', 'KWWWWWK', 'KKKKKKK'],
    iDb: ['.KKKKK.', 'KWWWWWK', 'KKKKKKK', 'KWWWWWK', 'KKKKKKK', 'KWWWWWK', '.KKKKK.'],
    srv: ['KKKKKKKKK', 'KWWWWWWBK', 'KKKKKKKKK', 'KWWWWWWGK', 'KKKKKKKKK', 'KWWWWWWGK', 'KKKKKKKKK', '.K.....K.'],
  };
  SPR.seal = disc(15, 'B', 'S', [[4, 7, 'B'], [5, 8, 'B'], [6, 9, 'B'], [7, 8, 'B'], [8, 7, 'B'], [9, 6, 'B'], [10, 5, 'B'], [4, 8, 'B'], [5, 9, 'B'], [6, 10, 'B'], [7, 9, 'B'], [8, 8, 'B'], [9, 7, 'B'], [10, 6, 'B']]);
  SPR.okBig = disc(11, 'B', 'B', [[3, 5, 'W'], [4, 6, 'W'], [5, 7, 'W'], [6, 6, 'W'], [7, 5, 'W'], [8, 4, 'W']]);
  SPR.empty = disc(7, 'g', 'W');
  const EYES = {
    open: [[6, 6], [7, 6], [6, 7], [7, 7], [6, 8], [7, 8], [12, 6], [13, 6], [12, 7], [13, 7], [12, 8], [13, 8]],
    up: [[6, 5], [7, 5], [6, 6], [7, 6], [6, 7], [7, 7], [12, 5], [13, 5], [12, 6], [13, 6], [12, 7], [13, 7]],
    left: [[5, 6], [6, 6], [5, 7], [6, 7], [5, 8], [6, 8], [11, 6], [12, 6], [11, 7], [12, 7], [11, 8], [12, 8]],
    right: [[7, 6], [8, 6], [7, 7], [8, 7], [7, 8], [8, 8], [13, 6], [14, 6], [13, 7], [14, 7], [13, 8], [14, 8]],
    blink: [[6, 8], [7, 8], [12, 8], [13, 8]],
    happy: [[5, 8], [6, 7], [7, 7], [8, 8], [11, 8], [12, 7], [13, 7], [14, 8]],
    worried: [[6, 6], [7, 7], [6, 8], [13, 6], [12, 7], [13, 8]],
    think: [[6, 7], [7, 7], [12, 7], [13, 7], [13, 5], [14, 5]],
  };
  const ARMS = {
    down: [[4, 14], [3, 15], [3, 16], [15, 14], [16, 15], [16, 16]],
    up: [[4, 14], [3, 13], [2, 12], [15, 14], [16, 13], [17, 12]],
    point: [[4, 14], [3, 15], [3, 16], [15, 14], [16, 14], [17, 14], [18, 13]],
    pointL: [[4, 14], [3, 14], [2, 14], [1, 13], [15, 14], [16, 15], [16, 16]],
    hold: [[4, 14], [3, 14], [2, 14], [15, 14], [16, 15], [16, 16]],
  };
  const FLAME = [
    [[8, 18, 'b'], [9, 18, 'B'], [10, 18, 'B'], [11, 18, 'b'], [9, 19, 'b'], [10, 19, 'B']],
    [[9, 18, 'B'], [10, 18, 'B'], [9, 19, 'B'], [10, 19, 'b'], [10, 20, 'b']],
  ];
  const BOOST = [[8, 18, 'B'], [9, 18, 'B'], [10, 18, 'B'], [11, 18, 'B'], [9, 19, 'B'], [10, 19, 'B'], [9, 20, 'b'], [10, 20, 'b'], [9, 21, 'b'], [10, 22, 'b']];

  // ================= the vault: a constellation in the shape of a brain =================
  const VP = { x: 146, y: 40, w: 76, h: 68 }, BR = { x: 184, y: 72 };
  const rng = (seed => () => { seed |= 0; seed = seed + 0x6D2B79F5 | 0; let t = Math.imul(seed ^ seed >>> 15, 1 | seed); t = t + Math.imul(t ^ t >>> 7, 61 | t) ^ t; return ((t ^ t >>> 14) >>> 0) / 4294967296; })(7);
  const LOBES = [[0, -3, 27, 18], [-15, 3, 14, 13], [-2, 9, 17, 8], [16, 12, 9, 6], [9, 19, 3, 5]];
  const inLobe = (x, y, m = 1) => LOBES.some(([lx, ly, rx, ry]) => ((x - lx) / rx) ** 2 + ((y - ly) / ry) ** 2 <= m);
  const NODES = [], EDGES = [];
  let OUTLINE = 0;
  const far = (x, y, d) => NODES.every(([u, v]) => Math.hypot(u - x, v - y) >= d);
  LOBES.slice(0, 4).forEach(([lx, ly, rx, ry]) => {
    let prev = -1;
    for (let k = 0; k <= 34; k++) {
      const a0 = k / 34 * Math.PI * 2, x = lx + Math.cos(a0) * rx, y = ly + Math.sin(a0) * ry;
      if (inLobe(x, y, .86)) { prev = -1; continue; }
      let i = NODES.findIndex(([u, v]) => Math.hypot(u - x, v - y) < 4);
      if (i < 0) { NODES.push([x, y]); i = NODES.length - 1; }
      if (prev >= 0 && prev !== i) EDGES.push([prev, i]);
      prev = i;
    }
  });
  OUTLINE = NODES.length;
  for (let tries = 0; tries < 4000 && NODES.length < 88; tries++) {
    const x = -30 + rng() * 60, y = -22 + rng() * 46;
    if (inLobe(x, y, .78) && far(x, y, 5.6)) NODES.push([x, y]);
  }
  NODES.forEach(([x, y], i) => NODES.map(([u, v], j) => [j, Math.hypot(u - x, v - y)]).filter(([j, d]) => j !== i && d < 11)
    .sort((p, r) => p[1] - r[1]).slice(0, 3).forEach(([j]) => { if (!EDGES.some(([p, r]) => p === j && r === i)) EDGES.push([i, j]); }));
  // the order in which a young vault fills up
  const RANK = NODES.map((_, i) => [i, rng()]).sort((p, r) => p[1] - r[1]).reduce((acc, [i], k) => { acc[i] = k; return acc; }, []);
  const DEG = NODES.map((_, i) => EDGES.filter(([p, r]) => p === i || r === i).length);
  // stars, as fractions of the window so they follow it when it grows or shrinks
  const STARS = Array.from({ length: 26 }, () => [.04 + rng() * .92, .04 + rng() * .92, rng() * 6])
    .filter(([u, v]) => !inLobe((u - .5) * VP.w, (v - .5) * VP.h, 1.3));
  const nearestNodes = (x, y, n) => NODES.map(([u, v], j) => [j, Math.hypot(u - x, v - y)]).sort((p, r) => p[1] - r[1]).slice(0, n).map(([j]) => j);
  // the vault rests small in the bottom-right corner and grows when it takes the stage
  const MINI = { x: 182, y: 70, w: 40, h: 38 };
  let drift = 0, VR = { ...MINI }, VS = MINI.w / VP.w, VC = [0, 0];
  const setZoom = z => {
    VR = { x: MINI.x + (VP.x - MINI.x) * z, y: MINI.y + (VP.y - MINI.y) * z, w: MINI.w + (VP.w - MINI.w) * z, h: MINI.h + (VP.h - MINI.h) * z };
    VR = { x: Math.round(VR.x), y: Math.round(VR.y), w: Math.round(VR.w), h: Math.round(VR.h) };
    VS = VR.w / VP.w; VC = [VR.x + VR.w / 2, VR.y + VR.h / 2];
  };
  setZoom(0);
  const nodeAt = i => {
    const [x, y] = NODES[i], w = reduce ? 0 : Math.sin(drift * .9 + i * 1.7) * .8 * VS, h = reduce ? 0 : Math.cos(drift * .7 + i * 2.3) * .8 * VS;
    return [Math.round(VC[0] + x * VS + w), Math.round(VC[1] + y * VS + h)];
  };
  const vaultPoint = p => [Math.round(VC[0] + p[0] * VS), Math.round(VC[1] + p[1] * VS)];
  // a point in the scene, or ['v', dx, dy]: a point in the vault wherever the vault is right now
  const resolve = p => (p && p[0] === 'v') ? vaultPoint([p[1], p[2]]) : p;

  function drawVault(v, t) {
    const va = v.a; if (va <= 0) return;
    drift = t; setZoom(v.zoom);
    const { x, y, w, h } = VR;
    rect(x + 2, y, w - 4, h, 'K', va); rect(x, y + 2, w, h - 4, 'K', va); rect(x + 1, y + 1, w - 2, h - 2, 'K', va);
    rect(x + 2, y + 1, w - 4, h - 2, 'N', va); rect(x + 1, y + 2, w - 2, h - 4, 'N', va);
    STARS.forEach(([u, sv, ph]) => px(x + u * w, y + sv * h, 'T', va * (reduce ? .6 : .35 + .65 * Math.max(0, Math.sin(t * 1.6 + ph)))));
    const Pn = NODES.map((_, i) => nodeAt(i)), lim = (v.fill ?? 1) * NODES.length, vis = i => RANK[i] < lim;
    // a young vault shows its outline faintly; notes fill it in
    EDGES.forEach(([i, j]) => {
      if (vis(i) && vis(j)) trace(seg(Pn[i][0], Pn[i][1], Pn[j][0], Pn[j][1]), 'L', 1, va);
      else if (i < OUTLINE && j < OUTLINE) trace(seg(Pn[i][0], Pn[i][1], Pn[j][0], Pn[j][1]), 'L', 1, va * .4);
    });
    v.extra.forEach(e => {
      if (e.link <= 0) return;
      const [ex, ey] = vaultPoint(e.p);
      e.near = e.near || nearestNodes(e.p[0], e.p[1], 4);
      e.near.forEach(j => {
        trace(seg(ex, ey, Pn[j][0], Pn[j][1]), e.c, e.link, va);
        if (e.link >= 1 && e.pulse < 1) rect(Pn[j][0] - 1, Pn[j][1] - 1, 3, 3, e.c, va * (1 - e.pulse));
      });
    });
    Pn.forEach(([nx, ny], i) => {
      if (!vis(i)) return;
      const tw = reduce ? 1 : .55 + .45 * Math.sin(t * 2.1 + i * 1.3);
      if (DEG[i] >= 6 && VS > .75) rect(nx, ny, 2, 2, 'T', va); else px(nx, ny, 'T', va * (tw > .4 ? 1 : .55));
    });
    // marked notes: pointers the agent reads (gold), stale ones (coral)
    v.marks.forEach(mk => { if (mk.a > 0) mk.ids.forEach(i => { const [nx, ny] = Pn[i]; rect(nx - 1, ny - 1, 3, 3, mk.c, va * mk.a); px(nx - 2, ny, mk.c, va * mk.a); px(nx + 2, ny, mk.c, va * mk.a); }); });
    v.extra.forEach(e => {
      if (e.a <= 0) return;
      const [ex, ey] = vaultPoint(e.p), a = va * e.a, c = e.c;
      rect(ex - 1, ey - 1, 3, 3, c, a); px(ex - 2, ey, c, a); px(ex + 2, ey, c, a); px(ex, ey - 2, c, a); px(ex, ey + 2, c, a);
      if (e.pulse > 0 && e.pulse < 1) { const r = 3 + e.pulse * 8 * Math.max(.6, VS); for (let k = 0; k < 18; k++) { const a0 = k / 18 * Math.PI * 2; px(ex + Math.cos(a0) * r, ey + Math.sin(a0) * r, c, 1 - e.pulse); } }
    });
    // a dim veil over the window, used while the vault is only being read
    const dim = Math.max(v.dim, v.fdim || 0);
    if (dim > 0) for (let j = 1; j < h - 1; j++) for (let i = 1; i < w - 1; i++) px(x + i, y + j, 'N', dim * .6);
    label(v.name || 'VAULT', x + w / 2, FLOOR_Y + 4, va);
    if (v.ok > 0) spr(SPR.ok, x + w - 11, y + 2, v.ok);
    if (v.bad > 0) spr(SPR.bad, x + w - 11, y + 2, v.bad);
  }

  // ================= focus: where to look now =================
  // is the vault outside the zone in focus? then it darkens from inside instead of taking the veil
  const vaultAside = f => f.a > 0 && (VC[0] < f.x || VC[0] > f.x + f.w);
  function drawFocus(f, v) {
    if (f.a <= 0) return;
    const x = Math.round(f.x), y = Math.round(f.y), w = Math.round(f.w), h = Math.round(f.h), top = 15;
    o.save();
    if (v.a > 0) { o.beginPath(); o.rect(0, 0, W, H); o.rect(VR.x, VR.y, VR.w, VR.h); o.clip('evenodd'); }
    o.globalAlpha = f.a * .62; o.fillStyle = P.bg;
    o.fillRect(0, top, W, Math.max(0, y - top)); o.fillRect(0, y + h, W, H - y - h);
    o.fillRect(0, y, Math.max(0, x), h); o.fillRect(x + w, y, W - x - w, h);
    o.restore();
    const L = 5;
    [[x, y, 1, 1], [x + w - 1, y, -1, 1], [x, y + h - 1, 1, -1], [x + w - 1, y + h - 1, -1, -1]].forEach(([cx0, cy0, dx, dy]) => {
      for (let k = 0; k < L; k++) { px(cx0 + dx * k, cy0, 'B', f.a); px(cx0, cy0 + dy * k, 'B', f.a); }
    });
  }

  // ================= the agent robot =================
  function drawBot(m, t, casing, light) {
    if (m.a <= 0) return;
    const vx = m.x - m._x; m._x = m.x;
    m.speed = m.speed * .7 + Math.abs(vx) * .3;
    if (Math.abs(vx) > .05) m.dir = Math.sign(vx);
    const bob = reduce ? 0 : Math.round(Math.sin(t * 3.2 + (casing === 'S' ? 1.3 : 0)) * 1.2);
    const x = Math.round(m.x + Math.sin(m.shake * Math.PI * 7) * 2 * (1 - m.shake));
    const y = Math.round(m.y + bob - Math.sin(Math.PI * m.hop) * 8);
    const a = m.a;
    const hgt = FLOOR_Y - (m.y + 22), sw = Math.max(4, 12 - Math.round(hgt / 9) - Math.round(Math.sin(Math.PI * m.hop) * 3));
    rect(x + 10 - sw / 2, FLOOR_Y + 1, sw, 1, 'g', a);
    if (m.beam > 0) {
      if (m.beamDir === 'right') for (let i = 1; i < 16; i++) { const h = 1 + Math.floor(i / 2.2); for (let j = -h; j <= h; j++) px(x + 16 + i, y + 7 + j, 'B', m.beam * .38); }
      else if (m.beamDir === 'down') for (let i = 1; i < 14; i++) { const h = 1 + Math.floor(i / 2.2); for (let j = -h; j <= h; j++) px(x + 10 + j, y + 20 + i, 'B', m.beam * .38); }
      else for (let i = 1; i < 16; i++) { const h = 1 + Math.floor(i / 2.2); for (let j = -h; j <= h; j++) px(x + 3 - i, y + 7 + j, 'B', m.beam * .38); }
    }
    if (m.speed > .35 && !reduce) [4, 10, 15].forEach((ly, k) => rect(m.dir > 0 ? x - 4 - k : x + 21, y + ly, 3, 1, 'G', a * Math.min(1, m.speed)));
    spr(SPR.bot, x, y, a, { H: casing, L: light });
    const eyes = (m.eyes === 'open' && !reduce && (t % 3.3) < .13) ? 'blink' : m.eyes;
    EYES[eyes].forEach(([ex, ey]) => px(x + ex, y + ey, 'C', a));
    if (m.eyes === 'happy') { px(x + 4, y + 9, 'R', a); px(x + 15, y + 9, 'R', a); }
    if (m.eyes === 'think' && !reduce) [[21, 2], [23, -1], [26, -3]].forEach(([dx, dy], k) => { if ((t * 3 | 0) % 3 >= k) rect(x + dx, y + dy, k + 1, k + 1, 'G', a); });
    const arms = m.arms === 'wave' ? ((t * 6 | 0) % 2 ? 'up' : 'point') : m.arms;
    ARMS[arms].forEach(([ax, ay]) => px(x + ax, y + ay, 'K', a));
    (m.speed > .35 ? BOOST : FLAME[(t * 10 | 0) % 2]).forEach(([fx, fy, c]) => px(x + fx, y + fy, c, a));
    if (m.lens > 0) spr(SPR.lens, x - 7, y + 10, a * m.lens);
    if (m.carry && m.carryA > 0) spr(SPR[m.carry], x + 20, y + 10, a * m.carryA);
  }

  // ================= shared props =================
  // a code window; row 0..3 are code lines, `accent` marks the line that matters
  const CODE = [[4, 10], [6, 14], [8, 8], [10, 12]];
  function drawRepo(x, y, r, accent = 1, a0 = 1) {
    const a = r.a * a0 * (1 - (r.dim || 0) * .7);
    if (a <= 0) return;
    box(x, y, 18, 13, 'W', 'K', a);
    rect(x + 1, y + 2, 16, 1, 'K', a);
    [2, 4, 6].forEach(dx => px(x + dx, y + 1, 'K', a));
    CODE.forEach(([cy, end], k) => rect(x + 3 + (k % 2) * 2, y + cy, end - 2 - (k % 2) * 2, 1, k === accent ? 'B' : 'G', a));
    if (r.badge > 0) spr(SPR[r.badgeKind || 'changed'], x + 14, y - 3, r.badge * a0);
  }
  const codeRowAt = (x, y, row) => [x + CODE[row][1] + 2, y + CODE[row][0]];
  // a note: title bar and three claims, each marked with its evidence grade
  function drawNote(x, y, s, ends = [34, 24, 38], a0 = 1) {
    const a = s.a * a0; if (a <= 0) return;
    x += Math.round(Math.sin((s.shake || 0) * Math.PI * 8) * 2 * (1 - (s.shake || 0)));
    box(x, y, 44, 16, 'W', 'K', a);
    rect(x + 3, y + 2, 14, 2, s.title || 'K', a);
    rect(x + 1, y + 5, 42, 1, 'g', a);
    [7, 10, 13].forEach((cy, k) => {
      if (k === 0) rect(x + 3, y + cy - 1, 2, 2, 'K', a);
      else if (k === 1) { px(x + 3, y + cy - 1, 'K', a); px(x + 4, y + cy, 'K', a); }
      else px(x + 3, y + cy, 'K', a);
      const len = Math.round((ends[k] - 7) * (s['l' + k] ?? 1));
      rect(x + 7, y + cy, len, 1, (s.redLine === k && s.red > 0) ? 'R' : (s.blueLine === k ? 'B' : 'G'), a);
    });
    if (s.diff > 0) { spr(SPR.plus, x + 38, y + 6, s.diff * a0); spr(s.minus ? SPR.minus : SPR.plus, x + 38, y + 10, s.diff * a0); }
    if (s.bad > 0) spr(SPR.bad, x + 40, y - 3, s.bad * a0);
    if (s.ok > 0) spr(SPR.ok, x + 40, y - 3, s.ok * a0);
  }
  const claimAt = (x, y, k) => [x + 2, y + [7, 10, 13][k]];
  // a checklist card: an icon, a progress bar and a tick per row
  function drawChecklist(x, y, icons, ck) {
    if (ck.a <= 0) return;
    const w = 74, h = 10 + icons.length * 12;
    box(x, y, w, h, 'W', 'K', ck.a);
    icons.forEach((ic, i) => {
      const ry = y + 6 + i * 12;
      spr(SPR[ic], x + 5, ry, ck.a);
      rect(x + 16, ry + 2, 38, 3, 'g', ck.a);
      rect(x + 16, ry + 2, Math.round(38 * ck.bars[i].v), 3, ck.bars[i].bad ? 'R' : 'B', ck.a);
      spr(ck.ticks[i].a > 0 ? (ck.bars[i].bad ? SPR.bad : SPR.ok) : SPR.empty, x + 60, ry, ck.ticks[i].a > 0 ? ck.ticks[i].a * ck.a : ck.a);
    });
    if (ck.ok > 0) spr(SPR.okBig, x + w - 7, y - 5, ck.ok * ck.a);
  }
  const drawCloud = (c, x = c.x ?? 174, y = c.y ?? 17, name = c.name || 'TEAM') => {
    if (c.a <= 0) return;
    spr(SPR.cloud, x, y, c.a); label(name, x + 10, y + 11, c.a);
    if (c.ok > 0) spr(SPR.ok, x + 16, y + 3, c.ok);
  };
  const dashedBox = (x, y, w, h) => seg(x, y, x + w, y).concat(seg(x + w, y, x + w, y + h), seg(x + w, y + h, x, y + h), seg(x, y + h, x, y));

  // ================= common state and the frame =================
  const A = n => Array.from({ length: n }, () => ({ a: 0 }));
  const bot = (x, y) => ({ x, y, a: 0, eyes: 'open', arms: 'down', hop: 0, shake: 0, beam: 0, beamDir: 'left', lens: 0, carry: '', carryA: 0, _x: x, speed: 0, dir: 1 });
  function baseState(flow) {
    return {
      status: { text: '', n: 0, tone: 'K', a: 0 },
      task: { mode: 0, x: -48, y: 44, a: 0, done: 0, bad: 0 },
      cloud: { a: 0, ok: 0 },
      vault: { a: 0, ok: 0, bad: 0, dim: 0, fill: 1, zoom: 0, name: 'VAULT', marks: [], extra: [] },
      focus: { a: 0, x: 0, y: 15, w: W, h: FLOOR_Y - 15 },
      fly: [],
      m: bot(-24, 44), rv: bot(150, 16),
      poofs: [],
      conf: { t: 0, x: 188, y: 50 },
      word: flowWord(flow, flow.task),
    };
  }
  function render(t) {
    if (!cur) return;
    if (paletteDirty) { readPalette(); paletteDirty = false; }
    const st = cur.st;
    o.fillStyle = P.bg; o.fillRect(0, 0, W, H);
    for (let y = 20; y < FLOOR_Y; y += 8) for (let x = 4; x < W; x += 8) px(x, y, 'grid');
    rect(0, FLOOR_Y + 1, W, 1, 'g');
    drawCloud(st.cloud);
    setZoom(st.vault.zoom);
    st.vault.fdim = vaultAside(st.focus) ? st.focus.a : 0;
    drawVault(st.vault, t);
    cur.flow.draw(K, st, t);
    drawFocus(st.focus, st.vault);
    // flying packets: a dot, or a small page
    st.fly.forEach(b => {
      if (b.a <= 0) return;
      const to = resolve(b.to), fr = resolve(b.from);
      if (!to || !fr) return;
      const arc = Math.sin(Math.PI * b.t) * (b.arc ?? 18);
      const x = fr[0] + (to[0] - fr[0]) * b.t, y = fr[1] + (to[1] - fr[1]) * b.t - arc;
      const pos = q => [fr[0] + (to[0] - fr[0]) * q, fr[1] + (to[1] - fr[1]) * q - Math.sin(Math.PI * q) * (b.arc ?? 18)];
      if (b.t > 0 && b.t < 1 && !reduce) [1, 2, 3].forEach(k => { const [tx, ty] = pos(Math.max(0, b.t - k * .045)); px(tx, ty, b.c, b.a * (1 - k / 4)); });
      if (b.shape === 'card') {
        // the item itself travels, shrinking until it is a single node
        const k = Math.pow(b.t, .8), w = Math.max(3, Math.round(b.w0 + (3 - b.w0) * k)), h = Math.max(3, Math.round(b.h0 + (3 - b.h0) * k));
        if (w > 6) { box(x - w / 2, y - h / 2, w, h, 'W', 'K', b.a); if (w > 12) rect(x - w / 2 + 2, y - h / 2 + 2, Math.round(w / 3), 1, 'K', b.a); rect(x - w / 2 + 2, y + 1, Math.round(w / 2), 1, b.c, b.a); }
        else rect(x - 1, y - 1, 3, 3, b.c, b.a);
      }
      else if (b.shape === 'page') { box(x - 3, y - 4, 7, 8, 'W', 'K', b.a); rect(x - 1, y - 2, 3, 1, b.c, b.a); rect(x - 1, y, 3, 1, 'G', b.a); }
      else rect(x - 1, y - 1, 3, 3, b.c, b.a);
    });
    // status bar: the request pinned on the left, what is happening now on the right
    box(3, 3, W - 6, 11, 'W', 'g');
    const tk = st.task, word = st.word;
    if (tk.a > 0) {
      if (tk.mode === 1) {
        const w = textW(word, 2) + 8, x = Math.round(tk.x), y = Math.round(tk.y);
        rect(x, y, w, 16, 'B', tk.a); for (let i = 0; i < 4; i++) rect(x + 4, y + 16 + i, 4 - i, 1, 'B', tk.a);
        text(word, x + 4, y + 3, 'O', tk.a, 2);
      } else {
        const tw = textW(word) + 6 + (tk.done > 0 ? 7 : 0);
        rect(5, 5, tw, 7, 'B', tk.a);
        text(word, 8, 6, 'O', tk.a);
        if (tk.done > 0) [[0, 2], [1, 3], [2, 4], [3, 3], [4, 2], [5, 1]].forEach(([dx, dy]) => px(8 + textW(word) + 2 + dx, 6 + dy, 'O', tk.a * tk.done));
      }
    }
    const sb = st.status, sx = 5 + textW(word) + 12 + (tk.done > 0 ? 7 : 0);
    if (sb.a > 0 && sb.text) {
      const shown = sb.text.slice(0, Math.floor(sb.n));
      text(shown, sx, 6, sb.tone, sb.a);
      if (!reduce && sb.n < sb.text.length && (t * 8 | 0) % 2) rect(sx + textW(shown) + 2, 6, 2, 5, sb.tone, sb.a);
    }
    drawBot(st.m, t, 'W', 'B');
    drawBot(st.rv, t, 'S', 'R');
    if (cur.flow.drawTop) cur.flow.drawTop(K, st, t);
    st.poofs.forEach(p => {
      if (p.t >= 1) return;
      const r = 3 + p.t * 10;
      for (let k = 0; k < 8; k++) { const ang = k * Math.PI / 4; px(p.x + Math.cos(ang) * r, p.y + Math.sin(ang) * r, k % 2 ? 'Y' : 'B', 1 - p.t); px(p.x + Math.cos(ang) * (r - 2), p.y + Math.sin(ang) * (r - 2), 'Y', (1 - p.t) * .6); }
    });
    const cf = st.conf;
    if (cf.t > 0 && cf.t < 1) {
      const cols = ['B', 'R', 'Y', 'K', 'b', 'C'];
      for (let k = 0; k < 36; k++) {
        const ang = (k / 36) * Math.PI * 2 + k * .7, sp = 24 + (k * 37 % 30);
        rect(cf.x + Math.cos(ang) * sp * cf.t, cf.y + Math.sin(ang) * sp * cf.t * .8 + 30 * cf.t * cf.t, k % 3 ? 1 : 2, k % 4 ? 2 : 1, cols[k % 6], 1 - cf.t * cf.t);
      }
    }
    cx.imageSmoothingEnabled = false;
    cx.drawImage(off, 0, 0, cv.width, cv.height);
  }

  // ================= sound: small synthesized cues, heard only while the timeline plays =================
  // Nothing is loaded: every cue is a few oscillator or noise notes. Cues ride on the tweens they
  // belong to (onStart/onComplete), so seeking, scrubbing and switching language stay silent.
  const SFX = (() => {
    let ac = null, out = null, noiseBuf = null, on = true, joins = 0;
    const last = {};
    const ctx = () => {
      if (ac) return ac;
      const AC = window.AudioContext || window.webkitAudioContext;
      if (!AC) return null;
      ac = new AC();
      out = ac.createGain(); out.gain.value = .55;
      const soft = ac.createBiquadFilter(); soft.type = 'lowpass'; soft.frequency.value = 4800;
      const comp = ac.createDynamicsCompressor();
      out.connect(soft); soft.connect(comp); comp.connect(ac.destination);
      return ac;
    };
    const tone = (f, t0, d, { type = 'square', v = .1, f1, at = .005 } = {}) => {
      const osc = ac.createOscillator(), g = ac.createGain();
      osc.type = type; osc.frequency.setValueAtTime(f, t0);
      if (f1) osc.frequency.exponentialRampToValueAtTime(f1, t0 + d);
      g.gain.setValueAtTime(0, t0); g.gain.linearRampToValueAtTime(v, t0 + at); g.gain.exponentialRampToValueAtTime(.0001, t0 + d);
      osc.connect(g); g.connect(out); osc.start(t0); osc.stop(t0 + d + .02);
    };
    const noise = (t0, d, { v = .08, type = 'bandpass', f = 1200, f1, q = 1, at = .02 } = {}) => {
      if (!noiseBuf) {
        noiseBuf = ac.createBuffer(1, ac.sampleRate * 2, ac.sampleRate);
        const ch = noiseBuf.getChannelData(0); for (let i = 0; i < ch.length; i++) ch[i] = Math.random() * 2 - 1;
      }
      const src = ac.createBufferSource(), bf = ac.createBiquadFilter(), g = ac.createGain();
      src.buffer = noiseBuf; bf.type = type; bf.Q.value = q; bf.frequency.setValueAtTime(f, t0);
      if (f1) bf.frequency.exponentialRampToValueAtTime(f1, t0 + d);
      g.gain.setValueAtTime(0, t0); g.gain.linearRampToValueAtTime(v, t0 + Math.min(at, d / 2)); g.gain.exponentialRampToValueAtTime(.0001, t0 + d);
      src.connect(bf); bf.connect(g); g.connect(out); src.start(t0, Math.random()); src.stop(t0 + d + .02);
    };
    // a C major pentatonic, so every chime agrees with the others
    const PENTA = [523, 587, 659, 784, 880, 1047, 1175, 1319, 1568, 1760];
    const bell = (f, t0, d = .5, v = .08) => { tone(f, t0, d, { type: 'sine', v }); tone(f * 2, t0, d * .5, { type: 'sine', v: v * .25 }); };
    const CUES = {
      // a new guiding line in the status bar: one soft blip, not one per letter
      line: () => tone(740, ac.currentTime, .05, { type: 'triangle', v: .035 }),
      // a request arrives
      req: () => { const t = ac.currentTime; bell(1319, t, .45, .07); bell(1760, t + .11, .6, .07); },
      ok: () => { const t = ac.currentTime; tone(784, t, .09, { type: 'triangle', v: .09 }); tone(1047, t + .07, .2, { type: 'triangle', v: .09 }); },
      bad: () => { const t = ac.currentTime; tone(233, t, .11, { v: .05 }); tone(175, t + .12, .22, { v: .05 }); },
      hop: () => tone(330, ac.currentTime, .1, { v: .03, f1: 700 }),
      shake: () => { const t = ac.currentTime; tone(150, t, .07, { type: 'triangle', v: .07 }); tone(130, t + .09, .09, { type: 'triangle', v: .07 }); },
      whoosh: d => noise(ac.currentTime, Math.max(.2, d * .8), { v: .03, f: 380, f1: 1300, q: .7, at: d * .3 }),
      poof: () => noise(ac.currentTime, .28, { v: .09, type: 'lowpass', f: 1400, f1: 180, q: .5 }),
      page: d => noise(ac.currentTime, Math.max(.18, d * .6), { v: .045, type: 'highpass', f: 1800, f1: 5200, q: .6, at: .05 }),
      zip: d => tone(420, ac.currentTime, Math.max(.12, d * .7), { type: 'sine', v: .03, f1: 980, at: .03 }),
      land: () => tone(1175, ac.currentTime, .05, { type: 'triangle', v: .035 }),
      // knowledge joins the vault: each new node rings one step higher
      join: () => { const t = ac.currentTime, f = PENTA[3 + (joins++ % 7)]; bell(f, t, .8, .08); bell(f * 1.5, t + .06, .5, .03); noise(t, .12, { v: .02, type: 'highpass', f: 6000 }); },
      zoomIn: () => { const t = ac.currentTime; tone(131, t, .6, { type: 'sine', v: .07, f1: 262, at: .25 }); tone(262, t, .6, { type: 'triangle', v: .02, f1: 523, at: .25 }); },
      zoomOut: () => { const t = ac.currentTime; tone(262, t, .5, { type: 'sine', v: .05, f1: 131, at: .15 }); },
      beam: () => { const t = ac.currentTime; tone(600, t, .16, { type: 'triangle', v: .04, f1: 1250 }); tone(1250, t + .12, .25, { type: 'sine', v: .015 }); },
      scan: d => { const t = ac.currentTime; tone(880, t, d, { type: 'sine', v: .012, f1: 990, at: .2 }); },
      think: () => { const t = ac.currentTime; tone(440, t, .08, { type: 'triangle', v: .04 }); tone(392, t + .11, .12, { type: 'triangle', v: .04 }); },
      tick: () => tone(1568, ac.currentTime, .035, { v: .03 }),
      select: () => { const t = ac.currentTime; tone(1047, t, .05, { v: .04 }); tone(1568, t + .05, .12, { type: 'triangle', v: .06 }); },
      lock: () => { const t = ac.currentTime; noise(t, .03, { v: .12, f: 3200, q: 2, at: .002 }); tone(196, t + .05, .06, { v: .05 }); noise(t + .05, .04, { v: .08, f: 2200, q: 2, at: .002 }); },
      commit: () => tone(300, ac.currentTime, .07, { type: 'sine', v: .09, f1: 900 }),
      stamp: () => { const t = ac.currentTime; tone(110, t, .2, { type: 'sine', v: .22, f1: 55 }); noise(t, .09, { v: .08, type: 'lowpass', f: 700 }); },
      alert: () => { const t = ac.currentTime; tone(880, t, .08, { type: 'triangle', v: .05 }); tone(880, t + .14, .08, { type: 'triangle', v: .05 }); },
      // a pen writing lines
      write: d => { const t = ac.currentTime; for (let i = 0; i < Math.max(2, Math.round(d / .09)); i++) noise(t + i * .09, .06, { v: .03, f: 2600 + Math.random() * 1200, q: 4, at: .01 }); },
      grow: d => tone(262, ac.currentTime, Math.max(.2, d), { type: 'triangle', v: .035, f1: 523, at: .05 }),
      fanfare: () => {
        const t = ac.currentTime;
        [523, 659, 784, 1047, 1319].forEach((f, i) => tone(f, t + i * .07, .3, { type: 'triangle', v: .07 }));
        for (let i = 0; i < 6; i++) noise(t + .3 + i * .08, .06, { v: .025, type: 'highpass', f: 5000 + Math.random() * 3000 });
      },
    };
    // cues that say the same thing at the same moment sound once
    const GROUP = { ok: 'yes', select: 'yes', bad: 'no', alert: 'no', tick: 'tick', commit: 'tick', land: 'tick' };
    return {
      get on() { return on; },
      set on(v) { on = v; if (v) SFX.unlock(); },
      unlock: () => { if (!on) return; const a = ctx(); if (a && a.state === 'suspended') a.resume(); },
      reset: () => { joins = 0; },
      play: (name, arg) => {
        if (!on || !ac || ac.state !== 'running' || !tl || tl.paused() || document.hidden) return;
        const key = GROUP[name] || name, now = performance.now();
        if (now - (last[key] || 0) < 90) return;
        last[key] = now;
        CUES[name](arg);
      },
    };
  })();
  // a flow's own show()/to() may name a cue for a key; these are the shared ones
  const KEY_CUE = { ok: 'ok', fixed: 'ok', lock: 'lock', bad: 'bad', no: 'bad', unk: 'bad', warn: 'alert', yes: 'tick', tag: 'tick', badge: 'tick', kos: 'tick', seg: 'tick', diff: 'tick', pick: 'select', closed: 'stamp' };

  // ================= timeline helpers handed to each flow =================
  function helpers(tl, st, labels) {
    const cue = (name, arg) => name ? () => SFX.play(name, typeof arg === 'function' ? arg() : arg) : undefined;
    // sfx plays when the tween starts, sfxEnd when it ends
    const cued = v => {
      const { sfx, sfxEnd, ...rest } = v;
      if (sfx) rest.onStart = cue(sfx, v.duration);
      if (sfxEnd) rest.onComplete = cue(sfxEnd);
      return rest;
    };
    const S = (obj, v, p) => {
      const { sfx, ...rest } = v;
      const name = sfx || (v.beam === 1 || v.lens === 1 ? 'beam' : v.eyes === 'think' ? 'think' : null);
      return tl.set(obj, { ...rest, immediateRender: false, onComplete: cue(name) }, p);
    };
    const T = {
      tl, S, reduce,
      show: (obj, p, d = .3, key = 'a', sfx = KEY_CUE[key]) => tl.to(obj, { [key]: 1, duration: d, ease: 'none', onStart: cue(sfx) }, p),
      hide: (obj, p, d = .3, key = 'a') => tl.to(obj, { [key]: 0, duration: d, ease: 'none' }, p),
      to: (obj, v, p) => tl.to(obj, cued(v), p),
      glide: (m, x, y, p, d = .8) => tl.to(m, { x, y, duration: d, ease: 'power2.inOut', onStart: () => { if (Math.hypot(m.x - x, m.y - y) > 14) SFX.play('whoosh', d); } }, p),
      hop: (m, p) => { S(m, { hop: 0 }, p); tl.to(m, { hop: 1, duration: .38, ease: 'none', onStart: cue('hop') }, '<'); },
      shake: (obj, p) => { S(obj, { shake: 0 }, p); tl.to(obj, { shake: 1, duration: .45, ease: 'none', onStart: cue('shake') }, '<'); },
      face: (m, eyes, p, arms) => S(m, arms ? { eyes, arms } : { eyes }, p),
      say: (en, p, tone = 'K') => {
        const text = tr(en);
        S(st.status, { text, tone, n: 0, a: 1, sfx: { B: 'ok', R: 'bad' }[tone] || 'line' }, p);
        tl.to(st.status, { n: text.length, duration: text.length * .035, ease: 'none' }, '<');
      },
      poof: (x, y, p) => { const f = { a: 0, x, y, t: 1 }; st.poofs.push(f); S(f, { x, y, t: 0 }, p); tl.to(f, { t: 1, duration: .45, ease: 'power1.out', onStart: cue('poof') }, '<'); },
      fly: (from, to, c, p, d = .7, opts = {}) => {
        // each flight is its own object: only its progress and visibility are tweened
        const b = { a: 0, t: 0, from, to, c, shape: opts.shape || 'dot', arc: opts.arc ?? 18, w0: opts.w || 44, h0: opts.h || 16 };
        st.fly.push(b);
        S(b, { t: 0, a: 1 }, p);
        tl.to(b, { t: 1, duration: d, ease: 'power1.inOut', onStart: cue(b.shape === 'dot' ? 'zip' : 'page', d), onComplete: cue('land') }, '<');
        S(b, { a: 0 }, '>');
      },
      // new knowledge joins the vault: it appears, links to its neighbours and pulses
      node: (p, c = 'C') => { const e = { a: 0, link: 0, pulse: 0, p, c }; st.vault.extra.push(e); return e; },
      join: (e, p) => { S(e, { a: 1, pulse: 0 }, p); tl.to(e, { link: 1, duration: .5, ease: 'power1.out', onStart: cue('join') }, '<'); tl.to(e, { pulse: 1, duration: .7, ease: 'power1.out' }, '<'); },
      vaultAt: p => ['v', p[0], p[1]],
      // each beat says where to look: a zone of the stage (or none), and whether the vault takes the stage
      scene: (p, zone, big = 0) => {
        const Z = { L: [2, 62], C: [50, 148], R: [138, W - 2], RW: [124, W - 2], LC: [2, 148], CR: [50, W - 2] }[zone];
        if (Z) tl.to(st.focus, { a: 1, x: Z[0], y: 15, w: Z[1] - Z[0], h: FLOOR_Y - 14, duration: .5, ease: 'power2.inOut' }, p);
        else tl.to(st.focus, { a: 0, duration: .4 }, p);
        const zoomCue = () => { if (Math.abs(st.vault.zoom - big) > .5) SFX.play(big ? 'zoomIn' : 'zoomOut'); };
        tl.to(st.vault, { zoom: big ? 1 : 0, duration: .6, ease: 'power2.inOut', onStart: zoomCue }, '<');
      },
      nodeAt: i => nodeAt(i),
      mark: (ids, c) => { const mk = { ids, c, a: 0 }; st.vault.marks.push(mk); return mk; },
      nearest: (x, y, n) => nearestNodes(x - BR.x, y - BR.y, n),
      confetti: (x, y, p) => { S(st.conf, { t: 0, x, y }, p); tl.to(st.conf, { t: 1, duration: 1.4, ease: 'power2.out', onStart: cue('fanfare') }, '<'); },
      step: (i, build) => { labels[i] = tl.duration(); tl.addLabel('s' + i); build(labels[i]); tl.addLabel('e' + i); tl.to({}, { duration: 1.6 }); },
      // the request arrives as a message, the agent catches it and it stays pinned as the running task
      intro: (t, park = [66, 40]) => {
        S(st.task, { mode: 1, a: 1 }, t);
        tl.to(st.task, { x: 14, duration: .8, ease: 'back.out(1.5)', onStart: cue('req') }, t + .1);
        // the agent stops beside the message, never on top of it, then goes to its place in the flow
        const beside = 14 + textW(st.word, 2) + 8 + 6;
        S(st.m, { a: 1 }, t);
        T.glide(st.m, beside, 40, t + .2, 1);
        S(st.m, { eyes: 'left' }, t + 1.1);
        S(st.m, { eyes: 'happy', arms: 'up' }, t + 1.5); T.hop(st.m, '<');
        T.poof(30, 50, t + 2); S(st.task, { mode: 2, a: 0 }, '<'); T.show(st.task, '<', .25);
        S(st.m, { eyes: 'open', arms: 'down' }, '<');
        T.glide(st.m, park[0], park[1], '<', .6);
      },
    };
    return T;
  }

  // ================= flows and the player =================
  const FLOWS = [];
  let cur = null, tl = null, labels = [], current = -1, timer = null;
  const K = {
    W, H, FLOOR_Y, VP, BR, SPR, CODE, reduce,
    px, rect, box, spr, text, textW, label, trace, cubic, seg, link, at, disc,
    drawRepo, drawNote, drawChecklist, drawCloud, codeRowAt, claimAt, dashedBox,
    A, bot, flow: f => FLOWS.push(f),
    nodeAt: i => nodeAt(i), vaultPoint, vaultRect: () => VR, nearestNode: (dx, dy) => nearestNodes(dx, dy, 1)[0],
  };
  window.KF = K;

  const rail = q('#rail'), scrub = q('#scrub'), playBtn = q('#play'), icon = q('#playIcon');
  const UI = {
    en: { eyebrow: 'knowledge-os \u00b7 flows', flows: 'Flows', stage: 'Flow animation', scrub: 'Animation position', prev: 'Previous step', next: 'Next step', play: 'Play', pause: 'Pause', lang: 'Language', sound: 'Sound', next: 'Next', again: 'Start over' },
    es: { eyebrow: 'knowledge-os \u00b7 flujos', flows: 'Flujos', stage: 'Animación del flujo', scrub: 'Posición de la animación', prev: 'Paso anterior', next: 'Paso siguiente', play: 'Reproducir', pause: 'Pausar', lang: 'Idioma', sound: 'Sonido', next: 'Siguiente', again: 'Volver al inicio' },
  };
  let playing = false;
  function setPlaying(onp) {
    playing = onp;
    icon.setAttribute('d', onp ? 'M7 5h3.5v14H7zM13.5 5H17v14h-3.5z' : 'M8 5.5v13a1 1 0 0 0 1.5.9l10-6.5a1 1 0 0 0 0-1.7l-10-6.5A1 1 0 0 0 8 5.5z');
    playBtn.setAttribute('aria-label', UI[LANG][onp ? 'pause' : 'play']);
  }
  function chrome() {
    const u = UI[LANG];
    document.documentElement.lang = LANG;
    q('#eyebrow').textContent = u.eyebrow;
    q('#flows').setAttribute('aria-label', u.flows); q('.stage').setAttribute('aria-label', u.stage);
    scrub.setAttribute('aria-label', u.scrub); q('#prev').setAttribute('aria-label', u.prev); q('#next').setAttribute('aria-label', u.next);
    q('#lang').setAttribute('aria-label', u.lang);
    q('#sound').setAttribute('aria-label', u.sound); q('#sound').setAttribute('aria-pressed', String(SFX.on));
    qa('#lang button').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.lang === LANG)));
    qa('#flows button').forEach(b => { b.querySelector('.name').textContent = flowWord(FLOWS.find(f => f.id === b.dataset.id), FLOWS.find(f => f.id === b.dataset.id).name); });
    if (cur) {
      const i = FLOWS.indexOf(cur.flow);
      q('#eyebrow').textContent = `${u.eyebrow} \u00b7 ${i + 1}/${FLOWS.length}`;
    }
    setPlaying(playing);
  }
  function stepAt(t) { let s = 0; labels.forEach((l, i) => { if (t >= l - 1e-3) s = i; }); return s; }
  function syncUI() {
    if (!tl) return;
    scrub.value = Math.round(tl.progress() * 1000);
    scrub.style.setProperty('--p', `${tl.progress() * 100}%`);
    showNext(tl.progress() >= 1);
    const s = stepAt(tl.time());
    if (s === current) return;
    current = s;
    qa('#rail button').forEach((b, i) => {
      b.dataset.state = i < s ? 'done' : '';
      if (i === s) b.setAttribute('aria-current', 'step'); else b.removeAttribute('aria-current');
    });
    const stp = cur.flow.steps[s];
    q('#capTitle').textContent = tr(stp.label);
    q('#capCmds').innerHTML = stp.cmds.map(c => `<code>${c}</code>`).join('');
    if (!reduce) gsap.fromTo('.caption', { opacity: 0, y: 8 }, { opacity: 1, y: 0, duration: .4, ease: 'power2.out' });
  }
  function stopTimer() { clearInterval(timer); timer = null; }
  function goto(i) {
    const n = cur.flow.steps.length;
    i = Math.max(0, Math.min(n - 1, i));
    stopTimer();
    if (reduce) { tl.pause(); tl.seek('e' + i); setPlaying(false); syncUI(); return; }
    tl.seek('s' + i); tl.play(); setPlaying(true);
  }
  function toggle() {
    if (reduce) {
      if (timer) { stopTimer(); setPlaying(false); return; }
      setPlaying(true);
      timer = setInterval(() => { if (current >= cur.flow.steps.length - 1) { stopTimer(); setPlaying(false); } else { tl.seek('e' + (current + 1)); syncUI(); } }, 3200);
      return;
    }
    if (tl.progress() >= 1) { tl.restart(); setPlaying(true); return; }
    if (tl.paused()) { tl.play(); setPlaying(true); } else { tl.pause(); setPlaying(false); }
  }

  function load(id, autoplay = true) {
    const flow = FLOWS.find(f => f.id === id) || FLOWS[0];
    stopTimer();
    if (tl) tl.kill();
    const st = baseState(flow);
    Object.assign(st, flow.state(K, st));
    tl = gsap.timeline({ paused: true, onUpdate: syncUI, onComplete: () => setPlaying(false) });
    labels = []; current = -1; SFX.reset();
    cur = { flow, st };
    flow.build(K, st, helpers(tl, st, labels));
    // UI for this flow
    q('#flowName').textContent = flowWord(flow, flow.name);
    q('#flowLede').textContent = tr(flow.lede);
    document.title = `${flowWord(flow, flow.name)} \u00b7 knowledge-os flows`;
    chrome(); showNext(false);
    qa('#flows button').forEach(b => { if (b.dataset.id === flow.id) b.setAttribute('aria-current', 'page'); else b.removeAttribute('aria-current'); });
    rail.style.gridTemplateColumns = `repeat(${flow.steps.length}, minmax(0, 1fr))`;
    rail.innerHTML = '';
    flow.steps.forEach((s, i) => {
      const li = document.createElement('li');
      li.innerHTML = `<button type="button"><span class="dot">${i + 1}</span><span class="label">${tr(s.label)}</span></button>`;
      li.firstChild.addEventListener('click', () => goto(i));
      rail.appendChild(li);
    });
    cv.setAttribute('aria-label', tr(flow.alt));
    if (reduce) { tl.seek('e0'); setPlaying(false); syncUI(); }
    else { syncUI(); if (autoplay) { setPlaying(true); gsap.delayedCall(.3, () => tl.play()); } else setPlaying(false); }
  }

  function open(id) { if (location.hash !== '#' + id) history.replaceState(null, '', '#' + id); load(id); }
  // at the end of a flow, the way on to the next one
  function showNext(on) {
    const b = q('#nextFlow'), i = cur ? FLOWS.indexOf(cur.flow) : -1;
    if (!b || !cur) return;
    const last = i === FLOWS.length - 1, u = UI[LANG];
    b.querySelector('.lbl').textContent = last ? u.again : `${u.next}: ${flowWord(FLOWS[i + 1], FLOWS[i + 1].name)}`;
    b.hidden = !on;
  }
  function start() {
    // language: the viewer's last choice, else the browser's
    let saved = null;
    try { saved = localStorage.getItem('kos-flows-lang'); } catch (e) { /* storage may be unavailable */ }
    LANG = saved || ((navigator.language || 'en').toLowerCase().startsWith('es') ? 'es' : 'en');
    qa('#lang button').forEach(b => b.addEventListener('click', () => {
      if (b.dataset.lang === LANG) return;
      LANG = b.dataset.lang;
      try { localStorage.setItem('kos-flows-lang', LANG); } catch (e) { /* ignore */ }
      chrome();
      const at = tl ? tl.progress() : 0, was = tl && !tl.paused();
      load(cur.flow.id, false);
      tl.progress(at); syncUI(); current = -1; syncUI();
      if (was) { tl.play(); setPlaying(true); }
    }));
    // sound: on unless the viewer turned it off; browsers let it start at the first click or key
    try { SFX.on = localStorage.getItem('kos-flows-sound') !== 'off'; } catch (e) { /* storage may be unavailable */ }
    q('#sound').addEventListener('click', () => {
      SFX.on = !SFX.on;
      try { localStorage.setItem('kos-flows-sound', SFX.on ? 'on' : 'off'); } catch (e) { /* ignore */ }
      chrome();
    });
    ['pointerdown', 'keydown'].forEach(ev => document.addEventListener(ev, SFX.unlock, { capture: true }));
    chrome();
    const nav = q('#flows');
    FLOWS.forEach(f => {
      const li = document.createElement('li');
      li.innerHTML = `<button type="button" data-id="${f.id}"><span class="n">${FLOWS.indexOf(f) + 1}</span><span class="name">${f.name}</span></button>`;
      li.firstChild.addEventListener('click', () => open(f.id));
      nav.appendChild(li);
    });
    q('#nextFlow').addEventListener('click', () => {
      const i = FLOWS.indexOf(cur.flow);
      open(FLOWS[(i + 1) % FLOWS.length].id);
    });
    playBtn.addEventListener('click', toggle);
    q('#prev').addEventListener('click', () => goto(current - 1));
    q('#next').addEventListener('click', () => goto(current + 1));
    scrub.addEventListener('input', () => { stopTimer(); tl.pause(); setPlaying(false); tl.progress(scrub.value / 1000); syncUI(); });
    document.addEventListener('keydown', e => {
      if (e.target.closest('input')) return;
      if (e.key === 'ArrowRight') goto(current + 1);
      else if (e.key === 'ArrowLeft') goto(current - 1);
      else if (e.key === ' ' && !e.target.closest('button')) { e.preventDefault(); toggle(); }
    });
    addEventListener('hashchange', () => load(location.hash.slice(1)));
    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => { paletteDirty = true; });
    new MutationObserver(() => { paletteDirty = true; }).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    addEventListener('resize', fit);
    fit();
    gsap.ticker.add(time => render(time));
    load(location.hash.slice(1) || FLOWS[0].id);
  }
  K.start = start;
})();
