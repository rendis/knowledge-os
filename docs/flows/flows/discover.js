// DISCOVER (kos discover): connections are extracted from code and read-only cloud snapshots, compared with the vault and gated.
KF.flow({
  id: 'discover',
  name: 'Discover',
  task: 'DISCOVER',
  lede: 'Connections come out of the code and read-only cloud snapshots, each with its evidence; the vault\'s notes are checked against them.',
  alt: 'The agent scans the repositories into facts, answers the judgments the scanner cannot make, snapshots the cloud read-only, records a service no provider reads, compares the facts with the vault, blocks a note with a stale citation and lists the corrections.',
  // Spanish: every English phrase shown on screen, mapped to its translation
  es: {
    "DISCOVER": "DESCUBRIR",
    "Discover": "Descubrir",
    "Connections come out of the code and read-only cloud snapshots, each with its evidence; the vault's notes are checked against them.": "Las conexiones salen del código y de snapshots de solo lectura de la nube, cada una con su evidencia; las notas del vault se contrastan con ellas.",
    "The agent scans the repositories into facts, answers the judgments the scanner cannot make, snapshots the cloud read-only, records a service no provider reads, compares the facts with the vault, blocks a note with a stale citation and lists the corrections.": "El agente escanea los repositorios y obtiene hechos, responde los juicios que el escáner no puede decidir, toma un snapshot de solo lectura de la nube, registra un servicio que ningún proveedor lee, compara los hechos con el vault, bloquea una nota con una cita vencida y lista las correcciones.",
    "Scan": "Escanear",
    "Classify": "Clasificar",
    "Snapshot": "Snapshot",
    "Record": "Registrar",
    "Compare": "Comparar",
    "Gate": "Control",
    "Correct": "Corregir",
    "SCAN THE CODE": "ESCANEA EL CÓDIGO",
    "IMPORTS CONFIG IAC": "IMPORTS CONFIG IAC",
    "8 FACTS FOUND": "8 HECHOS ENCONTRADOS",
    "WHAT DOES IT TALK TO?": "¿CON QUÉ HABLA?",
    "IT IS A QUEUE": "ES UNA COLA",
    "DB": "BD",
    "QUEUE": "COLA",
    "JUDGMENT STORED": "JUICIO GUARDADO",
    "READ THE CLOUD": "LEE LA NUBE",
    "READ ONLY: YOUR LOGIN": "SOLO LECTURA: TU LOGIN",
    "SNAPSHOT SAVED": "SNAPSHOT GUARDADO",
    "1 SCOPE NEEDS ACCESS": "1 ÁMBITO SIN ACCESO",
    "NO PROVIDER READS IT": "NINGÚN PROVEEDOR LO LEE",
    "RECORDED AS EVIDENCE": "REGISTRADO COMO EVIDENCIA",
    "COMPARE WITH THE VAULT": "COMPARA CON EL VAULT",
    "1 MISMATCH": "1 DISCREPANCIA",
    "1 NOT IN THE VAULT": "1 FALTA EN EL VAULT",
    "CHECK THE NOTES": "REVISA LAS NOTAS",
    "STALE CITATION: BLOCKED": "CITA VENCIDA: BLOQUEADA",
    "LIST CORRECTIONS": "LISTA LAS CORRECCIONES",
    "FIXED AT THE SOURCE": "CORREGIDO EN LA FUENTE",
    "NEXT: PUBLISH WITH SYNC": "SIGUE: PUBLICAR CON SYNC",
    "FACTS": "HECHOS",
  },
  steps: [
    { label: 'Scan', cmds: ['kos discover run'] },
    { label: 'Classify', cmds: ['kos discover questions', 'kos discover answer'] },
    { label: 'Snapshot', cmds: ['kos discover platform --referenced'] },
    { label: 'Record', cmds: ['kos discover platform --record'] },
    { label: 'Compare', cmds: ['kos discover report'] },
    { label: 'Gate', cmds: ['kos discover check --note …'] },
    { label: 'Correct', cmds: ['kos discover corrections'] },
  ],

  // facts: [x, y, colour]; repo facts are ink, cloud facts light blue, the recorded service gold
  geo: {
    REPO_Y: [20, 36, 52],
    FACTS: [[70, 30, 'K'], [88, 25, 'K'], [106, 32, 'K'], [78, 46, 'K'], [98, 48, 'K'], [116, 44, 'K'], [86, 62, 'K'], [108, 64, 'K'],
      [74, 80, 'b'], [96, 82, 'b'], [118, 80, 'Y']],
    LINKS: [[0, 1], [1, 2], [0, 3], [3, 4], [4, 5], [2, 5], [3, 6], [6, 7], [4, 7], [6, 8], [7, 9], [5, 10]],
    // fact -> vault point it matches: [fact, dx, dy, kind]
    MATCH: [[1, -6, -12, 'B'], [4, -14, 2, 'B'], [5, 10, -4, 'B'], [2, 4, -14, 'R']],
    MISSING: 7, ASK: 6,
  },

  icons: {
    DB: ['.KKKKK.', 'K.....K', '.KKKKK.', 'K.....K', '.KKKKK.', '.......'],
    API: ['.K...K.', 'K.....K', 'K..B..K', 'K.....K', '.K...K.', '.......'],
    QUEUE: ['KK.KK.B', 'KK.KK.B', '.......', 'KKKKKKB', '.......', '.......'],
  },

  state: K => ({
    repos: [0, 1, 2].map(() => ({ a: 0 })),
    platform: { a: 0, flash: 0, warn: 0 },
    sftp: { a: 0 },
    facts: { a: 1, frame: 0, n: [...Array(11)].map(() => ({ a: 0 })), l: [...Array(12)].map(() => ({ p: 0 })) },
    q: { a: 0, hover: -1, pick: 0 }, qnode: { a: 0, done: 0, link: 0 },
    match: [0, 1, 2, 3].map(() => ({ p: 0 })),
    missing: { a: 0 },
    note: { a: 0, l0: 1, l1: 1, l2: 1, ok: 0, bad: 0, red: 0, redLine: 2, shake: 0 },
    stale: { p: 0, fix: 0 },
    fixes: { a: 0, bars: [0, 1].map(() => ({ v: 0 })), ticks: K.A(2), ok: 0 },
  }),

  draw(K, s, t) {
    const g = this.geo, fa = s.facts.a, Q_ICON = this.icons;
    s.repos.forEach((r, i) => K.drawRepo(12, g.REPO_Y[i], r, i));
    if (s.repos[0].a > 0) K.label('CODE', 21, 68, s.repos[0].a);
    if (s.platform.a > 0) {
      K.spr(K.SPR.cloud, 8, 88, s.platform.a); K.rect(13, 92, 9, 1, 'G', s.platform.a); K.rect(13, 94, 9, 1, 'G', s.platform.a); K.px(20, 92, 'B', s.platform.a); K.label('CLOUD', 18, 99, s.platform.a);
      if (s.platform.flash > 0) K.rect(8, 88, 20, 9, 'Y', s.platform.flash * .8);
      if (s.platform.warn > 0) K.spr(K.SPR.warn, 24, 84, s.platform.warn);
    }
    if (s.sftp.a > 0) { K.spr(K.SPR.srv, 41, 90, s.sftp.a); K.label('SFTP', 45, 99, s.sftp.a); }
    // facts: extracted connections, kept apart from the vault until they are published
    if (s.facts.frame > 0) {
      K.trace(K.dashedBox(60, 18, 66, 72), 'g', s.facts.frame, fa, { dash: 1 });
      K.label('FACTS', 93, 93, s.facts.frame * fa * (1 - s.q.a));
    }
    g.LINKS.forEach(([i, j], k) => { const l = s.facts.l[k]; if (l.p > 0) K.trace(K.seg(g.FACTS[i][0], g.FACTS[i][1], g.FACTS[j][0], g.FACTS[j][1]), 'G', l.p, fa); });
    // comparison with the vault: matches in blue, a mismatch in coral
    g.MATCH.forEach(([f, dx, dy, c], k) => {
      const mt = s.match[k]; if (mt.p <= 0) return;
      const to = K.nodeAt(K.nearestNode(dx, dy));
      K.trace(K.link([g.FACTS[f][0], g.FACTS[f][1]], to, 18), c, mt.p, fa, c === 'R' ? { dash: 2 } : {});
    });
    g.FACTS.forEach(([x, y, c], i) => {
      const n = s.facts.n[i]; if (n.a <= 0) return;
      K.rect(x - 1, y - 1, 3, 3, c, n.a * fa);
      if (i === g.MISSING && s.missing.a > 0) {
        const pulse = K.reduce ? 1 : .5 + .5 * Math.sin(t * 6);
        K.rect(x - 2, y - 2, 5, 5, 'C', s.missing.a * fa * pulse); K.px(x, y - 4, 'C', s.missing.a * fa); K.px(x, y + 4, 'C', s.missing.a * fa);
      }
    });
    // a judgment the scanner cannot make: what does this dependency talk to?
    const [qx, qy] = g.FACTS[g.ASK];
    if (s.qnode.a > 0) {
      const a = s.qnode.a * fa;
      if (s.qnode.done > 0) { K.spr(Q_ICON.QUEUE, qx - 3, qy - 3, a); }
      else {
        const pulse = K.reduce ? 1 : .55 + .45 * Math.sin(t * 6);
        K.rect(qx - 3, qy - 3, 7, 7, 'R', a * pulse); K.rect(qx - 1, qy - 1, 3, 3, 'W', a);
        K.text('?', qx - 1, qy - 11, 'R', a);
      }
    }
    if (s.q.a > 0) {
      const a = s.q.a, cx0 = 59, cy0 = 74;
      K.trace(K.seg(qx, qy + 5, qx, cy0 - 1), 'R', 1, a * (1 - s.q.pick), { dash: 1 });
      K.box(cx0, cy0, 75, 22, 'W', 'K', a);
      ['DB', 'API', 'QUEUE'].forEach((opt, k) => {
        const x = cx0 + 3 + k * 24, sel = s.q.pick > 0 && k === 2, hov = s.q.hover === k;
        K.box(x, cy0 + 3, 21, 16, sel ? 'B' : 'W', sel || hov ? 'B' : 'g', a);
        K.spr(Q_ICON[opt], x + 7, cy0 + 4, a, sel ? { K: 'O', B: 'O' } : null);
        K.label(opt, x + 10.5, cy0 + 12, a, sel ? 'O' : 'G');
      });
    }
    // a vault note whose citation no longer matches the code
    K.drawNote(76, 40, s.note, [34, 26, 38]);
    if (s.stale.p > 0) K.trace(K.link(K.codeRowAt(12, 36, 2), K.claimAt(76, 40, 2)), s.stale.fix > 0 ? 'B' : 'R', s.stale.p, 1, s.stale.fix > 0 ? {} : { dash: 2 });
    K.drawChecklist(64, 66, ['iLink', 'iNote'], s.fixes);
  },

  build(K, s, T) {
    const g = this.geo, { m } = s, F = s.facts;
    // 1 SCAN: imports, manifests, configuration and IaC become facts, each with its evidence
    T.step(0, t => {
      T.intro(t, [40, 40]);
      T.scene('>', 'LC', 0);
      T.show(s.vault, '>', .4);
      s.repos.forEach((r, i) => T.show(r, `<${i ? .1 : 0}`, .3));
      T.say('SCAN THE CODE', '<');
      T.glide(m, 36, 12, '>', .6);
      T.S(m, { eyes: 'left', beam: 1, beamDir: 'left', arms: 'pointL' }, '>');
      const s0 = T.tl.duration();
      T.to(m, { y: 48, duration: 2.2, ease: 'sine.inOut', sfx: 'scan' }, s0);
      T.to(F, { frame: 1, duration: .8, ease: 'none' }, s0);
      [0, 1, 2, 3, 4, 5, 6, 7].forEach((i, k) => {
        const [x, y] = g.FACTS[i];
        T.fly([30, g.REPO_Y[k % 3] + 6], [x, y], 'K', s0 + .2 + k * .22, .5, { arc: 6 });
        T.show(F.n[i], '>', .1);
      });
      [0, 1, 2, 3, 4, 5, 6, 7, 8].forEach((k, j) => { if (g.LINKS[k][1] < 8) T.to(F.l[k], { p: 1, duration: .3 }, s0 + 1.2 + j * .12); });
      T.say('IMPORTS CONFIG IAC', s0 + .6);
      T.S(m, { beam: 0, eyes: 'happy', arms: 'down' }, s0 + 2.3); T.hop(m, '<');
      T.say('8 FACTS FOUND', '<', 'B');
    });

    // 2 CLASSIFY: what the scanner cannot decide becomes a question with fixed options; the answer is stored
    T.step(1, t => {
      T.scene(t, 'C', 0);
      // one connection the scanner cannot type on its own
      T.show(s.qnode, t + .1, .3, 'a', 'alert');
      T.say('WHAT DOES IT TALK TO?', t);
      T.glide(m, 127, 50, t, .7);
      T.show(s.q, '>', .3);
      T.face(m, 'think', '<', 'pointL');
      // the agent weighs the fixed options and picks one
      [0, 1, 2].forEach(k => T.S(s.q, { hover: k, sfx: 'tick' }, '>+.45'));
      T.show(s.q, '>+.35', .15, 'pick');
      T.S(s.qnode, { done: 1 }, '<');
      T.face(m, 'happy', '<');
      T.say('IT IS A QUEUE', '<', 'B');
      // the judgment is stored in the vault, versioned
      T.hide(s.q, '>+.7', .25);
      T.fly([94, 84], T.vaultAt([0, -2]), 'G', '<', .8, { shape: 'page', arc: 14 });
      T.say('JUDGMENT STORED', '<', 'B');
      T.face(m, 'open', '>', 'down');
    });

    // 3 SNAPSHOT: the cloud is listed read-only with the developer's own login
    T.step(2, t => {
      T.scene(t, 'LC', 0);
      T.show(s.platform, t, .3);
      T.say('READ THE CLOUD', t);
      T.glide(m, 34, 74, t, .9);
      T.S(m, { beam: 1, beamDir: 'left', eyes: 'left', carry: 'iKey', carryA: 1 }, '>');
      T.say('READ ONLY: YOUR LOGIN', '<');
      T.to(s.platform, { flash: 1, duration: .1, yoyo: true, repeat: 1 }, '>+.4');
      [8, 9].forEach((i, k) => { T.fly([18, 90], [g.FACTS[i][0], g.FACTS[i][1]], 'b', `>${k ? '-.3' : ''}`, .6); T.show(F.n[i], '>', .1); });
      T.to(F.l[9], { p: 1, duration: .3 }, '>'); T.to(F.l[10], { p: 1, duration: .3 }, '<');
      T.say('SNAPSHOT SAVED', '<', 'B');
      T.show(s.platform, '>+.4', .2, 'warn');
      T.face(m, 'worried', '<');
      T.say('1 SCOPE NEEDS ACCESS', '<', 'R');
      T.S(m, { beam: 0, carryA: 0 }, '>+.6');
    });

    // 4 RECORD: a service no provider reads is inspected and recorded as evidence
    T.step(3, t => {
      T.scene(t, 'LC', 0);
      T.show(s.sftp, t, .3);
      T.say('NO PROVIDER READS IT', t);
      T.face(m, 'open', t);
      T.glide(m, 37, 64, t + .1, .6);
      T.S(m, { beam: 1, beamDir: 'down', eyes: 'up' }, '>');
      T.fly([45, 92], [g.FACTS[10][0], g.FACTS[10][1]], 'Y', '>+.5', .7);
      T.show(F.n[10], '>', .1);
      T.to(F.l[11], { p: 1, duration: .3 }, '>');
      T.S(m, { beam: 0, eyes: 'happy' }, '<');
      T.say('RECORDED AS EVIDENCE', '<', 'B');
    });

    // 5 COMPARE: facts against the vault's notes: matches, a mismatch, something undocumented
    T.step(4, t => {
      T.scene(t, 'CR', 1);
      T.hide(s.platform, t, .3, 'warn');
      T.say('COMPARE WITH THE VAULT', t);
      T.face(m, 'right', t);
      T.glide(m, 124, 84, t, .8);
      s.match.forEach((mt, k) => T.to(mt, { p: 1, duration: .5, ease: 'power1.inOut' }, `>${k ? '-.25' : ''}`));
      T.show(s.vault, '>', .2, 'bad');
      T.say('1 MISMATCH', '<', 'R');
      T.show(s.missing, '>+.4', .3, 'a', 'alert');
      T.say('1 NOT IN THE VAULT', '<', 'R');
      T.face(m, 'worried', '<');
    });

    // 6 GATE: a note whose citation no longer matches the code is blocked
    T.step(5, t => {
      T.scene(t, 'LC', 0);
      T.hide(s.vault, t, .2, 'bad');
      T.hide(F, t, .4); T.hide(s.missing, t, .3);
      s.match.forEach(mt => T.hide(mt, t, .2, 'p'));
      T.hide(s.sftp, t, .3);
      T.say('CHECK THE NOTES', t);
      T.face(m, 'left', t);
      T.glide(m, 124, 36, t, .7);
      T.show(s.note, t + .4, .3);
      T.to(s.stale, { p: 1, duration: .5 }, '>');
      T.show(s.note, '<', .2, 'red');
      T.show(s.note, '>', .2, 'bad');
      T.shake(s.note, '<');
      T.face(m, 'worried', '<'); T.shake(m, '<');
      T.say('STALE CITATION: BLOCKED', '<', 'R');
    });

    // 7 CORRECT: every unsupported relation becomes a correction, fixed at the source, published with sync
    T.step(6, t => {
      T.scene(t, 'LC', 0);
      T.say('LIST CORRECTIONS', t);
      T.face(m, 'open', t);
      T.show(s.fixes, t + .2, .3);
      T.face(m, 'left', '>', 'pointL');
      T.to(s.fixes.bars[0], { v: 1, duration: .5 }, '>');
      T.show(s.fixes.ticks[0], '>', .15, 'a', 'tick');
      T.S(s.stale, { fix: 1 }, '<');
      T.hide(s.note, '<', .2, 'red'); T.hide(s.note, '<', .2, 'bad'); T.show(s.note, '>', .2, 'ok');
      T.say('FIXED AT THE SOURCE', '<', 'B');
      T.to(s.fixes.bars[1], { v: 1, duration: .5 }, '>+.2');
      T.show(s.fixes.ticks[1], '>', .15, 'a', 'tick');
      T.show(s.fixes, '>+.1', .2, 'ok');
      T.say('NEXT: PUBLISH WITH SYNC', '<', 'B');
      T.show(s.task, '<', .2, 'done');
      T.face(m, 'happy', '<', 'up'); T.hop(m, '<');
      T.S(m, { arms: 'wave' }, '>+.3');
    });
  },
});
