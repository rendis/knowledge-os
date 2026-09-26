// HANDOFF (kos handoff): an atomic task package goes to a repository worktree for any harness and comes back as evidence.
KF.flow({
  id: 'handoff',
  name: 'Handoff',
  task: 'HANDOFF',
  lede: 'A case hands atomic tasks to a repository worktree any agent can work in; the commits come back to the case as evidence.',
  alt: 'The agent writes a task package in the case, previews the handoff, creates a worktree on a new issue branch cut from main with the package kept local, another agent builds and commits there, the progress is read, and reconcile brings the commits and deltas back into the case as evidence.',
  // Spanish: every English phrase shown on screen, mapped to its translation
  es: {
    "HANDOFF": "DELEGAR",
    "Handoff": "Delegar",
    "A case hands atomic tasks to a repository worktree any agent can work in; the commits come back to the case as evidence.": "Un caso entrega tareas atómicas a un worktree del repositorio donde puede trabajar cualquier agente; los commits vuelven al caso como evidencia.",
    "The agent writes a task package in the case, previews the handoff, creates a worktree on a new issue branch cut from main with the package kept local, another agent builds and commits there, the progress is read, and reconcile brings the commits and deltas back into the case as evidence.": "El agente escribe un paquete de tareas en el caso, presenta el repositorio y previsualiza el handoff, crea un worktree en una rama issue nueva cortada desde main con el paquete guardado localmente, otro agente construye y hace commits ahí, se lee el avance, y reconcile trae los commits y los deltas de vuelta al caso como evidencia.",
    "Package": "Paquete",
    "Preview": "Vista previa",
    "Start": "Iniciar",
    "Build": "Construir",
    "Status": "Estado",
    "Reconcile": "Reconciliar",
    "WRITE THE PACKAGE": "ESCRIBE EL PAQUETE",
    "ATOMIC TASKS: CHECKED": "TAREAS ATÓMICAS: REVISADAS",
    "THE REPOSITORY": "EL REPOSITORIO",
    "PREVIEW THE HANDOFF": "PREVISUALIZA EL HANDOFF",
    "NOTHING APPLIED YET": "AÚN NO SE APLICA NADA",
    "BRANCH FROM MAIN": "RAMA DESDE MAIN",
    "WORKTREE CREATED": "WORKTREE CREADO",
    ".HANDOFF STAYS LOCAL": ".HANDOFF QUEDA LOCAL",
    "KOS NEVER COMMITS": "KOS NUNCA HACE COMMIT",
    "ANY AGENT CAN BUILD": "CUALQUIER AGENTE CONSTRUYE",
    "COMMITS: HANDOFF DH-001": "COMMITS: HANDOFF DH-001",
    "CHECK PROGRESS": "REVISA EL AVANCE",
    "RECONCILE INTO THE CASE": "RECONCILIA EN EL CASO",
    "READ THE COMMITS": "LEE LOS COMMITS",
    "COMMITS: DEMONSTRATED": "COMMITS: DEMOSTRADO",
    "READ THE DELTAS": "LEE LOS DELTAS",
    "FINDINGS INTO THE CASE": "HALLAZGOS AL CASO",
    "MARK ADVANCED": "MARCA AVANZADA",
    "RUN AGAIN: NOTHING NEW": "OTRA VEZ: NADA NUEVO",
    "CASE STAYS OPEN": "EL CASO SIGUE ABIERTO",
    "CASE": "CASO",
    "REPO": "REPO",
    "WORKTREE": "WORKTREE",
  },
  steps: [
    { label: 'Package', cmds: ['kos investigation add --kind handoff --package handoffs/DH-001.md'] },
    { label: 'Preview', cmds: ['kos handoff start --package …'] },
    { label: 'Start', cmds: ['kos handoff start --package … --apply'] },
    { label: 'Build', cmds: [] },
    { label: 'Status', cmds: ['kos handoff status'] },
    { label: 'Reconcile', cmds: ['kos handoff reconcile', 'kos handoff reconcile --apply'] },
  ],

  geo: {
    CASE: { x: 8, y: 26, w: 50, h: 40 },
    WT: { x: 132, y: 20, w: 86, h: 70 },
    MAIN_Y: 98, BR_Y: 82, TIP: 124, MAIN: [84, 104, 124], BC: [160, 180, 200],
  },

  state: K => ({
    kase: { a: 0, pkg: 0, ok: 0, recs: K.A(2) },
    repo: { a: 0, len: 0 },
    wt: { a: 0, ghost: 0 },
    branch: { len: 0, ghost: 0, tag: 0 },
    bc: K.A(3).map(c => Object.assign(c, { hl: 0 })),
    dir: { a: 0, lock: 0 }, agents: { a: 0, seg: 0 }, deltas: { a: 0, n: 0, hl: 0 },
    dash: { a: 0, bars: [0, 1, 2].map(() => ({ v: 0 })), ticks: K.A(3), ok: 0 },
    mark: { a: 0, x: 0 },
  }),

  draw(K, s, t) {
    const g = this.geo, C = g.CASE, Wt = g.WT;
    // the case, in the vault: the package and what comes back
    if (s.kase.a > 0) {
      const a = s.kase.a;
      K.rect(C.x, C.y - 4, 16, 4, 'Y', a); K.box(C.x, C.y, C.w, C.h, 'W', 'K', a); K.rect(C.x, C.y - 4, 16, 1, 'K', a); K.rect(C.x, C.y - 4, 1, 4, 'K', a); K.rect(C.x + 15, C.y - 4, 1, 4, 'K', a);
      K.label('CASE', C.x + C.w / 2, C.y + C.h + 4, a);
      if (s.kase.pkg > 0) { K.spr(K.SPR.box, C.x + 4, C.y + 5, s.kase.pkg); K.text('DH-001', C.x + 20, C.y + 7, 'K', s.kase.pkg); }
      if (s.kase.ok > 0) K.spr(K.SPR.ok, C.x + C.w - 5, C.y - 3, s.kase.ok);
      s.kase.recs.forEach((r, i) => { if (r.a > 0) { K.spr(K.SPR.file, C.x + 5, C.y + 17 + i * 11, r.a); K.text(i ? 'E-003' : 'E-002', C.x + 15, C.y + 19 + i * 11, 'B', r.a); } });
    }
    // the repository: main and, in its worktree, the issue branch
    if (s.repo.a > 0) {
      const a = s.repo.a;
      K.rect(74, g.MAIN_Y, Math.round((g.TIP - 74) * s.repo.len), 2, 'K', a);
      g.MAIN.forEach(x => { if (74 + (g.TIP - 74) * s.repo.len >= x) K.spr(['.KKK.', 'KWWWK', 'KWWWK', 'KWWWK', '.KKK.'], x - 2, g.MAIN_Y - 1, a); });
      const tw = K.textW('MAIN') + 6; K.rect(g.TIP - tw / 2, g.MAIN_Y + 5, tw, 9, 'K', a); K.text('MAIN', g.TIP - tw / 2 + 3, g.MAIN_Y + 7, 'W', a);
      K.label('REPO', 145, K.FLOOR_Y + 4, a);
    }
    const bp = K.cubic([g.TIP, g.MAIN_Y], [g.TIP + 8, g.MAIN_Y], [g.TIP + 4, g.BR_Y], [g.TIP + 12, g.BR_Y]).concat(K.seg(g.TIP + 13, g.BR_Y, 214, g.BR_Y));
    if (s.branch.ghost > 0) K.trace(bp, 'B', 1, s.branch.ghost * .5, { dash: 2 });
    K.trace(bp, 'B', s.branch.len, 1);
    K.trace(bp.map(([x, y]) => [x, y + 1]), 'B', s.branch.len, 1);
    s.bc.forEach((c, i) => {
      if (c.a <= 0) return;
      if (c.hl > 0) { K.rect(g.BC[i] - 4, g.BR_Y - 3, 9, 9, 'Y', c.hl); }
      K.spr(['.BBB.', 'BWWWB', 'BWWWB', 'BWWWB', '.BBB.'], g.BC[i] - 2, g.BR_Y - 1, c.a);
    });
    if (s.branch.tag > 0) { const lb = 'ISSUE/PAY-1', tw = K.textW(lb) + 6; K.rect(176 - tw / 2, g.BR_Y - 11, tw, 9, 'S', s.branch.tag); K.text(lb, 176 - tw / 2 + 3, g.BR_Y - 9, 'B', s.branch.tag); }
    if (s.mark.a > 0) { const mx = g.BC[0] + (g.BC[2] - g.BC[0]) * s.mark.x; K.rect(mx + 3, g.BR_Y - 9, 1, 8, 'K', s.mark.a); K.rect(mx + 4, g.BR_Y - 9, 5, 3, 'Y', s.mark.a); }
    // the worktree: a ghost while previewing, solid once applied
    if (s.wt.ghost > 0) K.trace(K.dashedBox(Wt.x, Wt.y, Wt.w, Wt.h), 'G', 1, s.wt.ghost, { dash: 2 });
    if (s.wt.a > 0) {
      K.trace(K.dashedBox(Wt.x, Wt.y, Wt.w, Wt.h), 'B', 1, s.wt.a, { dash: 2, shift: K.reduce ? 0 : Math.floor(t * 8) });
      K.label('WORKTREE', Wt.x + Wt.w / 2, Wt.y + Wt.h + 2, s.wt.a, 'B');
    }
    // inside: the local .handoff folder (never committed), the managed AGENTS.md section, the deltas
    if (s.dir.a > 0) { K.spr(K.SPR.folder, 144, 26, s.dir.a); K.label('.HANDOFF', 153, 38, s.dir.a); }
    if (s.dir.lock > 0) K.spr(K.SPR.lock, 163, 27, s.dir.lock);
    if (s.agents.a > 0) {
      K.spr(K.SPR.file, 188, 26, s.agents.a); K.label('AGENTS.MD', 192, 36, s.agents.a);
      if (s.agents.seg > 0) K.rect(190, 29, 3, 3, 'B', s.agents.seg);
    }
    if (s.deltas.a > 0) {
      if (s.deltas.hl > 0) K.rect(139, 44, 22, 12, 'Y', s.deltas.hl);
      K.spr(K.SPR.file, 141, 46, s.deltas.a); K.label('DELTAS', 147, 56, s.deltas.a);
      for (let k = 0; k < Math.floor(s.deltas.n); k++) K.rect(150, 48 + k * 2, 8 - (k % 2) * 3, 1, 'B', s.deltas.a);
    }
    K.drawChecklist(64, 20, ['iNote', 'iNote', 'iNote'], s.dash);
  },

  build(K, s, T) {
    const g = this.geo, C = g.CASE, { m, rv } = s;
    // 1 PACKAGE: atomic tasks are written in the case and checked by its gate
    T.step(0, t => {
      T.intro(t, [66, 36]);
      T.scene('>', 'L', 0);
      T.show(s.kase, '>', .4);
      T.say('WRITE THE PACKAGE', '<');
      T.face(m, 'left', '>', 'pointL');
      T.fly([66, 50], [C.x + 11, C.y + 9], 'B', '>+.2', .6, { shape: 'page', arc: 8 });
      T.show(s.kase, '>', .25, 'pkg');
      T.show(s.kase, '>+.2', .2, 'ok');
      T.say('ATOMIC TASKS: CHECKED', '<', 'B');
      T.face(m, 'happy', '<', 'down'); T.hop(m, '<');
      T.hide(s.kase, '>+.5', .2, 'ok');
    });

    // 2 PREVIEW: start without --apply lists what would happen and changes nothing
    T.step(1, t => {
      T.scene(t, 'CR', 0);
      // the repository the tasks go to, with its main branch
      T.say('THE REPOSITORY', t);
      T.face(m, 'right', t);
      T.glide(m, 98, 40, t, .7);
      T.show(s.repo, t + .2, .3);
      T.to(s.repo, { len: 1, duration: .9, ease: 'power1.out', sfx: 'grow' }, '<');
      T.say('PREVIEW THE HANDOFF', '>+.3');
      T.show(s.wt, '>', .4, 'ghost');
      T.show(s.branch, '<', .4, 'ghost');
      T.say('NOTHING APPLIED YET', '>+.3', 'B');
    });

    // 3 START: a worktree on a new issue branch cut from main; the task copy stays local
    T.step(2, t => {
      T.scene(t, null, 0);
      T.say('BRANCH FROM MAIN', t);
      T.face(m, 'right', t, 'point');
      T.hide(s.branch, t, .2, 'ghost');
      T.to(s.branch, { len: 1, duration: 1, ease: 'none', sfx: 'grow' }, t + .2);
      T.show(s.branch, '>-.4', .3, 'tag');
      T.hide(s.wt, '>', .2, 'ghost'); T.show(s.wt, '<', .4);
      T.say('WORKTREE CREATED', '<', 'B');
      T.fly([C.x + 11, C.y + 9], [152, 30], 'Y', '>+.2', .9, { shape: 'page', arc: 24 });
      T.show(s.dir, '>', .25);
      T.show(s.dir, '>+.1', .25, 'lock');
      T.say('.HANDOFF STAYS LOCAL', '<', 'B');
      T.show(s.agents, '>+.3', .25);
      T.show(s.agents, '>', .25, 'seg');
      T.say('KOS NEVER COMMITS', '>+.2', 'B');
      T.face(m, 'happy', '<', 'down');
    });

    // 4 BUILD: any agent works in the worktree; commits carry the handoff trailer, discoveries go to the deltas
    T.step(3, t => {
      T.scene(t, 'CR', 0);
      T.say('ANY AGENT CAN BUILD', t);
      T.S(rv, { x: 176, y: 44 }, t);
      T.poof(186, 52, t + .3); T.show(rv, '<', .25);
      T.show(s.deltas, '>', .25);
      g.BC.forEach((x, i) => {
        T.face(rv, 'open', '>+.2', 'point');
        T.glide(rv, x - 1, 50, '<', .5);
        T.fly([x, 72], [x, g.BR_Y], 'B', '>', .35, { arc: 0 });
        T.show(s.bc[i], '>', .15, 'a', 'commit');
        T.to(s.deltas, { n: (i + 1) * 1.4, duration: .3 }, '<');
        T.face(rv, 'happy', '<', 'down');
      });
      T.say('COMMITS: HANDOFF DH-001', '<', 'B');
    });

    // 5 STATUS: a read-only view of the tasks, commits and deltas
    T.step(4, t => {
      T.scene(t, 'C', 0);
      T.say('CHECK PROGRESS', t);
      T.face(m, 'up', t);
      T.glide(m, 64, 76, t, .6);
      T.show(s.dash, t + .3, .3);
      [1, 1, .6].forEach((v, i) => { T.to(s.dash.bars[i], { v, duration: .4 }, '>'); if (v === 1) T.show(s.dash.ticks[i], '>', .15, 'a', 'tick'); });
      T.say('READ ONLY', '>', 'B');
      T.hide(s.dash, '>+.9', .3);
    });

    // 6 RECONCILE: commits and deltas are read into the case as evidence; the worktree is never touched
    T.step(5, t => {
      T.scene(t, null, 0);
      T.say('RECONCILE INTO THE CASE', t);
      T.face(m, 'right', t);
      T.glide(m, 66, 70, t, .7);
      T.S(m, { beam: 1, beamDir: 'right' }, '>');
      // read every commit carrying the handoff trailer
      T.say('READ THE COMMITS', '<');
      s.bc.forEach((c, i) => {
        T.to(c, { hl: 1, duration: .15 }, `>${i ? '+.1' : ''}`);
        T.fly([g.BC[i], g.BR_Y - 2], [C.x + 12, C.y + 20], 'B', '>', .7, { arc: 22 });
        T.to(c, { hl: 0, duration: .2 }, '<.3');
      });
      T.show(s.kase.recs[0], '>', .25);
      T.say('COMMITS: DEMONSTRATED', '<', 'B');
      // then what the building agent noted down
      T.to(s.deltas, { hl: 1, duration: .15 }, '>+.3');
      T.say('READ THE DELTAS', '<');
      T.fly([149, 50], [C.x + 12, C.y + 31], 'B', '>', .8, { shape: 'page', arc: 24 });
      T.to(s.deltas, { hl: 0, duration: .2 }, '<.3');
      T.show(s.kase.recs[1], '>', .25);
      T.say('FINDINGS INTO THE CASE', '<', 'B');
      // a mark on the last imported commit: running reconcile again imports nothing twice
      T.S(s.mark, { x: 1 }, '>+.2'); T.show(s.mark, '<', .25, 'a', 'select');
      T.say('MARK ADVANCED', '<', 'B');
      T.S(m, { beam: 0 }, '<');
      T.say('RUN AGAIN: NOTHING NEW', '>+.8');
      T.S(m, { beam: 1, eyes: 'right' }, '<');
      T.to(s.bc[2], { hl: 1, duration: .15, yoyo: true, repeat: 1 }, '<.3');
      T.S(m, { beam: 0 }, '>+.2');
      T.show(s.task, '>+.3', .2, 'done');
      T.say('CASE STAYS OPEN', '<', 'B');
      T.face(m, 'happy', '<', 'up'); T.hop(m, '<');
      T.face(rv, 'happy', '<');
      T.S(m, { arms: 'wave' }, '>+.3');
    });
  },
});
