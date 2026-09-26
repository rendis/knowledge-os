// ONBOARD (kos init, onboard-developer): the cell gets a vault once; each teammate's machine is set up once.
KF.flow({
  id: 'onboard',
  name: 'Onboard',
  task: 'ONBOARD',
  lede: 'The cell says who it is, where its code lives and where it runs; each teammate\'s machine is set up once.',
  alt: 'kos is installed on the machine, kos init asks who the cell is and creates the vault with its identity and kernel, a first discovery starts the map, then a teammate\'s machine is scanned read-only and gets its access card.',
  // Spanish: every English phrase shown on screen, mapped to its translation
  es: {
    "ONBOARD": "INCORPORAR",
    "Onboard": "Incorporar",
    "The cell says who it is, where its code lives and where it runs; each teammate's machine is set up once.": "La célula dice quién es, dónde vive su código y dónde corre; la máquina de cada compañero se configura una sola vez.",
    "kos is installed on the machine, kos init asks who the cell is and creates the vault with its identity and kernel, a first discovery starts the map, then a teammate's machine is scanned read-only and gets its access card.": "Se instala kos en la máquina, kos init pregunta quién es la célula y crea el vault con su identidad y su kernel, un primer descubrimiento empieza el mapa, y luego la máquina de un compañero se escanea en solo lectura y recibe su tarjeta de acceso.",
    "Install": "Instalar",
    "Ask": "Preguntar",
    "Create": "Crear",
    "Check": "Verificar",
    "Discover": "Descubrir",
    "Detect": "Detectar",
    "Confirm": "Confirmar",
    "INSTALL KOS ONCE": "INSTALA KOS UNA VEZ",
    "ONE KOS PER MACHINE": "UN KOS POR MÁQUINA",
    "WHO IS THE CELL?": "¿QUIÉN ES LA CÉLULA?",
    "WHERE IS THE CODE?": "¿DÓNDE ESTÁ EL CÓDIGO?",
    "WHICH BRANCHES?": "¿QUÉ RAMAS?",
    "WHICH CLOUDS?": "¿QUÉ NUBES?",
    "WHICH TRACKERS?": "¿QUÉ TRACKERS?",
    "CREATE THE VAULT": "CREA EL VAULT",
    "IDENTITY SAVED": "IDENTIDAD GUARDADA",
    "INSTALL THE KERNEL": "INSTALA EL KERNEL",
    "VERSION LOCKED": "VERSIÓN FIJADA",
    "KOS DOCTOR": "KOS DOCTOR",
    "INSTALLED AND CURRENT": "INSTALADO Y AL DÍA",
    "FIRST DISCOVERY": "PRIMER DESCUBRIMIENTO",
    "READ CLOUDS: ALLOWED": "LEER NUBES: PERMITIDO",
    "MAP STARTED": "MAPA INICIADO",
    "A TEAMMATE OPENS IT": "LO ABRE UN COMPAÑERO",
    "SCAN THIS MACHINE": "ESCANEA ESTA MÁQUINA",
    "READ ONLY: NOTHING SAVED": "SOLO LECTURA: NADA GUARDADO",
    "CONFIRM ONCE": "CONFIRMA UNA VEZ",
    "SAVED ON THIS MACHINE": "GUARDADO EN ESTA MÁQUINA",
    "ACCESS CARD READY": "TARJETA DE ACCESO LISTA",
    "YOU": "TÚ",
    "TEAMMATE": "COMPAÑERO",
  },
  steps: [
    { label: 'Install', cmds: ['gh release download … | sh'] },
    { label: 'Ask', cmds: ['kos init --vault ~/vaults/payments'] },
    { label: 'Create', cmds: ['kos init'] },
    { label: 'Check', cmds: ['kos doctor'] },
    { label: 'Discover', cmds: ['kos inventory', 'kos discover run', 'kos discover platform --referenced'] },
    { label: 'Detect', cmds: ['kos config detect'] },
    { label: 'Confirm', cmds: ['kos config workspace-init'] },
  ],

  state: K => ({
    cloud: { a: 0, ok: 0, name: 'GITHUB' },
    laptop: { a: 0, kos: 0 },
    mate: { a: 0 },
    found: K.A(4),
    card: { a: 0, lock: 0 },
    kernel: { a: 0, lock: 0 },
    repos: [0, 1, 2].map(() => ({ a: 0 })),
    platform: { a: 0 },
    ask: { a: 0, bars: [0, 1, 2, 3, 4].map(() => ({ v: 0 })), ticks: K.A(5), ok: 0 },
  }),

  draw(K, s) {
    // your machine, then kos on it
    if (s.laptop.a > 0) {
      K.spr(K.SPR.laptop, 12, 102, s.laptop.a); K.label('YOU', 21, K.FLOOR_Y + 4, s.laptop.a);
      if (s.laptop.kos > 0) { K.rect(14, 93, 15, 7, 'B', s.laptop.kos); K.text('KOS', 16, 94, 'O', s.laptop.kos); }
    }
    // the cell's answers to kos init
    K.drawChecklist(60, 24, ['iId', 'iRepo', 'iTree', 'iCloud', 'iTicket'], s.ask);
    // the kernel installed in the vault, pinned by its lock
    const vr = K.vaultRect();
    if (s.kernel.a > 0) K.spr(K.SPR.gear, vr.x + 3, vr.y + vr.h - 12, s.kernel.a);
    if (s.kernel.lock > 0) K.spr(K.SPR.lock, vr.x + 13, vr.y + vr.h - 11, s.kernel.lock);
    // what a first discovery reads
    s.repos.forEach((r, i) => K.drawRepo(64, 26 + i * 18, r, i % 3));
    if (s.platform.a > 0) { K.spr(K.SPR.cloudS, 68, 92, s.platform.a); K.label('CLOUD', 73, 101, s.platform.a); }
    const codeA = Math.max(...s.repos.map(r => r.a));
    if (codeA > 0) K.label('CODE', 73, 80, codeA);
    // a teammate's machine: what detect finds on it
    if (s.mate.a > 0) { K.spr(K.SPR.laptop, 74, 102, s.mate.a); K.label('TEAMMATE', 83, K.FLOOR_Y + 4, s.mate.a); }
    ['iRepo', 'iTree', 'iKey', 'iDb'].forEach((ic, i) => { const f = s.found[i]; if (f.a > 0) { K.spr(K.SPR[ic], 64 + i * 10, 86, f.a); } });
    if (s.card.a > 0) K.spr(K.SPR.card, 108, 94, s.card.a);
    if (s.card.lock > 0) K.spr(K.SPR.lock, 126, 96, s.card.lock);
  },

  build(K, s, T) {
    const { m } = s;
    // 1 INSTALL: kos, once per machine
    T.step(0, t => {
      T.intro(t, [40, 40]);
      T.scene('>', null, 0);
      T.show(s.laptop, '>', .3);
      T.show(s.cloud, '<', .3);
      T.say('INSTALL KOS ONCE', '<');
      T.face(m, 'right', '<');
      T.fly([184, 26], [21, 100], 'B', '>+.2', 1, { arc: 30 });
      T.show(s.laptop, '>', .25, 'kos');
      T.say('ONE KOS PER MACHINE', '<', 'B');
      T.face(m, 'happy', '<'); T.hop(m, '<');
    });

    // 2 ASK: kos init asks who the cell is, where its code lives, where it runs
    T.step(1, t => {
      T.scene(t, 'C', 0);
      T.hide(s.cloud, t, .3);
      T.face(m, 'open', t);
      T.glide(m, 140, 44, t, .8);
      T.show(s.ask, '<.3', .35);
      T.face(m, 'left', '>', 'pointL');
      ['WHO IS THE CELL?', 'WHERE IS THE CODE?', 'WHICH BRANCHES?', 'WHICH CLOUDS?', 'WHICH TRACKERS?'].forEach((q, i) => {
        T.say(q, i ? '>+.25' : '>');
        T.to(s.ask.bars[i], { v: 1, duration: .5, ease: 'power1.inOut' }, '>');
        T.show(s.ask.ticks[i], '>', .15, 'a', 'tick');
      });
      T.show(s.ask, '>+.1', .2, 'ok');
      T.face(m, 'happy', '<', 'down'); T.hop(m, '<');
    });

    // 3 CREATE: the vault is born with the cell's identity and one note per system
    T.step(2, t => {
      T.scene(t, null, 1);
      T.hide(s.ask, t, .3);
      T.say('CREATE THE VAULT', t);
      T.face(m, 'right', t);
      T.glide(m, 118, 60, t, .8);
      T.S(s.vault, { fill: 0 }, t);
      T.show(s.vault, t + .3, .5);
      [0, 1, 2].forEach(k => {
        T.fly([21, 98], T.vaultAt([-10 + k * 10, -2]), 'B', k ? '>-.4' : '>', .8, { shape: 'page', arc: 24 });
        T.to(s.vault, { fill: .04 * (k + 1), duration: .3 }, '>');
      });
      T.say('IDENTITY SAVED', '>', 'B');
      T.face(m, 'happy', '<'); T.hop(m, '<');
    });

    // 4 CHECK: the kernel (router, skills, gates) is installed and pinned; kos doctor confirms
    T.step(3, t => {
      T.scene(t, 'R', 1);
      T.say('INSTALL THE KERNEL', t);
      T.face(m, 'right', t);
      T.fly([21, 98], T.vaultAt([-30, 28]), 'B', t + .2, .9, { arc: 20 });
      T.show(s.kernel, '>', .3);
      T.show(s.kernel, '>+.2', .3, 'lock');
      T.say('VERSION LOCKED', '<', 'B');
      T.say('KOS DOCTOR', '>+.6');
      T.face(m, 'open', '<');
      T.show(s.vault, '>+.3', .2, 'ok');
      T.say('INSTALLED AND CURRENT', '<', 'B');
      T.face(m, 'happy', '<'); T.hop(m, '<');
      T.hide(s.vault, '+=.7', .2, 'ok');
    });

    // 5 DISCOVER: a first reading of the code and clouds starts the map
    T.step(4, t => {
      T.scene(t, 'CR', 1);
      s.repos.forEach((r, i) => T.show(r, t + i * .1, .3));
      T.show(s.platform, t + .3, .3);
      T.say('FIRST DISCOVERY', t);
      T.glide(m, 92, 18, t, .7);
      T.S(m, { eyes: 'left', beam: 1, beamDir: 'left', arms: 'pointL' }, '>');
      const s0 = T.tl.duration();
      T.to(m, { y: 72, duration: 2.2, ease: 'sine.inOut', sfx: 'scan' }, s0);
      [0, 1, 2, 3].forEach(k => T.fly([72, 32 + k * 16], T.vaultAt([-8 + (k % 2) * 12, -10 + k * 5]), 'C', s0 + .3 + k * .45, .8, { arc: 14 }));
      T.to(s.vault, { fill: .55, duration: 2.2, ease: 'none' }, s0 + .8);
      T.say('READ CLOUDS: ALLOWED', s0 + 1.4);
      T.S(m, { beam: 0, eyes: 'happy', arms: 'down' }, s0 + 2.4); T.hop(m, '<');
      T.say('MAP STARTED', '<', 'B');
    });

    // 6 DETECT: a teammate opens the vault; their machine is scanned without writing anything
    T.step(5, t => {
      T.scene(t, 'C', 0);
      s.repos.forEach(r => T.hide(r, t, .3)); T.hide(s.platform, t, .3);
      T.show(s.mate, t + .2, .4);
      T.say('A TEAMMATE OPENS IT', t);
      T.face(m, 'open', t);
      T.glide(m, 74, 60, t + .3, .8);
      T.S(m, { beam: 1, beamDir: 'down', eyes: 'up' }, '>');
      T.say('SCAN THIS MACHINE', '<');
      s.found.forEach((f, i) => T.show(f, `>+${i ? .2 : .3}`, .25, 'a', 'tick'));
      T.S(m, { beam: 0, eyes: 'happy' }, '>+.2');
      T.say('READ ONLY: NOTHING SAVED', '<', 'B');
    });

    // 7 CONFIRM: one confirmation writes the machine's config locally and prints the access card
    T.step(6, t => {
      T.scene(t, 'C', 0);
      T.face(m, 'open', t);
      T.say('CONFIRM ONCE', t);
      T.glide(m, 100, 56, t + .2, .6);
      T.show(s.card, '>', .35);
      T.show(s.card, '>+.2', .3, 'lock');
      T.say('SAVED ON THIS MACHINE', '<', 'B');
      T.show(s.task, '>+.3', .2, 'done');
      T.say('ACCESS CARD READY', '<', 'B');
      T.face(m, 'happy', '<', 'up'); T.hop(m, '<');
      T.confetti(110, 70, '<');
      T.hop(m, '>-.9');
      T.S(m, { arms: 'wave' }, '>');
    });
  },
});
