// ASK (evidence contract): the vault is the map, not the boundary; every claim is sourced, graded and checked.
KF.flow({
  id: 'ask',
  name: 'Ask',
  task: 'ASK',
  lede: 'The agent answers from sources it inspected, grades every claim and checks the names it cites before replying.',
  alt: 'The agent binds the vault, searches it for pointers, finds a stale note, follows the trail to the code, the cloud snapshot, the database and the tracker, grades each claim, catches a name no note, fact or snapshot knows, gets an independent review and delivers a sourced, graded answer.',
  // Spanish: every English phrase shown on screen, mapped to its translation
  es: {
    "ASK": "PREGUNTAR",
    "Ask": "Preguntar",
    "The agent answers from sources it inspected, grades every claim and checks the names it cites before replying.": "El agente responde desde fuentes que inspeccionó, da un grado a cada afirmación y revisa los nombres que cita antes de responder.",
    "The agent binds the vault, searches it for pointers, finds a stale note, follows the trail to the code, the cloud snapshot, the database and the tracker, grades each claim, catches a name no note, fact or snapshot knows, gets an independent review and delivers a sourced, graded answer.": "El agente fija el vault, busca punteros en él, encuentra una nota vencida, sigue el rastro al código, al snapshot de la nube, a la base de datos y al tracker, da un grado a cada afirmación, detecta un nombre que ninguna nota, hecho ni snapshot conoce, pasa una revisión independiente y entrega una respuesta con fuentes y grados.",
    "Bind": "Fijar",
    "Search": "Buscar",
    "Check": "Revisar",
    "Trail": "Rastro",
    "Grade": "Calificar",
    "Claims": "Afirmaciones",
    "Review": "Revisión",
    "Answer": "Responder",
    "BIND THE VAULT": "FIJA EL VAULT",
    "SCOPE LOCKED": "ALCANCE FIJADO",
    "SEARCH THE MAP": "BUSCA EN EL MAPA",
    "3 POINTERS: NOT ANSWERS": "3 PISTAS, NO RESPUESTAS",
    "IS THE NOTE FRESH?": "¿LA NOTA ESTÁ AL DÍA?",
    "STALE: FOLLOW THE TRAIL": "VENCIDA: SIGUE EL RASTRO",
    "THE VAULT IS THE MAP": "EL VAULT ES EL MAPA",
    "FOLLOW THE TRAIL": "SIGUE EL RASTRO",
    "READ AT THE REF BRANCH": "LEÍDO EN LA RAMA DE REFERENCIA",
    "DEMONSTRATED": "DEMOSTRADO",
    "OBSERVED WITHIN LIMITS": "OBSERVADO CON LÍMITES",
    "INFERRED": "INFERIDO",
    "UNRESOLVED": "SIN RESOLVER",
    "CHECK EVERY NAME": "REVISA CADA NOMBRE",
    "UNKNOWN NAME!": "¡NOMBRE DESCONOCIDO!",
    "CLAIMS OK": "AFIRMACIONES OK",
    "INDEPENDENT REVIEW": "REVISIÓN INDEPENDIENTE",
    "SOURCED AND GRADED": "CON FUENTES Y GRADOS",
    "ANSWER DELIVERED": "RESPUESTA ENTREGADA",
    "DB": "BD",
  },
  steps: [
    { label: 'Bind', cmds: ['kos config resolve'] },
    { label: 'Search', cmds: ['rg -n "orders" 20-Repos 25-Topics 30-Flujos'] },
    { label: 'Check', cmds: ['kos discover check --note …'] },
    { label: 'Trail', cmds: ['git -C <repo> show main:<path>'] },
    { label: 'Grade', cmds: [] },
    { label: 'Claims', cmds: ['rg -n "<name>" 25-Topics .agents/state/discovery 90-Meta/discovery'] },
    { label: 'Review', cmds: [] },
    { label: 'Answer', cmds: [] },
  ],

  geo: {
    // sources the trail can reach: sprite, x, y, label, colour of its evidence
    SRC: [['repo', 12, 19, 'CODE', 'B'], ['cloud', 8, 44, 'CLOUD', 'b'], ['db', 14, 64, 'DB', 'G'], ['ticket', 14, 85, 'TICKETS', 'Y']],
    CARD: { x: 62, y: 34, w: 62, h: 38 },
    ROWS: [40, 30, 36, 22],
    POINTERS: [[-18, -6], [2, -10], [14, 4]],
  },

  state: K => ({
    bind: { a: 0 },
    src: [0, 1, 2, 3].map(() => ({ a: 0, flash: 0 })),
    card: { a: 0, ok: 0, unk: 0, fixed: 0, rows: [0, 1, 2, 3].map(() => ({ p: 0, g: 0, s: 0 })) },
    scan: { x: 0, a: 0 },
  }),

  draw(K, s, t) {
    const g = this.geo, c = g.CARD, cd = s.card;
    // the trail beyond the vault
    g.SRC.forEach(([kind, x, y, name], i) => {
      const a = s.src[i].a; if (a <= 0) return;
      if (kind === 'repo') K.drawRepo(x, y, { a }, 1); else K.spr(K.SPR[kind], x, y, a);
      K.label(name, kind === 'cloud' ? x + 10 : x + 7, y + (kind === 'repo' ? 15 : kind === 'cloud' ? 11 : 12), a);
      if (s.src[i].flash > 0) K.rect(x - 1, y - 1, kind === 'cloud' ? 22 : 13, 12, 'Y', s.src[i].flash * .7);
    });
    const vr = K.vaultRect();
    if (s.bind.a > 0) K.spr(K.SPR.lock, vr.x + vr.w - 10, vr.y + vr.h - 11, s.bind.a);
    // the draft answer: four claims, each with its evidence grade
    if (cd.a > 0) {
      const a = cd.a;
      K.box(c.x, c.y, c.w, c.h, 'W', 'K', a);
      K.rect(c.x + 4, c.y + 3, 22, 2, 'K', a);
      K.rect(c.x + 1, c.y + 7, c.w - 2, 1, 'g', a);
      cd.rows.forEach((r, k) => {
        const y = c.y + 12 + k * 6, x = c.x + 4;
        if (r.g > 0) {
          const ga = a * r.g;
          if (k === 0) K.rect(x, y - 1, 3, 3, 'K', ga);                                      // demonstrated
          else if (k === 1) { K.rect(x, y - 1, 3, 3, 'K', ga); K.rect(x + 1, y - 1, 2, 2, 'W', ga); K.px(x + 2, y + 1, 'K', ga); } // observed within limits
          else if (k === 2) { K.px(x, y - 1, 'K', ga); K.px(x + 2, y - 1, 'K', ga); K.px(x, y + 1, 'K', ga); K.px(x + 2, y + 1, 'K', ga); } // inferred
          else { K.rect(x, y - 1, 3, 3, 'K', ga); K.px(x + 1, y, 'W', ga); }                // unresolved
        }
        K.rect(x + 6, y, Math.round(g.ROWS[k] * r.p), 1, 'G', a);
        if (r.s > 0) K.rect(x + 8 + g.ROWS[k], y - 1, 3, 3, g.SRC[k][4], a * r.s);
      });
      // a name no repository, snapshot or note knows
      if (cd.unk > 0) { const y = c.y + 24; K.rect(c.x + 10 + 26, y, 12, 1, 'R', cd.unk * (1 - cd.fixed)); K.text('?', c.x + 55, y - 3, 'R', cd.unk * (1 - cd.fixed)); }
      if (cd.ok > 0) K.spr(K.SPR.ok, c.x + c.w - 4, c.y - 3, cd.ok);
      if (s.scan.a > 0) for (let y = c.y + 1; y < c.y + c.h - 1; y++) { K.px(s.scan.x, y, 'B', s.scan.a); K.px(s.scan.x - 1, y, 'b', s.scan.a * .5); K.px(s.scan.x + 1, y, 'b', s.scan.a * .5); }
    }
  },

  build(K, s, T) {
    const g = this.geo, { m, rv } = s, c = g.CARD;
    const ids = g.POINTERS.map(([x, y]) => K.nearestNode(x, y));
    const pointers = T.mark(ids, 'Y'), stale = T.mark([ids[1]], 'R');

    // 1 BIND: the vault is resolved and the question's scope fixed before any lookup
    T.step(0, t => {
      T.intro(t, [60, 40]);
      T.scene('>', 'R', 1);
      T.show(s.vault, '>', .4);
      T.say('BIND THE VAULT', '<');
      T.glide(m, 120, 60, '<', .8);
      T.face(m, 'right', '>');
      T.show(s.bind, '>', .3, 'a', 'lock');
      T.say('SCOPE LOCKED', '<', 'B');
      T.face(m, 'happy', '<'); T.hop(m, '<');
    });

    // 2 SEARCH: the map returns pointers to open and verify, not answers
    T.step(1, t => {
      T.scene(t, 'R', 1);
      T.say('SEARCH THE MAP', t);
      T.face(m, 'right', t);
      T.S(m, { beam: 1, beamDir: 'right', arms: 'point' }, t + .2);
      T.show(pointers, t + .8, .3, 'a', 'select');
      T.say('3 POINTERS: NOT ANSWERS', '>', 'B');
      T.S(m, { beam: 0, arms: 'down' }, '>+.4');
    });

    // 3 CHECK: the pointed note is gated; its citation is stale
    T.step(2, t => {
      T.scene(t, 'R', 1);
      T.say('IS THE NOTE FRESH?', t);
      T.face(m, 'think', t);
      T.show(stale, t + .9, .25);
      T.show(s.vault, '<', .2, 'bad');
      T.face(m, 'worried', '<'); T.shake(m, '<');
      T.say('STALE: FOLLOW THE TRAIL', '<', 'R');
    });

    // 4 TRAIL: the vault is the map, not the boundary; the agent reads the sources themselves
    T.step(3, t => {
      T.scene(t, 'LC', 0);
      T.hide(s.vault, t, .2, 'bad');
      T.say('THE VAULT IS THE MAP', t);
      s.src.forEach((sr, i) => T.show(sr, t + .2 + i * .12, .3));
      T.show(s.card, t + .4, .3);
      T.face(m, 'open', t);
      g.SRC.forEach(([, , y, name], i) => {
        T.glide(m, 36, y - 4, i ? '>+.1' : '>', .6);
        T.S(m, { beam: 1, beamDir: 'left', eyes: 'left' }, '>');
        if (i === 0) T.say('FOLLOW THE TRAIL', '<');
        T.to(s.src[i], { flash: 1, duration: .1, yoyo: true, repeat: 1, sfx: 'tick' }, '<');
        T.fly([28, y + 4], [c.x + 12 + g.ROWS[i], c.y + 12 + i * 6], g.SRC[i][4], '>', .6, { arc: 10 });
        T.to(s.card.rows[i], { p: 1, duration: .3 }, '>');
        T.show(s.card.rows[i], '<', .2, 's');
        T.S(m, { beam: 0 }, '<');
      });
      T.say('READ AT THE REF BRANCH', '>', 'B');
    });

    // 5 GRADE: every claim carries its grade
    T.step(4, t => {
      T.scene(t, 'C', 0);
      T.face(m, 'right', t);
      T.glide(m, 126, 76, t, .7);
      ['DEMONSTRATED', 'OBSERVED WITHIN LIMITS', 'INFERRED', 'UNRESOLVED'].forEach((grade, k) => {
        T.say(grade, k ? '>+.35' : '>', k === 3 ? 'R' : 'B');
        T.show(s.card.rows[k], '<', .25, 'g', 'tick');
        T.S(m, { arms: 'pointL' }, '<'); T.S(m, { arms: 'down' }, '>+.2');
      });
      T.face(m, 'happy', '>'); T.hop(m, '<');
    });

    // 6 CLAIMS: every resource the answer names is checked; an invented one is caught and removed
    T.step(5, t => {
      T.scene(t, 'C', 0);
      T.say('CHECK EVERY NAME', t);
      T.face(m, 'left', t);
      T.S(s.scan, { x: c.x + 2, a: 1 }, t + .2);
      T.to(s.scan, { x: c.x + c.w - 3, duration: 1.2, ease: 'sine.inOut', sfx: 'scan' }, '>');
      T.show(s.card, '<.7', .2, 'unk');
      T.hide(s.scan, '>', .2);
      T.say('UNKNOWN NAME!', '<', 'R');
      T.face(m, 'worried', '<'); T.shake(m, '<');
      T.show(s.card, '>+.6', .3, 'fixed');
      T.face(m, 'open', '<', 'pointL');
      T.say('CLAIMS OK', '>', 'B');
      T.face(m, 'happy', '<', 'down');
    });

    // 7 REVIEW: a new conclusion gets an independent review before it is delivered
    T.step(6, t => {
      T.scene(t, 'C', 0);
      T.say('INDEPENDENT REVIEW', t);
      T.glide(m, 100, 86, t, .7);
      T.face(m, 'open', t);
      T.S(rv, { x: 128, y: 30 }, t);
      T.poof(138, 38, t + .5); T.show(rv, '<', .25);
      T.S(rv, { beam: 1, beamDir: 'left', lens: 1, arms: 'hold', eyes: 'left' }, '>+.1');
      T.to(rv, { keyframes: [{ y: 26, duration: .5 }, { y: 44, duration: .8 }], ease: 'sine.inOut' }, '>');
      T.S(rv, { beam: 0, lens: 0, arms: 'pointL', eyes: 'happy' }, '>');
      T.show(s.card, '<', .2, 'ok');
      T.say('APPROVED', '<', 'B');
      T.S(rv, { arms: 'down' }, '>+.4');
    });

    // 8 ANSWER: the reply goes back with its sources and grades
    T.step(7, t => {
      T.scene(t, null, 0);
      T.poof(138, 52, t); T.hide(rv, t, .25);
      T.say('SOURCED AND GRADED', t);
      T.face(m, 'up', t);
      T.hide(s.card, t + .5, .3);
      T.fly([c.x + 30, c.y + 18], [14, 8], 'B', '<', .9, { shape: 'page', arc: 20 });
      T.show(s.task, '>', .2, 'done');
      T.say('ANSWER DELIVERED', '<', 'B');
      T.face(m, 'happy', '<', 'up'); T.hop(m, '<');
      T.confetti(40, 30, '<');
      T.S(m, { arms: 'wave' }, '>+.3');
    });
  },
});
