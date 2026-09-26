// INVESTIGATE (kos investigation): one case per line of work, written only through the CLI and gated on every write.
KF.flow({
  id: 'investigate',
  name: 'Investigate',
  task: 'INVESTIGATE',
  lede: 'One case per line of work. Every record goes through the CLI, and a claim without a source is refused.',
  alt: 'The agent opens a local case, a piece of evidence without a source is refused at the gate, the same evidence with file@commit is accepted, a finding is inferred from it, the case is published on a sync branch, the finding is absorbed into a vault note and the case is closed.',
  // Spanish: every English phrase shown on screen, mapped to its translation
  es: {
    "INVESTIGATE": "INVESTIGAR",
    "Investigate": "Investigar",
    "One case per line of work. Every record goes through the CLI, and a claim without a source is refused.": "Un caso por línea de trabajo. Cada registro pasa por el CLI y una afirmación sin fuente se rechaza.",
    "The agent opens a local case, a piece of evidence without a source is refused at the gate, the same evidence with file@commit is accepted, a finding is inferred from it, the case is published on a sync branch, the finding is absorbed into a vault note and the case is closed.": "El agente abre un caso local, el control rechaza una evidencia sin fuente, acepta la misma evidencia con file@commit, se infiere un hallazgo de ella, el caso se publica en una rama sync, el hallazgo se absorbe en una nota del vault y el caso se cierra.",
    "Open": "Abrir",
    "Refused": "Rechazo",
    "Evidence": "Evidencia",
    "Finding": "Hallazgo",
    "Publish": "Publicar",
    "Absorb": "Absorber",
    "Close": "Cerrar",
    "OPEN A CASE": "ABRE UN CASO",
    "LOCAL: NOT SHARED YET": "LOCAL: AÚN NO COMPARTIDO",
    "ADD EVIDENCE": "AGREGA EVIDENCIA",
    "NO SOURCE: REFUSED": "SIN FUENTE: RECHAZADA",
    "CITE THE SOURCE": "CITA LA FUENTE",
    "SOURCE: FILE@COMMIT": "FUENTE: FILE@COMMIT",
    "EVIDENCE E-001 SAVED": "EVIDENCIA E-001 GUARDADA",
    "RECORD A FINDING": "REGISTRA UN HALLAZGO",
    "INFERRED FROM E-001": "INFERIDO DE E-001",
    "MARKED FOR THE VAULT": "MARCADO PARA EL VAULT",
    "PUBLISH ON A SYNC BRANCH": "PUBLICA EN UNA RAMA SYNC",
    "ONE WAY: CASE SHARED": "SIN VUELTA: CASO COMPARTIDO",
    "UPDATE THE NOTE": "ACTUALIZA LA NOTA",
    "FINDING ABSORBED": "HALLAZGO ABSORBIDO",
    "CLOSE THE CASE": "CIERRA EL CASO",
    "CASE CLOSED: COMPLETED": "CASO CERRADO: COMPLETADO",
    "GATE": "CONTROL",
    "LOCAL CASE": "CASO LOCAL",
    "SHARED CASE": "CASO COMPARTIDO",
  },
  steps: [
    { label: 'Open', cmds: ['kos investigation new'] },
    { label: 'Refused', cmds: ['kos investigation add --kind evidence'] },
    { label: 'Evidence', cmds: ['kos investigation add --kind evidence --source file@commit'] },
    { label: 'Finding', cmds: ['kos investigation add --kind finding --from E-001'] },
    { label: 'Publish', cmds: ['kos sync start --name case-…'] },
    { label: 'Absorb', cmds: ['kos investigation absorb'] },
    { label: 'Close', cmds: ['kos investigation close --outcome completed'] },
  ],

  geo: { F: { x: 66, y: 32, w: 56, h: 40 }, GATE_X: 50 },

  state: K => ({
    repo: { a: 0 },
    gate: { a: 0, no: 0, yes: 0 },
    folder: { a: 0, pub: 0, closed: 0, sync: 0 },
    e: { a: 0 }, f: { a: 0, link: 0, tag: 0 },
    cite: { p: 0 },
  }),

  draw(K, s, t) {
    const { F, GATE_X: gx } = this.geo;
    if (s.repo.a > 0) { K.drawRepo(12, 24, s.repo, 1); K.label('CODE', 21, 40, s.repo.a); }
    // the CLI gate every write goes through
    if (s.gate.a > 0) {
      const a = s.gate.a;
      K.rect(gx - 1, 40, 2, 38, 'K', a);
      K.box(gx - 4, 33, 8, 7, s.gate.no > 0 ? 'R' : s.gate.yes > 0 ? 'B' : 'g', 'K', a);
      if (s.gate.no > 0) K.rect(gx + 1, 56, 10, 2, 'R', s.gate.no);
      if (s.gate.yes > 0) K.rect(gx + 1, 44, 2, 12, 'B', s.gate.yes);
      K.label('GATE', gx, 26, a);
    }
    // the case: local and unshared at first (dashed), shared once published (solid)
    if (s.folder.a > 0) {
      const a = s.folder.a, p = s.folder.pub;
      K.rect(F.x + 1, F.y + 1, F.w - 1, F.h - 1, 'W', a);
      K.rect(F.x, F.y - 4, 18, 4, p > .5 ? 'Y' : 'W', a);
      K.trace(K.seg(F.x, F.y - 4, F.x + 18, F.y - 4).concat(K.seg(F.x + 18, F.y - 4, F.x + 18, F.y)), 'K', 1, a, p > .5 ? {} : { dash: 1 });
      K.trace(K.dashedBox(F.x, F.y, F.w, F.h), 'K', 1, a * (1 - p), { dash: 1 });
      if (p > 0) { K.rect(F.x, F.y, F.w, 1, 'K', a * p); K.rect(F.x, F.y + F.h, F.w + 1, 1, 'K', a * p); K.rect(F.x, F.y, 1, F.h, 'K', a * p); K.rect(F.x + F.w, F.y, 1, F.h, 'K', a * p); }
      K.label(p > .5 ? 'SHARED CASE' : 'LOCAL CASE', F.x + F.w / 2, F.y + F.h + 4, a);
      if (s.folder.sync > 0) { K.rect(F.x + 30, F.y - 11, 24, 8, 'S', s.folder.sync); K.text('SYNC', F.x + 34, F.y - 10, 'B', s.folder.sync); }
      if (s.folder.closed > 0) K.spr(K.SPR.okBig, F.x + F.w - 6, F.y - 7, s.folder.closed);
    }
    // records: evidence E-001, and finding F-001 inferred from it
    if (s.e.a > 0) { K.spr(K.SPR.file, F.x + 6, F.y + 6, s.e.a); K.text('E-001', F.x + 16, F.y + 8, 'K', s.e.a); }
    if (s.cite.p > 0) K.trace(K.link(K.codeRowAt(12, 24, 1), [F.x + 6, F.y + 10], 20), 'B', s.cite.p);
    if (s.f.link > 0) K.trace(K.seg(F.x + 9, F.y + 15, F.x + 9, F.y + 22), 'G', s.f.link);
    if (s.f.a > 0) {
      K.spr(K.SPR.file, F.x + 6, F.y + 23, s.f.a); K.text('F-001', F.x + 16, F.y + 25, 'K', s.f.a);
      if (s.f.tag > 0) { const pulse = K.reduce ? 1 : .6 + .4 * Math.sin(t * 5); K.rect(F.x + 42, F.y + 25, 5, 5, 'C', s.f.tag * pulse); }
    }
  },

  build(K, s, T) {
    const { F, GATE_X: gx } = this.geo, { m } = s;
    const learned = T.node([4, -8]);

    // 1 OPEN: a new case starts in a local store, unpublished
    T.step(0, t => {
      T.intro(t, [22, 60]);
      T.scene('>', 'C', 0);
      T.show(s.vault, '>', .4);
      T.show(s.gate, '<', .3);
      T.say('OPEN A CASE', '<');
      T.face(m, 'right', '>', 'point');
      T.show(s.folder, '>+.2', .4);
      T.say('LOCAL: NOT SHARED YET', '>', 'B');
      T.face(m, 'happy', '<', 'down'); T.hop(m, '<');
    });

    // 2 REFUSED: evidence without a source never reaches the case
    T.step(1, t => {
      T.scene(t, 'LC', 0);
      T.say('ADD EVIDENCE', t);
      T.face(m, 'right', t, 'point');
      T.fly([42, 62], [gx - 2, 56], 'G', t + .3, .5, { shape: 'page', arc: 6 });
      T.show(s.gate, '>', .1, 'no');
      T.fly([gx - 2, 56], [30, 90], 'R', '<', .5, { shape: 'page', arc: 10 });
      T.say('NO SOURCE: REFUSED', '<', 'R');
      T.face(m, 'worried', '<', 'down'); T.shake(m, '<');
      T.hide(s.gate, '>+.6', .2, 'no');
    });

    // 3 EVIDENCE: the same record with file@commit is accepted as E-001
    T.step(2, t => {
      T.scene(t, 'LC', 0);
      T.show(s.repo, t, .3);
      T.say('CITE THE SOURCE', t);
      T.face(m, 'up', t);
      T.fly([30, 34], [34, 60], 'B', t + .3, .4, { arc: 4 });
      T.say('SOURCE: FILE@COMMIT', '>');
      T.face(m, 'right', '<', 'point');
      T.fly([42, 62], [F.x + 9, F.y + 10], 'B', '>+.2', .7, { shape: 'page', arc: 8 });
      T.show(s.gate, '<.2', .1, 'yes');
      T.show(s.e, '>', .25);
      T.to(s.cite, { p: 1, duration: .5, sfx: 'zip' }, '<');
      T.say('EVIDENCE E-001 SAVED', '<', 'B');
      T.face(m, 'happy', '<', 'down'); T.hop(m, '<');
      T.hide(s.gate, '>+.4', .2, 'yes');
    });

    // 4 FINDING: a conclusion is recorded from the evidence and marked for the vault
    T.step(3, t => {
      T.scene(t, 'C', 0);
      T.say('RECORD A FINDING', t);
      T.face(m, 'right', t, 'point');
      T.fly([42, 62], [F.x + 9, F.y + 27], 'B', t + .3, .7, { shape: 'page', arc: 8 });
      T.show(s.gate, '<.2', .1, 'yes');
      T.to(s.f, { link: 1, duration: .3 }, '>');
      T.show(s.f, '<', .25);
      T.say('INFERRED FROM E-001', '<', 'B');
      T.show(s.f, '>+.4', .3, 'tag');
      T.say('MARKED FOR THE VAULT', '<', 'B');
      T.face(m, 'happy', '<', 'down');
      T.hide(s.gate, '>+.3', .2, 'yes');
    });

    // 5 PUBLISH: on a sync branch the case moves to the shared store, one way
    T.step(4, t => {
      T.scene(t, 'C', 0);
      T.say('PUBLISH ON A SYNC BRANCH', t);
      T.face(m, 'right', t);
      T.show(s.folder, t + .2, .3, 'sync');
      T.to(s.folder, { pub: 1, duration: .8, ease: 'power1.inOut', sfx: 'grow' }, '>');
      T.say('ONE WAY: CASE SHARED', '<', 'B');
      T.face(m, 'happy', '<'); T.hop(m, '<');
    });

    // 6 ABSORB: the finding becomes part of a vault note
    T.step(5, t => {
      T.scene(t, 'CR', 1);
      T.say('UPDATE THE NOTE', t);
      T.face(m, 'right', t);
      T.glide(m, 130, 30, t, .8);
      T.fly([F.x + 44, F.y + 27], T.vaultAt(learned.p), 'C', '>', .8);
      T.join(learned, '>');
      T.hide(s.f, '<', .3, 'tag');
      T.say('FINDING ABSORBED', '<', 'B');
      T.face(m, 'happy', '<'); T.hop(m, '<');
    });

    // 7 CLOSE: the case is closed with its outcome
    T.step(6, t => {
      T.scene(t, 'C', 0);
      T.say('CLOSE THE CASE', t);
      T.face(m, 'left', t, 'pointL');
      T.hide(s.folder, t + .3, .2, 'sync');
      T.show(s.folder, '>', .3, 'closed');
      T.show(s.task, '<', .2, 'done');
      T.say('CASE CLOSED: COMPLETED', '<', 'B');
      T.face(m, 'happy', '<', 'up'); T.hop(m, '<');
      T.confetti(94, 50, '<');
      T.S(m, { arms: 'wave' }, '>+.3');
    });
  },
});
