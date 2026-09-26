// SYNC (kos sync, the README's Publish step): changed code becomes reviewed notes in a draft; the vault takes them in one verified step.
KF.flow({
  id: 'sync',
  name: 'Sync',
  task: 'SYNC',
  lede: 'Knowledge reaches the vault only through a draft that passes the gates, a fresh review and a final check.',
  alt: 'The agent gets the team\'s latest vault, finds changed code, opens a draft, writes notes that cite the code, checks every citation, gets a fresh agent\'s review, runs the final checks, then the notes join the vault and the team gets them.',
  // Spanish: every English phrase shown on screen, mapped to its translation
  es: {
    "SYNC": "SINCRONIZAR",
    "Sync": "Sincronizar",
    "Knowledge reaches the vault only through a draft that passes the gates, a fresh review and a final check.": "El conocimiento llega al vault solo a través de un borrador que pasa los controles, una revisión nueva y una verificación final.",
    "The agent gets the team's latest vault, finds changed code, opens a draft, writes notes that cite the code, checks every citation, gets a fresh agent's review, runs the final checks, then the notes join the vault and the team gets them.": "El agente trae lo último del vault del equipo, busca el código cambiado, abre un borrador, escribe notas que citan el código, revisa cada cita, pasa la revisión de un agente nuevo, hace la verificación final, y las notas entran al vault y llegan al equipo.",
    "Pull": "Traer",
    "Select": "Seleccionar",
    "Draft": "Borrador",
    "Write": "Escribir",
    "Gate": "Control",
    "Review": "Revisión",
    "Verify": "Verificar",
    "Publish": "Publicar",
    "GET LATEST FROM TEAM": "TRAE LO ÚLTIMO DEL EQUIPO",
    "VAULT UP TO DATE": "VAULT AL DÍA",
    "FIND CHANGED CODE": "BUSCA CÓDIGO CAMBIADO",
    "3 REPOS TO SYNC": "3 REPOS POR SINCRONIZAR",
    "OPEN A DRAFT": "ABRE UN BORRADOR",
    "WRITE NOTES FROM CODE": "ESCRIBE NOTAS DESDE EL CÓDIGO",
    "NOTHING NEW: ACKNOWLEDGE": "NADA NUEVO: QUEDA CONSTANCIA",
    "CHECK EVERY CITATION": "REVISA CADA CITA",
    "STALE CITATION": "CITA VENCIDA",
    "FIX AND CHECK AGAIN": "CORRIGE Y REVISA DE NUEVO",
    "ALL CITATIONS PASS": "TODAS LAS CITAS PASAN",
    "FRESH AGENT REVIEWS": "UN AGENTE NUEVO REVISA",
    "FINAL CHECKS": "VERIFICACIÓN FINAL",
    "ALL CLEAR": "TODO EN ORDEN",
    "SAVE INTO THE VAULT": "GUARDA EN EL VAULT",
    "SHARE WITH TEAM": "COMPARTE CON EL EQUIPO",
    "DONE": "LISTO",
    "DRAFT": "BORRADOR",
  },
  steps: [
    { label: 'Pull', cmds: ['kos sync pull'] },
    { label: 'Select', cmds: ['kos inventory', 'kos discover run'] },
    { label: 'Draft', cmds: ['kos sync start --name payments'] },
    { label: 'Write', cmds: ['git commit', 'kos sync acknowledge'] },
    { label: 'Gate', cmds: ['kos discover check --note …'] },
    { label: 'Review', cmds: ['kos sync review --verdict accept'] },
    { label: 'Verify', cmds: ['kos sync verify'] },
    { label: 'Publish', cmds: ['kos sync finish', 'git push'] },
  ],

  state: K => ({
    repos: [0, 1, 2, 3].map(i => ({ a: 0, dim: 0, badge: 0, badgeKind: i === 2 ? 'fresh' : 'changed' })),
    draft: { p: 0, a: 1, label: 0 },
    beam: { x: 34, a: 0 },
    notes: [0, 1].map(n => ({ a: 0, l0: 0, l1: 0, l2: 0, ok: 0, bad: 0, red: 0, redLine: 1, shake: 0, diff: 0, minus: n === 1 })),
    anchors: [0, 1].map(() => [0, 1, 2].map(() => ({ p: 0, a: 1 }))),
    anchorBad: { a: 0 }, anchorFix: { p: 0, a: 1 },
    ack: { a: 0, link: 0 },
    work: { a: 1 },
    bracket: { p: 0 }, lock: { a: 0 }, seal: { a: 0, drop: 0 },
    check: { a: 0, bars: [0, 1, 2, 3].map(() => ({ v: 0 })), ticks: K.A(4), ok: 0 },
  }),

  geo: {
    REPO_X: 12, REPO_Y: [24, 40, 56, 72], ACCENT: [1, 2, 0, 3],
    DRAFT: { x: 56, y: 17, w: 86, h: 74 },
    NOTE_X: 66, NOTE_Y: [22, 56], ACK: { x: 66, y: 41 },
    ENDS: [[34, 24, 38], [36, 28, 20]], CITE: [[0, 1, 3], [0, 1, 2]],
  },

  draw(K, s, t) {
    const g = this.geo, wa = s.work.a;
    const anchor = (n, k, row) => K.link(K.codeRowAt(g.REPO_X, g.REPO_Y[n ? 2 : 0], row), K.claimAt(g.NOTE_X, g.NOTE_Y[n], k));
    s.repos.forEach((r, i) => K.drawRepo(g.REPO_X, g.REPO_Y[i], r, g.ACCENT[i], wa));
    const codeA = Math.max(...s.repos.map(r => r.a)) * wa;
    if (codeA > 0) K.label('CODE', g.REPO_X + 9, 89, codeA);
    if (s.draft.p > 0) {
      const d = g.DRAFT;
      K.trace(K.dashedBox(d.x, d.y, d.w, d.h), 'B', s.draft.p, s.draft.a, { dash: 2, shift: K.reduce ? 0 : Math.floor(t * 8) });
      if (s.draft.label > 0) K.label('DRAFT', d.x + d.w / 2, d.y + d.h + 3, s.draft.label * s.draft.a);
    }
    if (s.ack.link > 0) K.trace(K.seg(31, g.REPO_Y[1] + 6, g.ACK.x - 1, g.ACK.y + 5), 'G', s.ack.link, wa, { dash: 1 });
    if (s.ack.a > 0) K.spr(K.SPR.ack, g.ACK.x, g.ACK.y, s.ack.a * wa);
    s.notes.forEach((n, i) => K.drawNote(g.NOTE_X, g.NOTE_Y[i], n, g.ENDS[i], wa));
    s.anchors.forEach((row, n) => row.forEach((an, k) => an.p > 0 && K.trace(anchor(n, k, g.CITE[n][k]), 'B', an.p, an.a * wa)));
    if (s.anchorBad.a > 0) K.trace(anchor(1, 1, 1), 'R', 1, s.anchorBad.a * wa, { dash: 2 });
    if (s.anchorFix.p > 0) K.trace(anchor(1, 1, 3), 'B', s.anchorFix.p, s.anchorFix.a * wa);
    if (s.beam.a > 0) for (let y = g.DRAFT.y + 1; y < g.DRAFT.y + g.DRAFT.h; y++) { K.px(s.beam.x, y, 'B', s.beam.a); for (let d = 1; d <= 3; d++) { K.px(s.beam.x - d, y, 'b', s.beam.a * (.6 - d * .15)); K.px(s.beam.x + d, y, 'b', s.beam.a * (.6 - d * .15)); } }
    if (s.bracket.p > 0) K.trace(K.seg(111, 22, 113, 22).concat(K.seg(113, 22, 113, 72), K.seg(113, 72, 111, 72), K.seg(114, 47, 117, 47)), 'B', s.bracket.p, wa);
    if (s.seal.a > 0) K.spr(K.SPR.seal, 118, 40 + Math.round(s.seal.drop), s.seal.a * wa);
    if (s.lock.a > 0) K.spr(K.SPR.lock, 110, 43, s.lock.a * wa);
    K.drawChecklist(62, 24, ['iNote', 'iTree', 'iClean', 'iSeal'], s.check);
  },

  build(K, s, T) {
    const g = this.geo, { m, rv } = s;
    const news = [T.node([-19, -7]), T.node([5, -12]), T.node([-5, 5], 'G'), T.node([15, 1])];

    // 1 PULL: the vault gets the team's latest knowledge first
    T.step(0, t => {
      T.intro(t);
      T.scene('>', 'R', 1);
      T.show(s.vault, '>+.1', .4);
      T.show(s.cloud, '<.2', .4);
      T.glide(m, 122, 66, '<', .9);
      T.say('GET LATEST FROM TEAM', '>');
      T.face(m, 'up', '<');
      T.fly([184, 26], T.vaultAt(news[0].p), 'C', '>+.1', .8);
      T.join(news[0], '>');
      T.show(s.vault, '<', .2, 'ok');
      T.say('VAULT UP TO DATE', '<', 'B');
      T.face(m, 'happy', '<'); T.hop(m, '<');
      T.hide(s.vault, '+=.8', .2, 'ok');
      T.face(m, 'open', '<');
    });

    // 2 SELECT: changed and new repositories are picked, unchanged ones dim
    T.step(1, t => {
      T.scene(t, 'L', 0);
      s.repos.forEach((r, i) => T.show(r, t + i * .1, .3));
      T.say('FIND CHANGED CODE', t);
      T.glide(m, 38, 16, t + .1, .9);
      T.S(m, { eyes: 'left', beam: 1, beamDir: 'left', arms: 'pointL' }, '>');
      const s0 = T.tl.duration();
      T.to(m, { y: 68, duration: 2.6, ease: 'sine.inOut', sfx: 'scan' }, s0);
      T.show(s.repos[0], s0 + .35, .2, 'badge');
      T.show(s.repos[1], s0 + 1.0, .2, 'badge');
      T.show(s.repos[2], s0 + 1.6, .2, 'badge');
      T.to(s.repos[3], { dim: 1, duration: .3 }, s0 + 2.3);
      T.S(m, { beam: 0, eyes: 'happy', arms: 'down' }, s0 + 2.7); T.hop(m, '<');
      T.say('3 REPOS TO SYNC', '<', 'B');
    });

    // 3 DRAFT: an isolated workspace (the sync branch); nothing reaches the vault until verified
    T.step(2, t => {
      T.scene(t, 'C', 0);
      T.hide(s.repos[3], t, .3);
      T.say('OPEN A DRAFT', t);
      T.face(m, 'open', t);
      T.glide(m, 58, 20, t + .1, .6);
      const d0 = T.tl.duration();
      T.to(m, { keyframes: [{ x: 120, y: 20, duration: .5 }, { x: 120, y: 66, duration: .45 }, { x: 58, y: 66, duration: .5 }, { x: 58, y: 20, duration: .45 }], ease: 'none' }, d0);
      T.to(s.draft, { p: 1, duration: 1.9, ease: 'none', sfx: 'grow' }, d0);
      T.show(s.draft, '>', .3, 'label');
      T.glide(m, 114, 40, '>', .6);
      T.face(m, 'happy', '>'); T.hop(m, '<');
    });

    // 4 WRITE: every claim cites its code; a repo with nothing new is acknowledged
    T.step(3, t => {
      T.scene(t, 'LC', 0);
      T.face(m, 'left', t, 'down');
      const write = (n, y) => {
        T.say('WRITE NOTES FROM CODE', '>');
        T.glide(m, 114, y, '<', .6);
        T.show(s.notes[n], '<.3', .3);
        [0, 1, 2].forEach(k => {
          T.S(m, { arms: 'pointL' }, '>');
          T.to(s.notes[n], { ['l' + k]: 1, duration: .35, ease: 'none', sfx: 'write' }, '<');
          T.to(s.anchors[n][k], { p: 1, duration: .45, ease: 'power1.inOut' }, '<.1');
          T.S(m, { arms: 'down' }, '>');
        });
      };
      write(0, 18);
      T.say('NOTHING NEW: ACKNOWLEDGE', '>+.2');
      T.glide(m, 114, 36, '<', .5);
      T.to(s.ack, { link: 1, duration: .45, ease: 'none' }, '<.3');
      T.show(s.ack, '>', .3, 'a', 'tick');
      T.S(m, { arms: 'pointL' }, '<'); T.S(m, { arms: 'down' }, '>+.4');
      write(1, 52);
      T.face(m, 'happy', '>'); T.hop(m, '<');
    });

    // 5 GATE: every citation is checked; a stale one fails, gets fixed and passes
    T.step(4, t => {
      T.scene(t, 'LC', 0);
      T.say('CHECK EVERY CITATION', t);
      T.face(m, 'left', t);
      T.show(s.beam, t, .2);
      T.to(s.beam, { x: 111, duration: 1.4, ease: 'sine.inOut', sfx: 'scan' }, '>');
      T.hide(s.beam, '>-.1', .2);
      T.show(s.notes[0], '<-.3', .25, 'ok');
      T.to(s.anchors[1][1], { a: 0, duration: .15 }, '<.2');
      T.show(s.anchorBad, '<', .2);
      T.show(s.notes[1], '<', .2, 'red');
      T.show(s.notes[1], '<', .2, 'bad');
      T.shake(s.notes[1], '<');
      T.say('STALE CITATION', '<', 'R');
      T.face(m, 'worried', '<'); T.shake(m, '<');
      T.glide(m, 40, 74, '+=.6', .8);
      T.say('FIX AND CHECK AGAIN', '>');
      T.face(m, 'up', '<', 'up');
      T.hide(s.anchorBad, '>+.1', .2);
      T.hide(s.notes[1], '<', .2, 'red');
      T.S(s.notes[1], { l1: 0 }, '<');
      T.to(s.notes[1], { l1: 1, duration: .35, ease: 'none', sfx: 'write' }, '>');
      T.to(s.anchorFix, { p: 1, duration: .45 }, '<.1');
      T.S(m, { arms: 'down' }, '>');
      T.hide(s.notes[1], '<', .15, 'bad');
      T.show(s.notes[1], '>', .2, 'ok');
      T.say('ALL CITATIONS PASS', '<', 'B');
      T.face(m, 'happy', '<'); T.hop(m, '<');
    });

    // 6 REVIEW: a second, fresh agent reads the changes and approves exactly that content
    T.step(5, t => {
      T.scene(t, 'C', 0);
      T.face(m, 'open', t);
      T.glide(m, 126, 84, t, .9);
      T.say('FRESH AGENT REVIEWS', t + .3);
      T.S(rv, { x: 118, y: 16 }, t);
      T.poof(126, 22, t + .6);
      T.show(rv, '<', .25);
      T.S(rv, { beam: 1, beamDir: 'left', lens: 1, arms: 'hold', eyes: 'left' }, '>+.1');
      T.to(rv, { keyframes: [{ y: 16, duration: .3 }, { y: 36, duration: .7 }, { y: 50, duration: .7 }], ease: 'sine.inOut' }, '>');
      T.show(s.notes[0], '<.5', .2, 'diff');
      T.show(s.notes[1], '<1.2', .2, 'diff');
      T.S(rv, { beam: 0, lens: 0, arms: 'down', eyes: 'open' }, '>+.4');
      T.glide(rv, 140, 32, '>', .6);
      T.S(rv, { arms: 'pointL', eyes: 'happy' }, '>');
      T.S(s.seal, { a: 1, drop: -20 }, '<');
      T.to(s.seal, { drop: 0, duration: .22, ease: 'power2.in', sfxEnd: 'stamp' }, '<');
      T.shake(s.notes[0], '>'); T.shake(s.notes[1], '<');
      T.say('APPROVED', '<', 'B');
      T.to(s.bracket, { p: 1, duration: .45, ease: 'none' }, '>');
      T.show(s.lock, '>', .25, 'a', 'lock');
      T.face(m, 'happy', '<');
      T.S(rv, { arms: 'down' }, '>+.3');
    });

    // 7 VERIFY: note gates, no new structure issues, clean tree, review covers the final content
    T.step(6, t => {
      T.scene(t, 'C', 0);
      T.poof(150, 42, t); T.hide(rv, t, .25);
      T.hide(s.work, t, .35);
      T.say('FINAL CHECKS', t);
      T.face(m, 'open', t);
      T.glide(m, 140, 44, t + .2, .8);
      T.show(s.check, t + .4, .35);
      T.face(m, 'left', '>');
      s.check.bars.forEach((b, i) => {
        T.to(b, { v: 1, duration: .45, ease: 'power1.inOut' }, '>+.05');
        T.show(s.check.ticks[i], '>', .15, 'a', 'tick');
      });
      T.show(s.check, '>+.1', .2, 'ok');
      T.say('ALL CLEAR', '<', 'B');
      T.face(m, 'happy', '<'); T.hop(m, '<');
    });

    // 8 PUBLISH: the notes join the vault, the draft closes, the team gets it
    T.step(7, t => {
      T.scene(t, null, 1);
      T.hide(s.check, t, .3);
      T.to(s.anchors.flat().concat(s.anchorFix), { a: 0, duration: .01 }, t);
      T.hide(s.bracket, t, .01, 'p'); T.hide(s.lock, t, .01); T.hide(s.seal, t, .01);
      [0, 1].forEach(n => { T.hide(s.notes[n], t, .01, 'ok'); T.hide(s.notes[n], t, .01, 'diff'); });
      T.show(s.work, t + .1, .3);
      T.say('SAVE INTO THE VAULT', t + .2);
      T.face(m, 'right', t + .2);
      T.glide(m, 124, 88, t + .2, .7);
      const items = [[g.NOTE_X + 20, g.NOTE_Y[0] + 6, s.notes[0]], [g.ACK.x + 4, g.ACK.y + 3, s.ack], [g.NOTE_X + 20, g.NOTE_Y[1] + 6, s.notes[1]]];
      items.forEach(([x, y, src], k) => {
        const card = k === 1 ? { w: 12, h: 10 } : { w: 44, h: 16 };
        T.S(src, { a: 0 }, k ? '>-.2' : '>+.1');
        T.fly([x + 2, y + 2], T.vaultAt(news[k + 1].p), k === 1 ? 'G' : 'C', '<', 1.1, { shape: 'card', arc: 16, ...card });
        T.join(news[k + 1], '>');
      });
      T.hide(s.draft, '>', .4); // finish fast-forwards main and deletes the sync branch
      T.hide(s.ack, '<', .4, 'link');
      [0, 1, 2].forEach(i => T.hide(s.repos[i], '<', .4));
      T.face(m, 'happy', '<'); T.hop(m, '<');
      T.say('SHARE WITH TEAM', '>+.3');
      T.face(m, 'up', '<');
      T.fly(T.vaultAt(news[1].p), [183, 23], 'C', '>', .7);
      T.fly(T.vaultAt(news[3].p), [186, 23], 'C', '<.15', .7);
      T.show(s.cloud, '>', .2, 'ok');
      T.show(s.task, '<', .2, 'done');
      T.show(s.vault, '<', .2, 'ok');
      T.say('DONE', '<', 'B');
      T.face(m, 'happy', '<', 'up'); T.hop(m, '<');
      T.confetti(184, 50, '<');
      T.hop(m, '>-.9');
      T.S(m, { arms: 'wave' }, '>');
    });
  },
});
