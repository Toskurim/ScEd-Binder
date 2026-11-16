'use strict';
const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const esc = s => String(s ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

const S = {
  maps: [], actions: new Map(), templates: [], profile: null, profRev: 0,
  devices: [], installs: [], sync: '', input: {}, view: { type: 'all' },
  filter: { q: '', map: '', mode: 'all', hidden: false }, follow: true,
  capture: null, placing: null, saving: Promise.resolve(), tplCache: {},
};

async function api(path, body) {
  const opt = body === undefined ? {} : { method: 'POST', headers: { 'X-SCB': '1', 'Content-Type': 'application/json' }, body: JSON.stringify(body) };
  const r = await fetch(path, opt);
  const j = await r.json().catch(() => ({}));
  if (!r.ok) throw new Error(j.error || ('Errore ' + r.status));
  return j;
}

function toast(msg, err) {
  const t = $('#toast');
  t.textContent = msg; t.className = 'toast' + (err ? ' err' : '');
  clearTimeout(toast.t); toast.t = setTimeout(() => t.classList.add('hidden'), err ? 7000 : 3500);
}

// ---------- nomi leggibili ----------
const AXN = { x: 'Asse X', y: 'Asse Y', z: 'Asse Z', rotx: 'Asse Rot X', roty: 'Asse Rot Y', rotz: 'Asse Rot Z', slider1: 'Slider 1', slider2: 'Slider 2' };
const DIRN = { up: 'su', down: 'giù', left: 'sinistra', right: 'destra' };
function inputLabel(i) {
  let m;
  if ((m = /^button(\d+)$/.exec(i))) return 'Pulsante ' + m[1];
  if ((m = /^hat(\d+)_(up|down|left|right)$/.exec(i))) return 'Hat ' + m[1] + ' ' + DIRN[m[2]];
  return AXN[i] || i;
}
function inputShort(i) {
  let m;
  if ((m = /^button(\d+)$/.exec(i))) return 'P' + m[1];
  if ((m = /^hat(\d+)_(up|down|left|right)$/.exec(i))) return 'H' + m[1] + ({ up: '↑', down: '↓', left: '←', right: '→' }[m[2]]);
  return ({ x: 'X', y: 'Y', z: 'Z', rotx: 'RX', roty: 'RY', rotz: 'RZ', slider1: 'SL1', slider2: 'SL2' }[i]) || i;
}
const isJsDev = d => d && !['kb', 'mo', 'gp', 'jsclear'].includes(d);
const devInfo = k => S.profile.devices[k] || { name: k, hotspots: [] };
const devName = k => { const d = devInfo(k); return (d.alias || d.name || k).trim(); };
const liveDev = k => S.devices.find(d => d.key === k);
const jsNum = k => { const d = liveDev(k); return d ? d.js : 0; };
const isIgnored = (k, input) => ((S.profile.devices[k] || {}).ignored || []).includes(input);
function invertState(k, group) { const d = S.profile.devices[k] || {}; const v = (d.invert || {})[group]; return v === undefined ? !!S.invertDefaults[group] : !!v; }
function setInvert(k, group, val) { const d = devInfo(k); d.invert = d.invert || {}; d.invert[group] = val; saveProfile(); }
const actKey = (m, a) => m + '/' + a;
const actOf = b => S.actions.get(actKey(b.map, b.action));
const mapLabel = m => { const x = S.maps.find(y => y.name === m); return x ? x.label : m; };
const actLabel = b => { const a = actOf(b); return a ? a.label : b.action; };
function kbLabel(b) {
  if (b.input.trim() === '') return 'disattivato';
  const pre = b.dev === 'mo' ? 'Mouse: ' : '';
  if (b.dev !== 'gp') return pre + b.input.split('+').map(k => KBN[k] || KEYN[k] || k.toUpperCase()).join(' + ');
  return pre + b.input.split('+').map(k => KEYN[k] || k.toUpperCase()).join(' + ');
}
const KBN = { a: 'A', b: 'B', x: 'X', y: 'Y' };
const fmtKeys = s => s.split('+').map(k => KEYN[k] || k.toUpperCase()).join(' + ');
const KEYN = { a: 'A', b: 'B', x: 'X', y: 'Y', shoulderl: 'LB', shoulderr: 'RB', triggerl_btn: 'LT', triggerr_btn: 'RT', thumbl: 'L3', thumbr: 'R3', back: 'Back', start: 'Start', dpad_up: 'D-pad su', dpad_down: 'D-pad giù', dpad_left: 'D-pad sx', dpad_right: 'D-pad dx', thumblx: 'Levetta sx ↔', thumbly: 'Levetta sx ↕', thumbrx: 'Levetta dx ↔', thumbry: 'Levetta dx ↕', maxis_x: 'Mouse asse X', maxis_y: 'Mouse asse Y', maxis_z: 'Rotella (asse)', comma: ',', period: '.', minus: '-', equals: '=', slash: '/', lbracket: '[', rbracket: ']', semicolon: ';', apostrophe: "'", tilde: '\\', backslash: '\\', lalt: 'Alt sx', ralt: 'Alt dx', lctrl: 'Ctrl sx', rctrl: 'Ctrl dx', lshift: 'Maiusc sx', rshift: 'Maiusc dx', space: 'Spazio', enter: 'Invio', escape: 'Esc', backspace: 'Backspace', tab: 'Tab', up: '↑', down: '↓', left: '←', right: '→', mouse1: 'Clic sinistro', mouse2: 'Clic destro', mouse3: 'Clic centrale', mouse4: 'Mouse 4', mouse5: 'Mouse 5', mwheel_up: 'Rotella su', mwheel_down: 'Rotella giù', pgup: 'Pag su', pgdn: 'Pag giù', insert: 'Ins', delete: 'Canc', home: 'Home', end: 'Fine', capslock: 'Bloc Maiusc' };

// ---------- profilo ----------
function saveProfile() {
  const snap = JSON.stringify(S.profile);
  S.saving = S.saving.then(async () => {
    try { const r = await api('/api/profile', JSON.parse(snap)); S.profRev = r.profRev; }
    catch (e) { toast('Profilo non salvato: ' + e.message, true); }
  });
  return S.saving;
}
function bindingsOf(m, a) { return S.profile.bindings.filter(b => b.map === m && b.action === a); }
function assignJS(act, devKey, input, extra) {
  const before = S.profile.bindings.filter(b => b.map === act.map && b.action === act.name && isJsDev(b.dev));
  S.profile.bindings = S.profile.bindings.filter(b => !(b.map === act.map && b.action === act.name && (isJsDev(b.dev) || b.dev === 'jsclear')));
  S.profile.bindings.push(Object.assign({ map: act.map, action: act.name, dev: devKey, input }, extra || {}));
  saveProfile(); render();
  if (before.length) toast('Sostituita l\'assegnazione precedente (' + inputLabel(before[0].input) + ' su ' + devName(before[0].dev) + ')');
}
function assignKBM(act, dev, input, extra) {
  S.profile.bindings = S.profile.bindings.filter(b => !(b.map === act.map && b.action === act.name && (b.dev === 'kb' || b.dev === 'mo')));
  S.profile.bindings.push(Object.assign({ map: act.map, action: act.name, dev, input }, extra || {}));
  saveProfile(); render();
}
function removeBinding(b) {
  S.profile.bindings = S.profile.bindings.filter(x => x !== b);
  saveProfile(); render();
}

// ---------- layout (più insiemi di assegnazioni) ----------
function layoutNames() {
  const p = S.profile; p.layouts = p.layouts || []; p.layout = p.layout || 'Principale';
  if (!p.layouts.some(l => l.name === p.layout)) p.layouts.push({ name: p.layout, bindings: [], invert: {} });
  return p.layouts.map(l => l.name);
}
function stashLayout() { // copia le assegnazioni correnti dentro il layout attivo
  layoutNames();
  const p = S.profile, l = p.layouts.find(x => x.name === p.layout);
  l.bindings = JSON.parse(JSON.stringify(p.bindings)); l.invert = {};
  for (const k in p.devices) l.invert[k] = Object.assign({}, p.devices[k].invert || {});
}
function applyLayout(name) { // carica un layout come assegnazioni correnti
  const p = S.profile, l = p.layouts.find(x => x.name === name); if (!l) return;
  p.layout = name; p.bindings = JSON.parse(JSON.stringify(l.bindings || []));
  for (const k in p.devices) p.devices[k].invert = Object.assign({}, (l.invert || {})[k] || {});
  S.learn = null; saveProfile().then(pollState); render();
}
function switchLayout(name) { if (name === S.profile.layout) return; stashLayout(); applyLayout(name); toast('Layout "' + name + '" caricato nell\'app. Per usarlo nel gioco premi "Scrivi nel gioco".'); }
function askLayoutName(msg, def) {
  const v = (prompt(msg, def || '') || '').trim(); if (!v) return null;
  if (layoutNames().includes(v)) { toast('Esiste già un layout con questo nome', true); return null; }
  return v.slice(0, 40);
}
function layoutAction(a) {
  const p = S.profile; layoutNames();
  if (a === 'new') { const n = askLayoutName('Nome del nuovo layout (parte vuoto):'); if (!n) return; stashLayout(); p.layouts.push({ name: n, bindings: [], invert: {} }); applyLayout(n); }
  if (a === 'dup') { const n = askLayoutName('Nome della copia di "' + p.layout + '":', p.layout + ' 2'); if (!n) return; stashLayout(); const cur = p.layouts.find(x => x.name === p.layout); p.layouts.push({ name: n, bindings: JSON.parse(JSON.stringify(cur.bindings)), invert: JSON.parse(JSON.stringify(cur.invert)) }); applyLayout(n); }
  if (a === 'ren') { const n = askLayoutName('Nuovo nome per "' + p.layout + '":', p.layout); if (!n) return; p.layouts.find(x => x.name === p.layout).name = n; p.layout = n; saveProfile(); render(); }
  if (a === 'del') {
    if (p.layouts.length < 2) return toast('È l\'unico layout: non si può eliminare', true);
    if (!confirm('Elimino il layout "' + p.layout + '" con tutte le sue assegnazioni?\n(Il file già scritto nel gioco non viene toccato.)')) return;
    p.layouts = p.layouts.filter(x => x.name !== p.layout); applyLayout(p.layouts[0].name);
  }
}

// ---------- avvio e aggiornamento ----------
async function boot() {
  const r = await api('/api/boot');
  S.maps = r.maps || []; S.templates = r.templates || []; S.profile = r.profile; S.profRev = r.profRev;
  S.invertDefaults = r.invertDefaults || {}; S.photoDevices = (r.photoDevices || []).slice().sort((a, b) => (a.brand + a.name).localeCompare(b.brand + b.name)); S.photoById = {}; for (const p of S.photoDevices) S.photoById[p.id] = p; S.gameData = r.gameData; S.version = r.version; S.dataDir = r.dataDir;
  for (const m of S.maps) for (const a of (m.actions || [])) S.actions.set(actKey(a.map, a.name), a);
  S.game = S.profile.game === 'ed' ? 'ed' : 'sc';
  if (isED()) { try { S.ed = await api('/api/ed/state'); S.edCmd = {}; for (const c of (S.ed.commands || [])) S.edCmd[c.tag] = c; } catch (e) { } }
  $('#ver').textContent = 'v' + r.version;
  document.title = 'ScEd Binder v' + r.version;
  applyState(r);
  // primo avvio: leggo subito quello che c'è nel gioco
  const inst = curInstall();
  if (inst && inst.exists && !S.profile.imported && S.profile.bindings.length === 0) await doImport(true);
  render();
  setInterval(pollState, 2000);
  pollInput();
}
function curInstall() {
  return S.installs.find(i => i.path.toLowerCase() === (S.profile.install || '').toLowerCase())
    || S.installs.find(i => i.launcher) || S.installs.find(i => i.name.toUpperCase() === 'LIVE') || S.installs[0];
}
function applyState(r) {
  S.gameOrder = r.gameOrder || []; S.advanced = r.advanced || {};
  const sig = JSON.stringify([r.devices, r.installs, r.sync, r.gameOrder, r.advanced]);
  const changed = sig !== S.sig;
  S.sig = sig; S.devices = r.devices || []; S.installs = r.installs || []; S.sync = r.sync || '';
  return changed;
}
async function pollState() {
  try {
    const r = await api('/api/state');
    if (r.version && S.version && r.version !== S.version) { location.reload(); return; } // è partita una versione nuova
    let changed = applyState(r);
    if (isED()) { const e = await api('/api/ed/state'); const sg = JSON.stringify([e.file, e.sync, e.devIds, e.disk]); if (sg !== S.edSig) { S.edSig = sg; changed = true; } S.ed = e; if (!S.edCmd || !Object.keys(S.edCmd).length) { S.edCmd = {}; for (const c of (e.commands || [])) S.edCmd[c.tag] = c; changed = true; } }
    if (r.profRev !== S.profRev) {
      await S.saving;
      const p = await api('/api/profile');
      if (p.profRev !== S.profRev) { S.profile = p.profile; S.profRev = p.profRev; changed = true; }
    }
    if (changed && !S.capture) render();
  } catch (e) { /* app chiusa */ }
}
async function pollInput() {
  try {
    const st = await api('/api/input');
    const map = {};
    for (const s of (st.d || [])) map[s.key] = s;
    S.gp = st.gp || { on: false, p: [], a: {} };
    const prev = S.input; S.input = map;
    updateLive(prev);
    if (S.capture && S.capture.kind === 'js') captureTick();
    if (S.capture && S.capture.kind === 'gp') captureGP();
    if (S.capture && S.capture.kind === 'edjs') captureTickED();
  } catch (e) { }
  setTimeout(pollInput, 60);
}
function activeInputs(s) {
  const out = new Set();
  if (!s) return out;
  for (const b of s.b || []) if (!isIgnored(s.key, 'button' + b)) out.add('button' + b);
  (s.h || []).forEach((v, i) => { const d = hatDir(v); if (d && !isIgnored(s.key, 'hat' + (i + 1) + '_' + d)) out.add('hat' + (i + 1) + '_' + d); });
  return out;
}
function hatDir(v) { return v === 0 ? 'up' : v === 9000 ? 'right' : v === 18000 ? 'down' : v === 27000 ? 'left' : ''; }

function updateLive(prev) {
  // lampeggio nella lista periferiche
  for (const el of $$('.dev[data-key]')) {
    const s = S.input[el.dataset.key];
    $('.badge', el)?.classList.toggle('live', activeInputs(s).size > 0);
  }
  if (S.view.type !== 'dev') return;
  const key = S.view.key, s = S.input[key];
  if (!$('.inpcol')) return;
  const act = activeInputs(s), now = Date.now();
  // un asse "si accende" mentre lo muovi
  S.axisHot = S.axisHot || {};
  if (s && prev[key]) for (const a in (s.a || {})) { const p = (prev[key].a || {})[a]; if (p !== undefined && Math.abs(s.a[a] - p) > 300 && !isIgnored(key, a)) S.axisHot[key + '|' + a] = now; }
  for (const k in S.axisHot) { const [dk, a] = k.split('|'); if (dk === key && now - S.axisHot[k] < 700) act.add(a); }
  const old = S.prevActKey === key && S.prevAct ? S.prevAct : act;
  S.prevAct = act; S.prevActKey = key;
  for (const el of $$('[data-live]')) el.classList.toggle('live', act.has(el.dataset.live));
  if (s) for (const el of $$('.bar i[data-axis]')) el.style.width = Math.round(((s.a || {})[el.dataset.axis] ?? 32767) / 655.35) + '%';
  let fresh = null;
  for (const i of act) if (!old.has(i)) fresh = i;
  if (fresh && !S.capture && S.learn) {
    const img = devInfo(key).image || '';
    if (img.startsWith('foto:')) setPhotoInput(key, img.slice(5), S.learn.ctl, S.learn.idx, fresh);
    S.learn = null; renderDevice(); return;
  }
  if (fresh && !S.capture) {
    if (S.placingMode) setPlacing(fresh);
    if (S.follow) $('.row[data-live="' + fresh + '"]')?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }
}

// ---------- disegno ----------
function render() {
  renderHeader(); renderSidebar();
  if (S.view.type === 'dev' && !S.profile.devices[S.view.key]) S.view = { type: 'all' };
  document.body.classList.toggle('game-ed', isED());
  if (isED() && S.view.type === 'ship') S.view = { type: 'all' };
  if (S.view.type === 'dev') renderDevice(); else if (S.view.type === 'ship') renderShip(); else if (isED()) renderAllED(); else renderAll();
}
function renderHeader() {
  const sel = $('#install'), cur = curInstall();
  sel.innerHTML = S.installs.map(i => `<option value="${esc(i.path)}" ${cur && i.path === cur.path ? 'selected' : ''}>${esc(i.name)} — ${esc(i.path)}${i.launcher ? '  ★ avviato dal launcher' : ''}${i.lastRun ? '  (ultimo avvio ' + new Date(i.lastRun * 1000).toLocaleDateString('it-IT') + ')' : '  (mai avviato)'}</option>`).join('') || '<option>non trovata</option>';
  $('#layout').innerHTML = layoutNames().map(n => `<option ${n === S.profile.layout ? 'selected' : ''}>${esc(n)}</option>`).join('');
  const sy = $('#sync'), ban = [];
  const map = { ok: ['ok', '● Il gioco è allineato'], todo: ['todo', '● Modifiche da scrivere nel gioco'], ext: ['ext', '● File del gioco cambiato fuori dall\'app'] };
  $('#game').value = S.game;
  const m = isED() ? (map[(S.ed || {}).sync] || ['', '']) : (map[S.sync] || ['', '']);
  sy.className = 'sync ' + m[0]; sy.textContent = m[1];
  if (isED()) {
    const E = S.ed || {}; const eb = [];
    if (E.file && E.file.error) eb.push(`<div class="bad">${esc(E.file.error)}</div>`);
    else if (E.file) eb.push(`<div class="info">Elite Dangerous — file dei comandi: <b>${esc(E.file.path)}</b>${(E.file.start || []).some(s => s !== E.file.preset) ? ` · Attenzione: nel gioco per alcune modalità è selezionato un altro preset (${esc([...new Set(E.file.start)].join(', '))}); l'app modifica "${esc(E.file.preset)}".` : ''}</div>`);
    if (!S.devices.some(d => !d.gamepad)) eb.push('<div class="info">Nessun joystick rilevato in questo momento.</div>');
    $('#banner').innerHTML = eb.join(''); return;
  }
  if (!S.installs.length) ban.push(`<div class="bad">Non trovo Star Citizen su questo PC. Indica la cartella (es. D:\\Giochi\\StarCitizen oppure ...\\StarCitizen\\LIVE): <input type="text" id="rootInput" size="40"> <button id="rootSave">Cerca</button> <button id="rootBrowse">Sfoglia…</button> — oppure <button data-b="savefile">Salva il file nella cartella dell'app</button> e copialo tu nel gioco.</div>`);
  const used = S.installs.find(i => i.launcher);
  if (cur && used && !cur.launcher) ban.push(`<div class="bad"><b>Attenzione: stai lavorando sulla cartella sbagliata.</b> Il launcher avvia Star Citizen da <b>${esc(used.path)}</b>, ma l'installazione selezionata è <b>${esc(cur.path)}</b>: quello che scrivi lì il gioco non lo legge. <button data-b="useinst" class="primary">Usa ${esc(used.path)}</button></div>`);
  else if (cur && !used && S.installs.length > 1) ban.push(`<div class="info">Ho trovato più copie di Star Citizen. Controlla nel menu "Installazione" di avere scelto quella che avvii davvero (nel launcher RSI: Impostazioni → cartella della libreria).</div>`);
  if (S.profile.orderManual) {
    const on = S.profile.order.filter(k => liveDev(k)), go = (S.gameOrder || []).filter(k => on.includes(k));
    const differs = go.length && on.filter(k => go.includes(k)).join() !== go.join();
    ban.push(`<div class="${differs ? 'warn' : 'info'}">Numerazione scelta da te con le frecce${differs ? ' — <b>diversa da quella che risulta nel file del gioco</b>. Se dopo aver giocato i comandi finiscono sulla periferica sbagliata, il gioco non ha accettato il tuo ordine' : ''}. <button data-b="autoorder">Segui la numerazione del gioco</button></div>`);
  }
  if (S.sync === 'ext') ban.push(`<div class="warn">Nel gioco ci sono comandi diversi da quelli dell'app (modificati dentro il gioco, oppure è un'altra installazione). <button data-b="import">Importa dal gioco</button> (prende i comandi del gioco) oppure <button data-b="write">Scrivi nel gioco</button> (tiene quelli dell'app; la numerazione js resta quella del gioco).</div>`);
  const off = S.profile.order.filter(k => !liveDev(k) && S.profile.bindings.some(b => b.dev === k));
  if (off.length) ban.push(`<div class="info">Periferiche scollegate: <b>${off.map(k => esc(devName(k))).join(', ')}</b>. Le loro assegnazioni restano salvate nell'app e tornano attive quando le ricolleghi; intanto le altre periferiche vengono rinumerate.</div>`);
  if (!S.devices.some(d => !d.gamepad)) ban.push(`<div class="info">Nessun joystick rilevato in questo momento. Collega le periferiche: compaiono da sole entro qualche secondo.</div>`);
  $('#banner').innerHTML = ban.join('');
}
function renderSidebar() {
  $('#navAll').classList.toggle('sel', S.view.type === 'all');
  $('#navShip').classList.toggle('sel', S.view.type === 'ship');
  const tot = S.profile.bindings.filter(b => b.input.trim() !== '').length;
  $('#navAllCount').textContent = isED() ? 'Elite Dangerous' : tot + ' assegnazioni in totale';
  const keys = S.profile.order.filter(k => S.profile.devices[k]);
  const on = keys.filter(k => liveDev(k)), offl = keys.filter(k => !liveDev(k));
  const item = (k, i, list) => {
    const n = jsNum(k), cnt = isED() ? edCountFor(k) : S.profile.bindings.filter(b => b.dev === k).length;
    return `<div class="dev ${n ? '' : 'off'} ${S.view.type === 'dev' && S.view.key === k ? 'sel' : ''}" data-key="${esc(k)}">
      <span class="badge ${n ? '' : 'off'}">${n ? 'js' + n : 'scollegata'}</span>
      <div><div class="name" title="${esc(devName(k))}">${esc(devName(k))}</div><div class="sub">${cnt} comandi assegnati</div></div>
      ${n ? `<div class="arrows"><button data-mv="-1" ${i === 0 ? 'disabled' : ''} title="Sposta su (numero più basso)">▲</button><button data-mv="1" ${i === list.length - 1 ? 'disabled' : ''} title="Sposta giù">▼</button></div>` : '<span></span>'}
    </div>`;
  };
  $('#devlist').innerHTML = on.map(item).join('') + offl.map(item).join('') || '<p class="hint">Nessuna periferica.</p>';
  $('#padlist').innerHTML = S.devices.filter(d => d.gamepad).map(d => `<div class="pad">🎮 ${esc(d.name)} — gamepad (il gioco lo gestisce a parte, non occupa un numero js)</div>`).join('');
}

function chipJS(b) {
  if (b.dev === 'jsclear') return `<span class="def">predefinito disattivato</span>`;
  const n = jsNum(b.dev), a = actOf(b);
  const extra = (a && a.canInvert && AXN[b.input] && invertState(b.dev, a.optGroup) ? ' ⇅invertito' : '') + (b.multiTap === '2' ? ' ×2' : '') + (b.mode ? ' [' + esc(b.mode) + ']' : '');
  return `<span class="chip ${n ? '' : 'off'}" title="${esc(devName(b.dev))}">${n ? 'js' + n : 'scollegata'} · ${esc(inputLabel(b.input))}${extra} <small>${esc(devName(b.dev).slice(0, 22))}</small></span>`;
}
function renderAll() {
  const f = S.filter, q = f.q.trim().toLowerCase();
  let rows = '', shown = 0;
  for (const m of S.maps) {
    if (f.map && m.name !== f.map) continue;
    let part = '';
    for (const a of (m.actions || [])) {
      if (!a.visible && !f.hidden) continue;
      const bs = bindingsOf(a.map, a.name);
      const js = bs.filter(b => isJsDev(b.dev)), kb = bs.filter(b => b.dev === 'kb' || b.dev === 'mo'), gp = bs.filter(b => b.dev === 'gp');
      if (f.mode === 'bound' && !js.length) continue;
      if (f.mode === 'unbound' && js.length) continue;
      if (q && !(a.label + ' ' + a.labelEn + ' ' + a.name).toLowerCase().includes(q)) continue;
      const defK = [a.defKb, a.defMo].filter(x => x).map(fmtKeys).join(' / ');
      const clr = bs.some(b => b.dev === 'jsclear');
      part += `<tr data-a="${esc(actKey(a.map, a.name))}">
        <td><div class="lab" title="${esc(a.desc || '')}">${esc(a.label)}${a.axis ? '<span class="ax">asse</span>' : ''}</div><div class="en">${esc(a.labelEn)}</div></td>
        <td class="c" data-cap="kbm">${kb.map(b => `<span class="chip kb">${esc(kbLabel(b))}${b.multiTap === '2' ? ' ×2' : ''}</span>`).join('') || (defK ? `<span class="def">pred.: ${esc(defK)}</span>` : '<span class="def">—</span>')}</td>
        <td class="c" data-cap="js">${js.map(chipJS).join('') || (clr ? '<span class="def">—</span>' : (a.defJs ? `<span class="def">pred.: ${esc(inputLabel(a.defJs))}${S.profile.clearJsDefaults ? ' (verrà disattivato)' : ''}</span>` : '<span class="def">—</span>'))}</td>
        <td class="c" data-cap="gp">${gp.map(b => `<span class="chip kb">${esc(kbLabel(b))}</span>`).join('') || (a.defGp ? `<span class="def">pred.: ${esc(fmtKeys(a.defGp))}</span>` : '<span class="def">—</span>')}</td></tr>`;
      shown++;
    }
    if (part) rows += `<tr class="grp"><td colspan="4">${esc(m.label)}<small>${esc(m.category || '')}</small></td></tr>` + part;
  }
  $('#view').innerHTML = `<div class="toolbar">
      <input type="search" id="q" placeholder="Cerca un comando (italiano o inglese)…" value="${esc(f.q)}">
      <select id="fmap"><option value="">Tutte le categorie</option>${S.maps.filter(m => (m.actions || []).some(a => a.visible || f.hidden)).map(m => `<option value="${esc(m.name)}" ${f.map === m.name ? 'selected' : ''}>${esc(m.label)}</option>`).join('')}</select>
      <select id="fmode"><option value="all">Tutti</option><option value="bound" ${f.mode === 'bound' ? 'selected' : ''}>Solo assegnati a joystick</option><option value="unbound" ${f.mode === 'unbound' ? 'selected' : ''}>Solo non assegnati</option></select>
      <label><input type="checkbox" id="fhid" ${f.hidden ? 'checked' : ''}> comandi nascosti</label>
      <small>${shown} comandi</small></div>
    <div class="scroll"><table class="cmd"><thead><tr><th>Comando</th><th>Tastiera / mouse</th><th>Joystick, manetta, pedali</th><th>Gamepad</th></tr></thead><tbody>${rows || '<tr><td colspan="4">Nessun comando trovato.</td></tr>'}</tbody></table></div>`;
}

function deviceInputs(key) {
  const d = liveDev(key), bs = isED() ? [] : S.profile.bindings.filter(b => b.dev === key);
  let nb = d ? d.buttons : 0, axes = d ? [...(d.axes || [])] : [], hats = d ? d.hats : 0;
  if (isED()) { const id = edDevId(key), B = edB(); for (const tag in B) for (const sl of ['p', 's', 'a']) { const s = B[tag][sl]; if (edEmpty(s) || s.d !== id[0] || (s.i || '') !== (id[1] || '')) continue; const p = edParseKey(s.k); if (p) bs.push({ input: p.input }); } }
  for (const b of bs) {
    let m;
    if ((m = /^button(\d+)$/.exec(b.input))) nb = Math.max(nb, +m[1]);
    else if ((m = /^hat(\d+)_/.exec(b.input))) hats = Math.max(hats, +m[1]);
    else if (AXN[b.input] && !axes.includes(b.input)) axes.push(b.input);
  }
  return { nb, axes, hats, axisNames: d ? (d.axisNames || []) : [] };
}
function renderDevice() {
  const key = S.view.key, di = devInfo(key), d = liveDev(key), n = jsNum(key);
  const inp = deviceInputs(key);
  const rowFor = (input, extra) => {
    const bs = S.profile.bindings.filter(b => b.dev === key && b.input === input);
    const ign = isIgnored(key, input);
    const invBox = b => { const a = actOf(b); return a && a.canInvert && AXN[input] ? `<label class="inv" title="Inverte questo movimento per questa periferica (viene scritto nel gioco)"><input type="checkbox" data-inv="${esc(a.optGroup)}" ${invertState(key, a.optGroup) ? 'checked' : ''}> inverti</label>` : ''; };
    return `<div class="row ${S.placing === input ? 'placing' : ''} ${ign ? 'ign' : ''}" data-live="${input}" data-input="${input}">
      <span class="led"></span><div><b>${esc(inputLabel(input))}</b>${extra || ''}</div>
      <div>${isED() ? edChips(key, input) : bs.map((b, i) => `<span class="chip" title="${esc((actOf(b) || {}).labelEn || '')}">${esc(actLabel(b))}${b.multiTap === '2' ? ' ×2' : ''} <small>${esc(mapLabel(b.map))}</small> <span class="x" data-rm="${i}" title="Rimuovi">×</span></span>${invBox(b)}`).join('')}${ign ? '<span class="def">ignorato: l\'app non lo rileva</span>' : ''}</div>
      <div><button class="small" data-add="1" title="Assegna un comando a questo ingresso">+ comando</button> <button class="small" data-ign="1" title="${ign ? 'Torna a rilevare questo ingresso' : 'Ignora questo ingresso: non viene più rilevato né evidenziato (utile per interruttori sempre accesi o assi che tremano)'}">${ign ? 'riattiva' : 'ignora'}</button>${(di.image || '').startsWith('img:') ? ` <button class="small" data-place="1" title="Posiziona l'etichetta sull'immagine">📍</button>` : ''}</div></div>`;
  };
  let list = '';
  if (inp.axes.length) list += '<div class="sect">Assi</div>' + inp.axes.map((a, i) => rowFor(a, `<div class="bar"><i data-axis="${a}"></i></div>`)).join('');
  if (inp.hats) { list += '<div class="sect">Hat (levette a 4 direzioni viste come POV)</div>'; for (let h = 1; h <= inp.hats; h++) for (const dir of ['up', 'down', 'left', 'right']) list += rowFor('hat' + h + '_' + dir); }
  if (inp.nb) { list += '<div class="sect">Pulsanti</div>'; for (let b = 1; b <= inp.nb; b++) list += rowFor('button' + b); }
  if (!list) list = '<p class="hint" style="padding:12px">Periferica scollegata e senza assegnazioni.</p>';
  const img = di.image || '';
  $('#view').innerHTML = `<div class="toolbar">
      <span class="badge ${n ? '' : 'off'}">${n ? 'js' + n : 'scollegata'}</span><h2>${esc(devName(key))}</h2>
      <button class="small" id="rename">Rinomina</button>
      <small>${esc(di.guid || '')}${d ? ` · ${d.buttons} pulsanti, ${(d.axes || []).length} assi, ${d.hats} hat` : ''}</small>
      <div class="spacer"></div>
      <label><input type="checkbox" id="follow" ${S.follow ? 'checked' : ''}> segui il tasto premuto</label>
      ${n ? '' : '<button class="small danger" id="forget">Dimentica periferica</button>'}
    </div>
    <div class="devwrap">
      <div class="imgcol">
        <div class="toolbar" style="padding:0 0 10px;border:0">
          <select id="imgsel"><option value="">— nessuna immagine —</option><optgroup label="Foto con mappa dei comandi (${S.photoDevices.length})">${S.photoDevices.map(p => `<option value="foto:${esc(p.id)}" ${img === 'foto:' + p.id ? 'selected' : ''}>${esc(p.name)}</option>`).join('')}</optgroup><optgroup label="Schemi disegnati (${S.templates.length})">${S.templates.map(t => `<option value="tpl:${esc(t)}" ${img === 'tpl:' + t ? 'selected' : ''}>${esc(t)}</option>`).join('')}</optgroup>${img.startsWith('img:') ? `<option value="${esc(img)}" selected>Immagine caricata da te</option>` : ''}</select>
          <button class="small" id="imgup">Carica immagine…</button><input type="file" id="imgfile" accept=".png,.jpg,.jpeg,.webp,.gif,.svg" class="hidden">
          <button class="small" id="imgweb">Cerca sul web</button>
          ${img.startsWith('img:') ? `<button class="small ${S.placingMode ? 'primary' : ''}" id="placemode">${S.placingMode ? 'Fine posizionamento' : 'Posiziona etichette'}</button>` : ''}
          ${img ? '<button class="small" id="schexp" title="Salva immagine ed etichette in un file da condividere con altri">Esporta schema</button>' : ''}
          <button class="small" id="schimp" title="Carica uno schema ricevuto da altri">Importa schema…</button><input type="file" id="schfile" accept=".json" class="hidden">
        </div>
        ${!isED() ? `<p class="hint adv" title="Curve, sensibilità, zone morte e saturazione si regolano nel gioco. L'app le rilegge dal file del gioco a ogni scrittura e le tiene legate a questa periferica: se il suo numero js cambia, le riscrive sotto il numero nuovo.">⚙ Impostazioni avanzate del gioco legate a questa periferica: ${S.advanced[key] ? '<b>' + esc(S.advanced[key]) + '</b>' : 'nessuna (valori standard)'}</p>` : ''}
        <div id="imgarea"></div>
        <p class="hint" id="imghint"></p>
      </div>
      <div class="inpcol">${list}</div>
    </div>`;
  renderImage();
}

const PH = /^(BUTTON_(\d+)|AXIS_(X|Y|Z|RX|RY|RZ)|AXIS_SLIDER_(\d)|([XYZ])_AXIS|POV_(\d+)_([UDLR])|TEMPLATE_NAME)$/;
function phInput(t) {
  const m = PH.exec(t); if (!m) return null;
  if (m[2]) return 'button' + m[2];
  if (m[3]) return { X: 'x', Y: 'y', Z: 'z', RX: 'rotx', RY: 'roty', RZ: 'rotz' }[m[3]];
  if (m[4]) return 'slider' + m[4];
  if (m[5]) return m[5].toLowerCase();
  if (m[6]) return 'hat' + m[6] + '_' + { U: 'up', D: 'down', L: 'left', R: 'right' }[m[7]];
  return '@name';
}
async function renderImage() {
  const key = S.view.key, di = devInfo(key), img = di.image || '', area = $('#imgarea'), hint = $('#imghint');
  const labelFor = input => isED() ? edInputBindings(key, input).map(x => edCmd(x.tag).en + (x.half ? (x.half === 'Pos' ? ' (+)' : ' (−)') : '')).join(' / ') : S.profile.bindings.filter(b => b.dev === key && b.input === input).map(actLabel).join(' / ');
  if (!img) {
    area.innerHTML = `<div class="noimg">Nessuna immagine per questa periferica.<br>Scegli uno schema dall'elenco, oppure premi <b>Cerca sul web</b>, salva l'immagine che preferisci e caricala con <b>Carica immagine…</b>: poi potrai posizionare tu le etichette dei tasti.</div>`;
    hint.textContent = ''; return;
  }
  if (img.startsWith('foto:')) return renderPhoto(key, S.photoById[img.slice(5)], area, hint, labelFor);
  if (img.startsWith('tpl:')) {
    const name = img.slice(4);
    if (!S.tplCache[name]) {
      area.innerHTML = '<div class="noimg">Carico lo schema…</div>';
      try { S.tplCache[name] = await (await fetch('/tpl/' + encodeURIComponent(name))).text(); } catch (e) { area.innerHTML = '<div class="noimg">Schema non disponibile.</div>'; return; }
      if (S.view.type !== 'dev' || S.view.key !== key || (devInfo(key).image || '') !== img) return;
    }
    const doc = new DOMParser().parseFromString(S.tplCache[name], 'image/svg+xml');
    const svg = doc.documentElement;
    if (!svg || svg.nodeName !== 'svg') { area.innerHTML = '<div class="noimg">Schema non leggibile.</div>'; return; }
    svg.removeAttribute('content'); svg.removeAttribute('width'); svg.removeAttribute('height');
    for (const s of [...svg.querySelectorAll('script')]) s.remove();
    const box = document.createElement('div'); box.className = 'imgbox';
    box.appendChild(document.importNode(svg, true));
    for (const el of box.querySelectorAll('div,text')) {
      if (el.children.length) continue;
      const input = phInput((el.textContent || '').trim());
      if (!input) continue;
      if (input === '@name') { el.textContent = devName(key); continue; }
      const lab = labelFor(input);
      el.textContent = lab ? lab : inputShort(input);
      el.setAttribute('title', inputLabel(input) + (lab ? ': ' + lab : ' (non assegnato)'));
      if (el.tagName.toLowerCase() === 'div') {
        el.classList.add('ph'); el.dataset.live = input; el.dataset.input = input;
        el.style.fontSize = lab ? (lab.length > 40 ? '5.5px' : '6.5px') : '9px';
        el.style.color = lab ? '#003a75' : '#666'; el.style.fontWeight = lab ? '700' : '600';
        el.style.pointerEvents = 'all';
      }
    }
    area.innerHTML = ''; area.appendChild(box);
    hint.innerHTML = 'Premi un tasto sulla periferica: la sua casella si illumina di giallo. Clicca una casella per assegnare un comando. Se le caselle non corrispondono ai tuoi tasti (numerazione personalizzata nel software della periferica), scegli un altro schema o carica una tua immagine. <small>Schemi dal progetto open source Joystick Diagrams.</small>';
    return;
  }
  const file = img.slice(4);
  area.innerHTML = `<div class="imgbox ${S.placingMode ? 'placing' : ''}" id="custimg"><img src="/img/${encodeURIComponent(file)}" alt="">${(di.hotspots || []).map((h, i) => {
    const lab = labelFor(h.input);
    return `<div class="hs" data-live="${esc(h.input)}" data-hs="${i}" style="left:${h.x}%;top:${h.y}%" title="${esc(inputLabel(h.input))}"><b>${esc(inputShort(h.input))}</b>${lab ? '<br>' + esc(lab) : ''}</div>`;
  }).join('')}</div>`;
  hint.innerHTML = S.placingMode
    ? (S.placing ? `Ora clicca sull'immagine nel punto in cui si trova <b>${esc(inputLabel(S.placing))}</b>.` : 'Premi un tasto sulla periferica (oppure 📍 nella lista a destra), poi clicca sull\'immagine dove si trova.')
    : 'Usa <b>Posiziona etichette</b> per indicare dove si trova ogni tasto. Clic su un\'etichetta per assegnare un comando; clic destro per toglierla.';
}
// ---------- modelli con foto ----------
const VALID_IN = /^(button\d+|hat\d_(up|down|left|right)|x|y|z|rot[xyz]|slider[12])$/;
function photoInputs(key, tpl, ctl) { const m = ((devInfo(key).photoMap || {})[tpl.id] || {})[ctl.id]; return m && m.length === ctl.inputs.length ? m : ctl.inputs; }
function posLabels(ctl) {
  const n = ctl.inputs.length;
  if (ctl.kind === 'hat') return ['↑', '→', '↓', '←', '●'];
  if (ctl.kind === 'encoder') return ['↻', '↺', '●'];
  if (ctl.kind === 'switch' && n > 1) return Array.from({ length: n }, (_, i) => String(i + 1));
  return [];
}
function renderPhoto(key, tpl, area, hint, labelFor) {
  if (!tpl) { area.innerHTML = '<div class="noimg">Modello non disponibile in questa versione.</div>'; hint.textContent = ''; return; }
  let html = '';
  for (const v of tpl.views) {
    const ctls = tpl.controls.filter(c => c.view === v.id).sort((a, b) => a.y - b.y);
    let L = ctls.filter(c => c.x < 0.5), R = ctls.filter(c => c.x >= 0.5);
    const mv = (from, to) => { let bi = 0; from.forEach((c, i) => { if (Math.abs(c.x - 0.5) < Math.abs(from[bi].x - 0.5)) bi = i; }); to.push(from.splice(bi, 1)[0]); to.sort((a, b) => a.y - b.y); };
    while (L.length - R.length > 3) mv(L, R);
    while (R.length - L.length > 3) mv(R, L);
    const box = c => {
      const ins = photoInputs(key, tpl, c), pos = posLabels(c);
      const rows = ins.map((inp, i) => {
        const ok = VALID_IN.test(inp), lab = ok ? labelFor(inp) : '';
        const lrn = S.learn && S.learn.ctl === c.id && S.learn.idx === i;
        return `<div class="pr ${lrn ? 'learn' : ''} ${ok && isIgnored(key, inp) ? 'ign' : ''}" data-ctl="${esc(c.id)}" data-idx="${i}" ${ok ? `data-live="${inp}" data-input="${inp}"` : ''} title="${ok ? esc(inputLabel(inp)) + ' — clic: assegna un comando · clic destro: cambia il tasto' : 'Clicca e poi premi il tasto vero sulla periferica'}">${pos[i] ? `<i>${pos[i]}</i>` : ''}<b class="${ok ? '' : 'q'}">${lrn ? '…' : ok ? esc(inputShort(inp)) : '?'}</b><span>${lrn ? 'premi il tasto' : esc(lab)}</span></div>`;
      }).join('');
      return `<div class="pl" data-ctl="${esc(c.id)}" data-x="${c.x}" data-y="${c.y}"><div class="plh">${esc(c.label || c.id)}</div>${rows}</div>`;
    };
    html += `<div class="pv"><div class="pvc">${esc(tpl.name)} — ${esc(v.label)}</div><div class="pvg"><div class="pcol L">${L.map(box).join('')}</div><div class="pimg"><img src="/photo/${encodeURIComponent(v.photo)}" alt=""></div><div class="pcol R">${R.map(box).join('')}</div><svg class="plines"></svg></div></div>`;
  }
  area.innerHTML = html || '<div class="noimg">Questo modello non ha comandi mappati.</div>';
  for (const im of area.querySelectorAll('img')) im.addEventListener('load', drawPhotoLines);
  drawPhotoLines();
  const unk = tpl.controls.some(c => photoInputs(key, tpl, c).some(i => !VALID_IN.test(i)));
  hint.innerHTML = (S.learn ? '<b>Premi ora sulla periferica il tasto (o muovi l\'asse) che corrisponde alla riga evidenziata.</b> ' : '') +
    'Premi un tasto vero: la sua riga si illumina di giallo. Clic su una riga per assegnare un comando. ' +
    (unk ? 'Le righe con <b>?</b> non hanno un numero noto per questo modello: clicca la riga e premi il tasto vero, l\'app lo impara. ' : '') +
    'Se una riga non corrisponde al tuo tasto (numerazione cambiata nel software della periferica): clic destro sulla riga e premi il tasto giusto. ' +
    '<small>Foto dei rispettivi produttori; mappe dal progetto open source SC Keymap. Numerazione di fabbrica, non verificata su ogni esemplare.</small>';
}
function drawPhotoLines() {
  for (const g of $$('.pvg')) {
    const svg = $('.plines', g), img = $('img', g); if (!svg || !img || !img.naturalWidth) continue;
    const gr = g.getBoundingClientRect(), ir = img.getBoundingClientRect();
    svg.setAttribute('viewBox', `0 0 ${gr.width} ${gr.height}`); svg.style.width = gr.width + 'px'; svg.style.height = gr.height + 'px';
    let out = '';
    for (const el of $$('.pl', g)) {
      const r = el.getBoundingClientRect(), left = !!el.closest('.pcol.L');
      const x1 = (left ? r.right : r.left) - gr.left, y1 = r.top + Math.min(12, r.height / 2) - gr.top;
      const x2 = ir.left - gr.left + (+el.dataset.x) * ir.width, y2 = ir.top - gr.top + (+el.dataset.y) * ir.height;
      out += `<path d="M${x1} ${y1} L${x1 + (left ? 10 : -10)} ${y1} L${x2} ${y2}" /><circle cx="${x2}" cy="${y2}" r="3.5" />`;
    }
    svg.innerHTML = out;
  }
}
window.addEventListener('resize', () => requestAnimationFrame(drawPhotoLines));
function setPhotoInput(key, tplId, ctlId, idx, input) {
  const tpl = S.photoById[tplId], ctl = tpl && tpl.controls.find(c => c.id === ctlId); if (!ctl) return;
  const di = devInfo(key); di.photoMap = di.photoMap || {}; di.photoMap[tplId] = di.photoMap[tplId] || {};
  const cur = [...photoInputs(key, tpl, ctl)]; cur[idx] = input; di.photoMap[tplId][ctlId] = cur;
  saveProfile();
}
function setPlacing(input) {
  S.placing = input;
  for (const r of $$('.row[data-input]')) r.classList.toggle('placing', r.dataset.input === input);
  const hint = $('#imghint');
  if (hint && S.placingMode) hint.innerHTML = `Ora clicca sull'immagine nel punto in cui si trova <b>${esc(inputLabel(input))}</b>.`;
}

// ---------- anteprima 3D della nave ----------
const MOVES = [
  { id: 'pitch', label: 'Beccheggio (pitch)', hint: ['muso giù', 'muso su'], axis: 'v_pitch', pos: 'v_pitch_up', neg: 'v_pitch_down', sign: 1 },
  { id: 'yaw', label: 'Imbardata (yaw)', hint: ['muso a sinistra', 'muso a destra'], axis: 'v_yaw', pos: 'v_yaw_right', neg: 'v_yaw_left', sign: 1 },
  { id: 'roll', label: 'Rollio (roll)', hint: ['inclina a sinistra', 'inclina a destra'], axis: 'v_roll', pos: 'v_roll_right', neg: 'v_roll_left', sign: 1 },
  { id: 'lat', label: 'Traslazione laterale', hint: ['sinistra', 'destra'], axis: 'v_strafe_lateral', pos: 'v_strafe_right', neg: 'v_strafe_left', sign: 1 },
  { id: 'vert', label: 'Traslazione verticale', hint: ['giù', 'su'], axis: 'v_strafe_vertical', pos: 'v_strafe_up', neg: 'v_strafe_down', sign: -1 },
  { id: 'lon', label: 'Avanti / indietro', hint: ['indietro', 'avanti'], axis: 'v_strafe_longitudinal', pos: 'v_strafe_forward', neg: 'v_strafe_back', sign: -1 },
];
const FM = 'spaceship_movement';
function jsBinding(name) { return bindingsOf(FM, name).find(b => isJsDev(b.dev)); }
function bindingValue(b) { // -1..1 per un asse, 0/1 per un pulsante
  if (!b) return 0;
  const s = S.input[b.dev]; if (!s) return 0;
  if (AXN[b.input]) { const v = ((s.a || {})[b.input] ?? 32767.5) / 32767.5 - 1; return Math.abs(v) < 0.04 ? 0 : v; }
  return activeInputs(s).has(b.input) ? 1 : 0;
}
// comando "a senso unico" (es. Manetta - Aumenta): un pulsante vale 0/1, un asse a corsa intera vale 0..1
function halfValue(name) {
  const b = jsBinding(name); if (!b) return 0;
  if (!AXN[b.input]) return bindingValue(b);
  const s = S.input[b.dev]; if (!s) return 0;
  let v = ((s.a || {})[b.input] ?? 0) / 65535;
  const act = S.actions.get(actKey(FM, name));
  if (act && act.canInvert && invertState(b.dev, act.optGroup)) v = 1 - v;
  return v < 0.03 ? 0 : v;
}
function moveValue(m) {
  let v = 0;
  const ab = jsBinding(m.axis), act = S.actions.get(actKey(FM, m.axis));
  if (ab) { let raw = bindingValue(ab); if (AXN[ab.input]) { if (act && act.canInvert && invertState(ab.dev, act.optGroup)) raw = -raw; v += raw * m.sign; } else v += raw; }
  v += halfValue(m.pos) - halfValue(m.neg);
  return Math.max(-1, Math.min(1, v));
}
function renderShip() {
  const bindTxt = name => { const b = jsBinding(name); return b ? `${jsNum(b.dev) ? 'js' + jsNum(b.dev) : 'scollegata'} · ${esc(inputLabel(b.input))} <small>${esc(devName(b.dev).slice(0, 24))}</small>` : '<span class="def">non assegnato — clicca per assegnare</span>'; };
  const invHalf = name => { const b = jsBinding(name), act = S.actions.get(actKey(FM, name)); return b && AXN[b.input] && act && act.canInvert ? `<label class="inv"><input type="checkbox" data-inv="${esc(act.optGroup)}" data-dev="${esc(b.dev)}" ${invertState(b.dev, act.optGroup) ? 'checked' : ''}> inverti</label>` : ''; };
  const rows = MOVES.map(m => {
    const ab = jsBinding(m.axis), act = S.actions.get(actKey(FM, m.axis));
    const inv = ab && AXN[ab.input] && act && act.canInvert ? `<label class="inv"><input type="checkbox" data-inv="${esc(act.optGroup)}" data-dev="${esc(ab.dev)}" ${invertState(ab.dev, act.optGroup) ? 'checked' : ''}> inverti</label>` : '';
    return `<div class="mv"><div class="mvh"><b>${m.label}</b><span class="mvd" id="mvd_${m.id}"></span></div>
      <div class="mbar"><i id="mv_${m.id}"></i></div>
      <div class="mvb" data-bind="${FM}/${m.axis}">Asse: ${bindTxt(m.axis)} ${inv}</div>
      <div class="mvb2"><span data-bind="${FM}/${m.neg}">◀ ${m.hint[0]}: ${bindTxt(m.neg)} ${invHalf(m.neg)}</span><span data-bind="${FM}/${m.pos}">${m.hint[1]} ▶: ${bindTxt(m.pos)} ${invHalf(m.pos)}</span></div></div>`;
  }).join('');
  $('#view').innerHTML = `<div class="toolbar"><h2>Anteprima 3D — movimenti della nave</h2><small>Muovi le periferiche: la nave risponde ai comandi di volo che hai assegnato ai joystick.</small></div>
    <div class="shipwrap"><div class="shipcv"><canvas id="ship"></canvas></div><div class="shiplist">${rows}
    <p class="hint">Se la nave si muove al contrario di come vuoi, spunta <b>inverti</b> sull'asse. L'anteprima usa la convenzione abituale (leva indietro = muso su, leva a destra = destra, leva avanti = avanti): l'ultima parola spetta comunque al gioco, fai una prova in volo. Tastiera e gamepad non sono mostrati qui.</p></div></div>`;
  if (!S.shipLoop) { S.shipLoop = true; S.shipV = {}; requestAnimationFrame(shipFrame); }
}
const SHIP = (() => {
  const N = [0, 0, 2.4], T = [0, .38, .3], B = [0, -.26, .3], L = [-.48, 0, .3], R = [.48, 0, .3], tT = [0, .3, -1.6], tB = [0, -.2, -1.6], tL = [-.36, 0, -1.6], tR = [.36, 0, -1.6];
  const hull = '#8fa3b8', wing = '#5f7890', fin = '#c0473f', glass = '#3aa0ff';
  return [
    { p: [N, T, R], c: hull }, { p: [N, R, B], c: hull }, { p: [N, B, L], c: hull }, { p: [N, L, T], c: hull },
    { p: [T, tT, tR, R], c: hull }, { p: [R, tR, tB, B], c: hull }, { p: [B, tB, tL, L], c: hull }, { p: [L, tL, tT, T], c: hull },
    { p: [tT, tL, tB, tR], c: '#2b3644' },
    { p: [[.4, 0, .5], [2.1, -.06, -1.0], [2.1, -.06, -1.5], [.34, 0, -1.4]], c: wing, two: 1 },
    { p: [[-.4, 0, .5], [-2.1, -.06, -1.0], [-2.1, -.06, -1.5], [-.34, 0, -1.4]], c: wing, two: 1 },
    { p: [[0, .3, -.5], [0, 1.05, -1.6], [0, .3, -1.6]], c: fin, two: 1 },
    { p: [[0, .16, 1.6], [.2, .34, .7], [0, .47, .45], [-.2, .34, .7]], c: glass, two: 1 },
    { p: [[2.1, -.06, -1.0], [2.1, .3, -1.35], [2.1, -.06, -1.5]], c: fin, two: 1 }, { p: [[-2.1, -.06, -1.0], [-2.1, .3, -1.35], [-2.1, -.06, -1.5]], c: fin, two: 1 },
  ];
})();
function shade(hex, k) { const n = parseInt(hex.slice(1), 16), f = c => Math.max(0, Math.min(255, Math.round(c * k))); return `rgb(${f(n >> 16)},${f((n >> 8) & 255)},${f(n & 255)})`; }
function shipFrame() {
  const cv = $('#ship');
  if (S.view.type !== 'ship' || !cv) { S.shipLoop = false; return; }
  const box = cv.parentElement.getBoundingClientRect(), dpr = window.devicePixelRatio || 1;
  const W = Math.max(200, box.width), H = Math.max(200, box.height);
  if (cv.width !== Math.round(W * dpr) || cv.height !== Math.round(H * dpr)) { cv.width = Math.round(W * dpr); cv.height = Math.round(H * dpr); cv.style.width = W + 'px'; cv.style.height = H + 'px'; }
  const g = cv.getContext('2d'); g.setTransform(dpr, 0, 0, dpr, 0, 0);
  const v = S.shipV;
  for (const m of MOVES) {
    const t = moveValue(m); v[m.id] = (v[m.id] || 0) + (t - (v[m.id] || 0)) * 0.18;
    const bar = $('#mv_' + m.id), lab = $('#mvd_' + m.id);
    if (bar) { const w = Math.abs(t) * 50; bar.style.width = w + '%'; bar.style.left = (t < 0 ? 50 - w : 50) + '%'; }
    if (lab) lab.textContent = Math.abs(t) < 0.05 ? '' : m.hint[t > 0 ? 1 : 0] + ' ' + Math.round(Math.abs(t) * 100) + '%';
  }
  g.fillStyle = '#070a10'; g.fillRect(0, 0, W, H);
  const f = Math.min(W, H) * 1.05, cx = W / 2, cy = H * 0.46, camY = 1.3, camZ = -8.5;
  const proj = p => { const z = p[2] - camZ; return [cx + f * p[0] / z, cy - f * (p[1] - camY) / z, z]; };
  // stelle e griglia di riferimento
  g.fillStyle = '#51607a';
  for (let i = 0; i < 70; i++) { const x = (Math.sin(i * 12.9898) * 43758.5453 % 1 + 1) % 1, y = (Math.sin(i * 78.233) * 12543.123 % 1 + 1) % 1; g.fillRect(x * W, y * H * 0.7, 1.5, 1.5); }
  g.strokeStyle = '#1d2b3d'; g.lineWidth = 1; g.beginPath();
  for (let i = -8; i <= 8; i++) { let a = proj([i * 1.5, -2.2, -5]), b = proj([i * 1.5, -2.2, 30]); g.moveTo(a[0], a[1]); g.lineTo(b[0], b[1]); }
  for (let k = 0; k <= 12; k++) { const zz = -5 + k * 3; let a = proj([-12, -2.2, zz]), b = proj([12, -2.2, zz]); g.moveTo(a[0], a[1]); g.lineTo(b[0], b[1]); }
  g.stroke();
  // posizione di riposo (riferimento)
  const o = proj([0, 0, 0]); g.strokeStyle = '#2b3d55'; g.setLineDash([4, 6]); g.beginPath(); g.moveTo(o[0] - 60, o[1]); g.lineTo(o[0] + 60, o[1]); g.moveTo(o[0], o[1] - 40); g.lineTo(o[0], o[1] + 40); g.stroke(); g.setLineDash([]);
  const pi = (v.pitch || 0) * 0.6, ya = (v.yaw || 0) * 0.6, ro = (v.roll || 0) * 0.95;
  const tx = (v.lat || 0) * 1.9, ty = (v.vert || 0) * 1.3, tz = (v.lon || 0) * 3.0;
  const cr = Math.cos(ro), sr = Math.sin(ro), cp = Math.cos(pi), sp = Math.sin(pi), cyw = Math.cos(ya), sy = Math.sin(ya);
  const tf = p => {
    let x = p[0] * cr + p[1] * sr, y = -p[0] * sr + p[1] * cr, z = p[2];
    let y2 = y * cp + z * sp, z2 = -y * sp + z * cp;
    let x3 = x * cyw + z2 * sy, z3 = -x * sy + z2 * cyw;
    return [x3 + tx, y2 + ty, z3 + tz];
  };
  const light = [0.35, 0.8, -0.5], ll = Math.hypot(...light);
  const faces = [];
  for (const fc of SHIP) {
    const w = fc.p.map(tf), a = w[0], b = w[1], c = w[2];
    const u = [b[0] - a[0], b[1] - a[1], b[2] - a[2]], q = [c[0] - a[0], c[1] - a[1], c[2] - a[2]];
    let n = [u[1] * q[2] - u[2] * q[1], u[2] * q[0] - u[0] * q[2], u[0] * q[1] - u[1] * q[0]];
    const nl = Math.hypot(...n) || 1; n = n.map(x => x / nl);
    const ctr = w.reduce((s, p) => [s[0] + p[0], s[1] + p[1], s[2] + p[2]], [0, 0, 0]).map(x => x / w.length);
    const view = [ctr[0], ctr[1] - camY, ctr[2] - camZ];
    let facing = n[0] * view[0] + n[1] * view[1] + n[2] * view[2];
    if (facing > 0) { if (!fc.two && false) continue; n = n.map(x => -x); }
    const k = 0.45 + 0.6 * Math.max(0, (n[0] * light[0] + n[1] * light[1] + n[2] * light[2]) / ll);
    faces.push({ pts: w.map(proj), d: Math.hypot(...view), col: shade(fc.c, k) });
  }
  faces.sort((a, b) => b.d - a.d);
  // scia dei motori (più lunga andando avanti)
  const thr = Math.max(0, v.lon || 0);
  for (const ex of [-.2, .2]) {
    const a = proj(tf([ex, .05, -1.6])), b = proj(tf([ex, .05, -1.9 - thr * 1.8]));
    const gr = g.createLinearGradient(a[0], a[1], b[0], b[1]); gr.addColorStop(0, 'rgba(120,200,255,.95)'); gr.addColorStop(1, 'rgba(120,200,255,0)');
    g.strokeStyle = gr; g.lineWidth = Math.max(3, f * 0.16 / a[2]); g.lineCap = 'round'; g.beginPath(); g.moveTo(a[0], a[1]); g.lineTo(b[0], b[1]); g.stroke();
  }
  for (const fc of faces) { g.beginPath(); fc.pts.forEach((p, i) => i ? g.lineTo(p[0], p[1]) : g.moveTo(p[0], p[1])); g.closePath(); g.fillStyle = fc.col; g.fill(); g.strokeStyle = '#0b1018'; g.lineWidth = 0.6; g.stroke(); }
  requestAnimationFrame(shipFrame);
}


// ====================== ELITE DANGEROUS ======================
const isED = () => S.game === 'ed';
function edState() { const p = S.profile; p.ed = p.ed || { bindings: {}, imported: false }; p.ed.bindings = p.ed.bindings || {}; return p.ed; }
const edB = () => edState().bindings;
const edCmd = tag => (S.edCmd || {})[tag] || { tag, en: tag, it: '', group: 'Altro', kind: 'button' };
function edDevId(key) {
  const c = ((S.ed || {}).devIds || {})[key]; if (c) return c;
  const m = /^\{([0-9A-F]{4})([0-9A-F]{4})-.*#(\d+)$/i.exec(key); if (!m) return ['', ''];
  return [(m[2] + m[1]).toUpperCase(), +m[3] > 1 ? String(+m[3] - 1) : ''];
}
function edKeyOf(slot) { if (!slot) return null; for (const k of S.profile.order) { const id = edDevId(k); if (id[0] && id[0] === slot.d && (id[1] || '') === (slot.i || '')) return k; } return null; }
const ED_AX = { x: 'XAxis', y: 'YAxis', z: 'ZAxis', rotx: 'RXAxis', roty: 'RYAxis', rotz: 'RZAxis', slider1: 'UAxis', slider2: 'VAxis' };
const ED_AX_R = Object.fromEntries(Object.entries(ED_AX).map(([a, b]) => [b, a]));
function edKeyFromInput(input, half) {
  let m;
  if ((m = /^button(\d+)$/.exec(input))) return 'Joy_' + m[1];
  if ((m = /^hat(\d)_(up|down|left|right)$/.exec(input))) return 'Joy_POV' + m[1] + m[2][0].toUpperCase() + m[2].slice(1);
  if (ED_AX[input]) return (half ? half + '_' : '') + 'Joy_' + ED_AX[input];
  return '';
}
function edParseKey(k) {
  const m = /^(Pos_|Neg_)?Joy_(.+)$/.exec(k || ''); if (!m) return null;
  const half = m[1] ? m[1].slice(0, 3) : '', r = m[2]; let x;
  if (/^\d+$/.test(r)) return { input: 'button' + r, half };
  if (ED_AX_R[r]) return { input: ED_AX_R[r], half };
  if ((x = /^POV(\d)(Up|Down|Left|Right)$/.exec(r))) return { input: 'hat' + x[1] + '_' + x[2].toLowerCase(), half };
  return null;
}
const edEmpty = s => !s || !s.k || !s.d || s.d === '{NoDevice}';
function edSlotLabel(s) {
  if (edEmpty(s)) return '';
  const mods = (s.m || []).map(m => edSlotLabel(m)).filter(Boolean);
  let t;
  if (s.d === 'Keyboard') t = 'Tastiera: ' + s.k.replace(/^Key_/, '');
  else if (s.d === 'Mouse') t = 'Mouse: ' + s.k.replace(/^Mouse_/, '');
  else {
    const key = edKeyOf(s), p = edParseKey(s.k);
    const dev = key ? (jsNum(key) ? '' : '(scollegata) ') + devName(key).slice(0, 22) : 'periferica ' + s.d + (s.i ? ' #' + (+s.i + 1) : '') + ' (sconosciuta)';
    t = (p ? inputLabel(p.input) + (p.half === 'Pos' ? ' (+)' : p.half === 'Neg' ? ' (−)' : '') : s.k) + ' · ' + dev;
  }
  return (mods.length ? mods.join(' + ') + ' + ' : '') + t + (s.h === '1' ? ' (tieni)' : '');
}
function edInputBindings(key, input) {
  const id = edDevId(key), out = [], B = edB();
  for (const tag in B) for (const sl of ['p', 's', 'a']) {
    const s = B[tag][sl]; if (edEmpty(s) || s.d !== id[0] || (s.i || '') !== (id[1] || '')) continue;
    const p = edParseKey(s.k); if (p && p.input === input) out.push({ tag, sl, slot: s, half: p.half });
  }
  return out;
}
function edCountFor(key) { const id = edDevId(key), B = edB(); let n = 0; for (const tag in B) for (const sl of ['p', 's', 'a']) { const s = B[tag][sl]; if (!edEmpty(s) && s.d === id[0] && (s.i || '') === (id[1] || '')) n++; } return n; }
function edAssign(tag, key, input, sl, half) {
  const c = edCmd(tag), B = edB(), b = (B[tag] = B[tag] || {}), id = edDevId(key), isAxis = !!ED_AX[input];
  if (!id[0]) return toast('Periferica non riconosciuta per Elite', true);
  if (c.kind === 'axis') {
    if (!isAxis) return toast('"' + c.en + '" è un comando analogico: serve un asse, non un pulsante', true);
    b.a = { d: id[0], i: id[1] || undefined, k: edKeyFromInput(input) };
  } else {
    if (!sl || sl === 'a') sl = edEmpty(b.p) ? 'p' : 's';
    const old = b[sl];
    b[sl] = { d: id[0], i: id[1] || undefined, k: edKeyFromInput(input, isAxis ? (half || 'Pos') : '') };
    if (!edEmpty(old)) toast('Sostituita l\'assegnazione precedente: ' + edSlotLabel(old));
  }
  saveProfile().then(pollState); render();
}
function edClear(tag, sl) { const b = edB()[tag]; if (!b) return; b[sl] = { d: '{NoDevice}', k: '' }; saveProfile().then(pollState); render(); }
function edChips(key, input) {
  return edInputBindings(key, input).map(x => {
    const c = edCmd(x.tag), b = edB()[x.tag];
    const inv = x.sl === 'a' ? `<label class="inv"><input type="checkbox" data-edinv="${esc(x.tag)}" ${b.inv ? 'checked' : ''}> inverti</label>` : '';
    return `<span class="chip" title="${esc(c.it)}">${esc(c.en)}${x.half ? (x.half === 'Pos' ? ' (+)' : ' (−)') : ''} <small>${esc(c.it)} · ${x.sl === 'a' ? 'asse' : x.sl === 'p' ? 'primario' : 'secondario'}</small> <span class="x" data-edrm="${esc(x.tag)}|${x.sl}" title="Rimuovi">×</span></span>${inv}`;
  }).join('');
}
function renderAllED() {
  S.edCmd = S.edCmd || {};
  const E = S.ed || {}, f = S.edFilter = S.edFilter || { q: '', group: '', mode: 'all' }, q = f.q.trim().toLowerCase(), B = edB();
  if (!E.found) { $('#view').innerHTML = `<div class="toolbar"><h2>Elite Dangerous</h2></div><div class="scroll"><p style="padding:20px">${esc((E.file || {}).error || 'Cerco Elite Dangerous…')}</p></div>`; return; }
  const cmds = (E.commands || []).filter(c => !E.tags || E.tags.includes(c.tag));
  const extra = (E.tags || []).filter(t => !S.edCmd[t]).map(t => ({ tag: t, en: t, it: '', group: 'Altri comandi', kind: ((E.disk || {})[t] || {}).a ? 'axis' : 'button' }));
  const groups = [...new Set([...cmds, ...extra].map(c => c.group))];
  let rows = '', shown = 0, cur = '';
  for (const c of [...cmds, ...extra]) {
    if (f.group && c.group !== f.group) continue;
    const b = B[c.tag] || {}, has = !edEmpty(b.p) || !edEmpty(b.s) || !edEmpty(b.a);
    const hasJoy = ['p', 's', 'a'].some(sl => !edEmpty(b[sl]) && b[sl].d !== 'Keyboard' && b[sl].d !== 'Mouse');
    if (f.mode === 'bound' && !hasJoy) continue;
    if (f.mode === 'unbound' && has) continue;
    if (q && !(c.en + ' ' + c.it + ' ' + c.tag).toLowerCase().includes(q)) continue;
    if (c.group !== cur) { cur = c.group; rows += `<tr class="grp"><td colspan="3">${esc(cur)}</td></tr>`; }
    const cell = (sl, s) => `<td class="c" data-edcap="${sl}">${edEmpty(s) ? '<span class="def">—</span>' : `<span class="chip ${s.d === 'Keyboard' || s.d === 'Mouse' ? 'kb' : (edKeyOf(s) && jsNum(edKeyOf(s)) ? '' : 'off')}">${esc(edSlotLabel(s))}</span>`}</td>`;
    rows += `<tr data-ed="${esc(c.tag)}"><td><div class="lab">${esc(c.en)}${c.kind === 'axis' ? '<span class="ax">asse</span>' : ''}</div><div class="en">${esc(c.it)}</div></td>` +
      (c.kind === 'axis' ? `<td class="c" data-edcap="a" colspan="2">${edEmpty(b.a) ? '<span class="def">—</span>' : `<span class="chip ${edKeyOf(b.a) && jsNum(edKeyOf(b.a)) ? '' : 'off'}">${esc(edSlotLabel(b.a))}</span>`} <label class="inv"><input type="checkbox" data-edinv="${esc(c.tag)}" ${b.inv ? 'checked' : ''}> inverti</label></td>` : cell('p', b.p) + cell('s', b.s)) + '</tr>';
    shown++;
  }
  $('#view').innerHTML = `<div class="toolbar">
      <input type="search" id="edq" placeholder="Cerca un comando (inglese o italiano)…" value="${esc(f.q)}">
      <select id="edgroup"><option value="">Tutte le categorie</option>${groups.map(g => `<option ${f.group === g ? 'selected' : ''}>${esc(g)}</option>`).join('')}</select>
      <select id="edmode"><option value="all">Tutti</option><option value="bound" ${f.mode === 'bound' ? 'selected' : ''}>Solo assegnati a joystick</option><option value="unbound" ${f.mode === 'unbound' ? 'selected' : ''}>Solo non assegnati</option></select>
      <small>${shown} comandi · file: ${esc((E.file || {}).preset || '')}</small></div>
    <div class="scroll"><table class="cmd"><thead><tr><th>Comando</th><th>Primario / asse</th><th>Secondario</th></tr></thead><tbody>${rows || '<tr><td colspan="3">Nessun comando trovato.</td></tr>'}</tbody></table>
    <p class="hint" style="padding:10px 14px">Clicca una cella e premi il tasto o muovi l'asse sulla periferica. I comandi di tastiera e mouse vengono mostrati e conservati, ma per ora si modificano dal gioco.</p></div>`;
}
function pickCommandED(key, input) {
  const isAxis = !!ED_AX[input];
  openModal(`<h2>Elite: assegna un comando a ${esc(inputLabel(input))} — ${esc(devName(key))}</h2>
    <input type="search" id="pq" placeholder="Cerca un comando…">
    <div class="pick" id="plist"></div>
    <div class="en">${isAxis ? 'Su un asse puoi mettere un comando analogico (etichetta "asse") oppure un comando a pulsante: in quel caso scatta spostando l\'asse in un verso.' : 'I comandi analogici (assi) non compaiono: servono su un asse.'}</div>
    <div class="btns"><button data-close="1">Annulla</button></div>`);
  const draw = () => {
    const q = $('#pq').value.trim().toLowerCase(); let out = '', cur = '';
    const list = (S.ed.commands || []).filter(c => (isAxis || c.kind !== 'axis') && (!S.ed.tags || S.ed.tags.includes(c.tag)));
    if (isAxis) list.sort((a, b) => (a.kind === 'axis' ? 0 : 1) - (b.kind === 'axis' ? 0 : 1));
    for (const c of list) {
      if (q && !(c.en + ' ' + c.it + ' ' + c.tag).toLowerCase().includes(q)) continue;
      const g = (isAxis ? (c.kind === 'axis' ? 'Assi — ' : 'Pulsanti — ') : '') + c.group;
      if (g !== cur) { cur = g; out += `<div class="g">${esc(g)}</div>`; }
      const b = edB()[c.tag] || {}, now = [b.a, b.p, b.s].filter(s => !edEmpty(s)).map(edSlotLabel).join(' | ');
      out += `<div class="a" data-ed="${esc(c.tag)}"><span>${esc(c.en)}${c.kind === 'axis' ? '<span class="ax">asse</span>' : ''} <span class="en">${esc(c.it)}</span></span><span class="def">${esc(now.slice(0, 60))}</span></div>`;
    }
    $('#plist').innerHTML = out || '<div class="a">Nessun comando trovato.</div>';
  };
  draw(); $('#pq').oninput = draw; $('#pq').focus();
  $('#plist').onclick = e => { const el = e.target.closest('.a[data-ed]'); if (!el) return; $('#modal').classList.add('hidden'); edAssign(el.dataset.ed, key, input); };
}
function startCaptureED(tag, sl) {
  const c = edCmd(tag), b = edB()[tag] || {}, cur = b[sl];
  S.capture = { kind: 'edjs', tag, sl, base: JSON.parse(JSON.stringify(S.input)), got: null };
  openModal(`<h2>${esc(c.en)}</h2><div class="en">${esc(c.it)} — ${sl === 'a' ? 'asse' : sl === 'p' ? 'assegnazione primaria' : 'assegnazione secondaria'}</div>
    <div class="capzone" id="capzone">${sl === 'a' ? 'Muovi l\'asse che vuoi usare…' : 'Premi un pulsante, sposta un hat o muovi un asse…'}</div>
    <div class="btns">${!edEmpty(cur) ? '<button class="danger" id="crem">Rimuovi assegnazione attuale</button>' : ''}<span class="spacer"></span>
      <button id="cretry" class="hidden">Riprova</button><button id="cok" class="primary hidden">Conferma (Invio)</button><button data-close="1">Annulla</button></div>`);
  $('#crem') && ($('#crem').onclick = () => { S.capture = null; $('#modal').classList.add('hidden'); edClear(tag, sl); });
  $('#cretry').onclick = () => { S.capture.got = null; S.capture.base = JSON.parse(JSON.stringify(S.input)); const z = $('#capzone'); z.classList.remove('got'); z.innerHTML = sl === 'a' ? 'Muovi l\'asse che vuoi usare…' : 'Premi un pulsante, sposta un hat o muovi un asse…'; $('#cok').classList.add('hidden'); $('#cretry').classList.add('hidden'); };
  $('#cok').onclick = confirmCapture;
}
function captureTickED() {
  const c = S.capture; if (!c || c.got) return;
  const found = g => { c.got = g; const z = $('#capzone'); z.classList.add('got');
    z.innerHTML = `Rilevato:<big>${esc(inputLabel(g.input))}${ED_AX[g.input] && c.sl !== 'a' ? (g.dir > 0 ? ' (+)' : ' (−)') : ''}</big>${esc(devName(g.key))}`;
    $('#cok').classList.remove('hidden'); $('#cretry').classList.remove('hidden'); };
  for (const key in S.input) {
    if (!S.profile.devices[key]) continue;
    const cur = S.input[key], base = c.base[key] || { b: [], a: {}, h: [] };
    if (c.sl !== 'a') {
      for (const b of cur.b || []) if (!(base.b || []).includes(b) && !isIgnored(key, 'button' + b)) return found({ key, input: 'button' + b });
      for (let i = 0; i < (cur.h || []).length; i++) { const d = hatDir(cur.h[i]); if (d && cur.h[i] !== (base.h || [])[i]) return found({ key, input: 'hat' + (i + 1) + '_' + d }); }
    }
    for (const a in (cur.a || {})) { const dv = cur.a[a] - ((base.a || {})[a] ?? cur.a[a]); if (!isIgnored(key, a) && Math.abs(dv) > 22000) return found({ key, input: a, dir: dv > 0 ? 1 : -1 }); }
  }
}
async function edImport(silent) {
  if (!S.ed || !S.ed.found) return toast((S.ed && S.ed.file && S.ed.file.error) || 'Elite Dangerous non trovato', true);
  if (!silent && edState().imported && !confirm('Importo i comandi dal file di Elite Dangerous.\nLe assegnazioni di Elite presenti nell\'app vengono sostituite con quelle del file.\n\nContinuo?')) return;
  S.profile.ed = { bindings: JSON.parse(JSON.stringify(S.ed.disk || {})), imported: true };
  await saveProfile(); await pollState(); render();
  toast('Importati i comandi di Elite Dangerous (' + Object.keys(S.profile.ed.bindings).length + ')');
}
async function edWrite() {
  if (!S.ed || !S.ed.found) return toast('Elite Dangerous non trovato', true);
  if (!confirm('Scrivo i comandi nel file di Elite Dangerous:\n' + S.ed.file.path + '\n\nElite deve essere chiuso. Del file attuale viene fatta una copia di sicurezza.\n\nProcedo?')) return;
  try {
    await S.saving; const r = await api('/api/ed/write', {}); await pollState(); render();
    openModal(`<h2>Fatto: comandi scritti in Elite Dangerous</h2><div>File: <b>${esc(r.path)}</b></div><div>Copia di sicurezza del file precedente: ${esc(r.backup)}</div><div>Avvia Elite: in Opzioni → Comandi deve essere selezionato il preset <b>${esc(S.ed.file.preset)}</b>.</div><div class="btns"><button data-close="1" class="primary">Chiudi</button></div>`);
  } catch (e) { toast(e.message, true); }
}
function setGame(g) { S.game = g; S.profile.game = g; S.view = { type: 'all' }; saveProfile().then(pollState).then(async () => { if (isED() && S.ed && S.ed.found && !edState().imported) await edImport(true); render(); }); render(); }

// ---------- finestre ----------
function openModal(html) { $('#modalBox').innerHTML = html; $('#modal').classList.remove('hidden'); }
function closeModal() { $('#modal').classList.add('hidden'); if (S.capture) { S.capture = null; } render(); }

function pickCommand(devKey, input) {
  if (isED()) return pickCommandED(devKey, input);
  openModal(`<h2>Assegna un comando a ${esc(inputLabel(input))} — ${esc(devName(devKey))}</h2>
    <input type="search" id="pq" placeholder="Cerca un comando…" autofocus>
    <div class="pick" id="plist"></div>
    <label class="chk"><input type="checkbox" id="pdbl"> <span>Doppio tocco (il comando parte premendo due volte rapidamente)</span></label>
    <div class="btns"><button data-close="1">Annulla</button></div>`);
  const draw = () => {
    const q = $('#pq').value.trim().toLowerCase(); let out = '';
    for (const m of S.maps) {
      let part = '';
      for (const a of (m.actions || [])) {
        if (!a.visible && !S.filter.hidden) continue;
        if (q && !(a.label + ' ' + a.labelEn + ' ' + a.name).toLowerCase().includes(q)) continue;
        const cur = bindingsOf(a.map, a.name).filter(b => isJsDev(b.dev))[0];
        part += `<div class="a" data-a="${esc(actKey(a.map, a.name))}"><span>${esc(a.label)}${a.axis ? '<span class="ax">asse</span>' : ''} <span class="en">${esc(a.labelEn)}</span></span><span class="def">${cur ? 'ora: ' + esc(inputLabel(cur.input)) + ' su ' + esc(devName(cur.dev).slice(0, 18)) : ''}</span></div>`;
      }
      if (part) out += `<div class="g">${esc(m.label)}</div>` + part;
    }
    $('#plist').innerHTML = out || '<div class="a">Nessun comando trovato.</div>';
  };
  draw();
  $('#pq').oninput = draw; $('#pq').focus();
  $('#plist').onclick = e => {
    const el = e.target.closest('.a[data-a]'); if (!el) return;
    const act = S.actions.get(el.dataset.a), dbl = $('#pdbl').checked;
    $('#modal').classList.add('hidden');
    assignJS(act, devKey, input, dbl ? { multiTap: '2' } : null);
  };
}

function startCapture(act, kind) {
  const bs = bindingsOf(act.map, act.name);
  const cur = kind === 'js' ? bs.filter(b => isJsDev(b.dev) || b.dev === 'jsclear') : kind === 'gp' ? bs.filter(b => b.dev === 'gp') : bs.filter(b => b.dev === 'kb' || b.dev === 'mo');
  S.capture = { act, kind, base: JSON.parse(JSON.stringify(S.input)), got: null, mods: [] };
  openModal(`<h2>${esc(act.label)}</h2><div class="en">${esc(act.labelEn)}${act.desc ? ' — ' + esc(act.desc) : ''}</div>
    <div class="capzone" id="capzone" tabindex="0">${kind === 'js' ? 'Premi un pulsante, sposta un hat o muovi un asse<br>su una qualsiasi periferica…' : kind === 'gp' ? 'Premi un tasto del gamepad o muovi una levetta…' : 'Premi il tasto (anche con Alt/Ctrl/Maiusc) oppure<br>clicca / usa la rotella del mouse dentro questo riquadro…'}</div>
    <label class="chk"><input type="checkbox" id="cdbl"> <span>Doppio tocco</span></label>
    <div class="btns">
      ${cur.length ? '<button class="danger" id="crem">Rimuovi assegnazione attuale</button>' : ''}
      ${kind === 'js' && act.defJs && !cur.length ? '<button id="cclr" title="Scrive nel gioco che questo comando non ha nessun tasto joystick">Disattiva il predefinito</button>' : ''}
      <span class="spacer"></span>
      <button id="cretry" class="hidden">Riprova</button><button id="cok" class="primary hidden">Conferma (Invio)</button><button data-close="1">Annulla</button></div>`);
  $('#capzone').focus();
  $('#crem') && ($('#crem').onclick = () => { S.capture = null; $('#modal').classList.add('hidden'); cur.forEach(b => { S.profile.bindings = S.profile.bindings.filter(x => x !== b); }); saveProfile(); render(); });
  $('#cclr') && ($('#cclr').onclick = () => { S.capture = null; $('#modal').classList.add('hidden'); S.profile.bindings.push({ map: act.map, action: act.name, dev: 'jsclear', input: ' ' }); saveProfile(); render(); });
  $('#cretry').onclick = () => { const c = S.capture; c.got = null; c.order = null; c.gbase = null; c.mods = []; c.base = JSON.parse(JSON.stringify(S.input)); showGot(); $('#capzone').focus(); };
  $('#cok').onclick = confirmCapture;
  if (kind === 'kbm') {
    const z = $('#capzone');
    z.onmousedown = e => { e.preventDefault(); if (S.capture.got) return; gotKBM('mo', ['mouse1', 'mouse3', 'mouse2', 'mouse4', 'mouse5'][e.button] || 'mouse1'); };
    z.oncontextmenu = e => e.preventDefault();
    z.onwheel = e => { e.preventDefault(); if (!S.capture.got) gotKBM('mo', e.deltaY < 0 ? 'mwheel_up' : 'mwheel_down'); };
  }
}
function showGot() {
  const c = S.capture, z = $('#capzone'); if (!c || !z) return;
  if (!c.got) { z.classList.remove('got'); z.innerHTML = c.kind === 'js' ? 'Premi un pulsante, sposta un hat o muovi un asse<br>su una qualsiasi periferica…' : c.kind === 'gp' ? 'Premi un tasto del gamepad o muovi una levetta…' : 'Premi il tasto oppure clicca / usa la rotella dentro questo riquadro…'; $('#cok').classList.add('hidden'); $('#cretry').classList.add('hidden'); return; }
  z.classList.add('got');
  if (c.kind === 'js') {
    const others = S.profile.bindings.filter(b => b.dev === c.got.key && b.input === c.got.input && !(b.map === c.act.map && b.action === c.act.name));
    z.innerHTML = `Rilevato:<big>js${jsNum(c.got.key)} · ${esc(inputLabel(c.got.input))}</big>${esc(devName(c.got.key))}` +
      (others.length ? `<div class="def" style="margin-top:8px">Questo ingresso è già usato anche per: ${others.map(b => esc(actLabel(b))).join(', ')}</div>` : '') +
      (c.act.axis && /^(button|hat)/.test(c.got.input) ? `<div class="def" style="margin-top:8px">Nota: questo comando è pensato per un asse.</div>` : '');
  } else z.innerHTML = `Rilevato:<big>${esc(kbLabel({ dev: c.got.dev, input: c.got.input }))}</big>`;
  $('#cok').classList.remove('hidden'); $('#cretry').classList.remove('hidden');
}
function captureTick() {
  const c = S.capture; if (!c || c.got) return;
  for (const key in S.input) {
    if (!S.profile.devices[key]) continue;
    const cur = S.input[key], base = c.base[key] || { b: [], a: {}, h: [] };
    for (const b of cur.b || []) if (!(base.b || []).includes(b) && !isIgnored(key, 'button' + b)) { c.got = { key, input: 'button' + b }; return showGot(); }
    for (let i = 0; i < (cur.h || []).length; i++) { const d = hatDir(cur.h[i]); if (d && cur.h[i] !== (base.h || [])[i] && !isIgnored(key, 'hat' + (i + 1) + '_' + d)) { c.got = { key, input: 'hat' + (i + 1) + '_' + d }; return showGot(); } }
    for (const a in (cur.a || {})) if (!isIgnored(key, a) && Math.abs(cur.a[a] - ((base.a || {})[a] ?? cur.a[a])) > 22000) { c.got = { key, input: a }; return showGot(); }
  }
}
function captureGP() {
  const c = S.capture, g = S.gp; if (!c || c.got || !g) return;
  const z = $('#capzone');
  if (!g.on) { if (z) z.innerHTML = 'Nessun gamepad rilevato. Collegalo e accendilo…'; return; }
  c.order = c.order || []; c.gbase = c.gbase || JSON.parse(JSON.stringify(g));
  for (const b of g.p || []) if (!c.order.includes(b) && !(c.gbase.p || []).includes(b)) c.order.push(b);
  for (const a in (g.a || {})) if (Math.abs(g.a[a] - (c.gbase.a[a] ?? g.a[a])) > 20000) { c.got = { dev: 'gp', input: [...c.order, a].join('+') }; return showGot(); }
  if (c.order.length && c.order.some(b => !(g.p || []).includes(b))) { c.got = { dev: 'gp', input: c.order.join('+') }; return showGot(); }
  if (c.order.length && z) z.innerHTML = 'Tieni premuto… <big>' + esc(fmtKeys(c.order.join('+'))) + '</big>rilascia per confermare, oppure aggiungi un altro tasto o una levetta';
}
function gotKBM(dev, input) { S.capture.got = { dev, input }; showGot(); }
function confirmCapture() {
  const c = S.capture; if (!c || !c.got) return;
  if (c.kind === 'edjs') { S.capture = null; $('#modal').classList.add('hidden'); edAssign(c.tag, c.got.key, c.got.input, c.sl, c.got.dir > 0 ? 'Pos' : 'Neg'); return; }
  const extra = $('#cdbl').checked ? { multiTap: '2' } : null;
  S.capture = null; $('#modal').classList.add('hidden');
  if (c.kind === 'js') assignJS(c.act, c.got.key, c.got.input, extra);
  else if (c.kind === 'gp') { S.profile.bindings = S.profile.bindings.filter(b => !(b.map === c.act.map && b.action === c.act.name && b.dev === 'gp')); S.profile.bindings.push(Object.assign({ map: c.act.map, action: c.act.name, dev: 'gp', input: c.got.input }, extra || {})); saveProfile(); render(); }
  else assignKBM(c.act, c.got.dev, c.got.input, extra);
}
const CODES = { Space: 'space', Enter: 'enter', Escape: 'escape', Tab: 'tab', Backspace: 'backspace', ShiftLeft: 'lshift', ShiftRight: 'rshift', ControlLeft: 'lctrl', ControlRight: 'rctrl', AltLeft: 'lalt', AltRight: 'ralt', ArrowUp: 'up', ArrowDown: 'down', ArrowLeft: 'left', ArrowRight: 'right', Insert: 'insert', Delete: 'delete', Home: 'home', End: 'end', PageUp: 'pgup', PageDown: 'pgdn', NumpadAdd: 'np_add', NumpadSubtract: 'np_subtract', NumpadMultiply: 'np_multiply', NumpadDivide: 'np_divide', NumpadDecimal: 'np_period', NumpadEnter: 'np_enter', Minus: 'minus', Equal: 'equals', BracketLeft: 'lbracket', BracketRight: 'rbracket', Semicolon: 'semicolon', Quote: 'apostrophe', Backquote: 'tilde', Backslash: 'backslash', Comma: 'comma', Period: 'period', Slash: 'slash', CapsLock: 'capslock', NumLock: 'numlock', ScrollLock: 'scrolllock', Pause: 'pause', PrintScreen: 'print', IntlBackslash: 'oem_102' };
const MODS = ['lalt', 'ralt', 'lctrl', 'rctrl', 'lshift', 'rshift'];
function codeToKey(code) {
  let m;
  if ((m = /^Key([A-Z])$/.exec(code))) return m[1].toLowerCase();
  if ((m = /^Digit(\d)$/.exec(code))) return m[1];
  if ((m = /^F(\d+)$/.exec(code))) return 'f' + m[1];
  if ((m = /^Numpad(\d)$/.exec(code))) return 'np_' + m[1];
  return CODES[code] || null;
}
document.addEventListener('keydown', e => {
  const c = S.capture;
  if (!c) { if (e.key === 'Escape' && !$('#modal').classList.contains('hidden')) closeModal(); return; }
  if (c.got) { if (e.key === 'Enter') { e.preventDefault(); confirmCapture(); } else if (e.key === 'Escape') closeModal(); return; }
  if (c.kind !== 'kbm') { if (e.key === 'Escape') closeModal(); return; }
  if (e.target && e.target.id === 'cdbl') return;
  e.preventDefault();
  const k = codeToKey(e.code); if (!k) return;
  if (MODS.includes(k)) { if (!c.mods.includes(k)) c.mods.push(k); $('#capzone').innerHTML = 'Tieni premuto… <big>' + esc(c.mods.map(x => KEYN[x]).join(' + ')) + ' + ?</big>'; return; }
  gotKBM('kb', [...c.mods, k].join('+'));
}, true);
document.addEventListener('keyup', e => {
  const c = S.capture; if (!c || c.kind !== 'kbm' || c.got) return;
  const k = codeToKey(e.code);
  if (k && MODS.includes(k) && c.mods.includes(k)) { gotKBM('kb', c.mods.join('+')); }
}, true);

async function doImport(silent) {
  const inst = curInstall(); if (!inst) return toast('Installazione di Star Citizen non trovata', true);
  if (!silent && S.profile.bindings.length && !confirm('Importo i comandi dal gioco (' + inst.name + ').\nLe assegnazioni presenti nell\'app vengono sostituite con quelle del file del gioco (immagini ed etichette restano).\n\nContinuo?')) return;
  try {
    await S.saving;
    const r = await api('/api/import', { path: inst.path });
    S.profile = r.profile; S.profRev = r.profRev;
    await pollState(); render();
    toast('Importati dal gioco: ' + S.profile.bindings.length + ' assegnazioni, ' + Object.keys(S.profile.devices).length + ' periferiche');
  } catch (e) { toast(e.message, true); }
}
async function doSaveFile() {
  try {
    await S.saving;
    const r = await api('/api/savefile', {});
    openModal(`<h2>File salvato nella cartella dell'app</h2><div>File: <b>${esc(r.stats.path)}</b></div>
      <div>Per usarlo, a gioco chiuso copialo in:<br><b>…\\StarCitizen\\LIVE\\user\\client\\0\\Profiles\\default\\</b><br>sostituendo quello esistente (fanne prima una copia).</div>
      <pre>${esc((r.stats.joysticks || []).join('\n') || '(nessun joystick collegato)')}</pre>
      <div class="en">Attenzione: i numeri js qui sopra valgono per le periferiche collegate adesso.</div>
      <div class="btns"><button data-close="1" class="primary">Chiudi</button></div>`);
  } catch (e) { toast(e.message, true); }
}
async function browseRoot() {
  try {
    const r = await api('/api/browse', {});
    if (!r.path) return;
    S.profile.customRoot = r.path; await saveProfile(); await pollState(); render();
    toast(S.installs.length ? 'Cartella impostata: ' + r.path : 'In quella cartella non trovo Star Citizen', !S.installs.length);
  } catch (e) { toast(e.message, true); }
}
async function doWrite() {
  const inst = curInstall(); if (!inst) return toast('Installazione di Star Citizen non trovata', true);
  try {
    await S.saving;
    const pv = await api('/api/preview');
    const st = pv.stats;
    let msg = 'Scrivo nel gioco il layout "' + S.profile.layout + '" (Star Citizen ' + inst.name + ').\nSostituisce i comandi attualmente in uso nel gioco; del file attuale viene fatta una copia di sicurezza.\n\nNumerazione periferiche:\n  ' + (st.joysticks.join('\n  ') || '(nessun joystick collegato)') +
      '\n\nAssegnazioni: ' + st.written + (st.cleared ? '\nPredefiniti joystick disattivati: ' + st.cleared : '');
    if (st.skipped) msg += '\n\nATTENZIONE: ' + st.skipped + ' assegnazioni non vengono scritte perché la periferica è scollegata (' + st.skippedDevs.join(', ') + '). Restano salvate nell\'app.';
    msg += '\n\nCartella: ' + inst.path + (inst.launcher ? '  (è quella avviata dal launcher)' : (S.installs.some(i => i.launcher) ? '\nATTENZIONE: NON è la cartella da cui il launcher avvia il gioco!' : ''));
    msg += '\n\nStar Citizen deve essere chiuso. Viene fatta una copia di sicurezza del file attuale.\n\nProcedo?';
    if (!confirm(msg)) return;
    const r = await api('/api/write', { path: inst.path });
    S.profile = r.profile; S.profRev = r.profRev;
    await pollState(); render();
    openModal(`<h2>Fatto: layout "${esc(S.profile.layout)}" scritto nel gioco</h2>
      <div>File: <b>${esc(r.stats.path)}</b></div>
      <div>Copia di sicurezza del file precedente: ${esc(r.stats.backup || '(non esisteva)')}</div>
      <pre>${esc(r.stats.joysticks.join('\n'))}</pre>
      <div>Avvia Star Citizen: troverai i comandi già assegnati.</div>
      <div class="btns"><button data-close="1" class="primary">Chiudi</button></div>`);
  } catch (e) { toast(e.message, true); }
}

// ---------- eventi ----------
$('#btnImport').onclick = () => isED() ? edImport(false) : doImport(false);
$('#btnWrite').onclick = () => isED() ? edWrite() : doWrite();
$('#game').onchange = e => setGame(e.target.value);
$('#btnMenu').onclick = e => { e.stopPropagation(); $('#lymenu').classList.add('hidden'); $('#menu').classList.toggle('hidden'); };
$('#btnLayout').onclick = e => { e.stopPropagation(); $('#menu').classList.add('hidden'); const m = $('#lymenu'); m.style.left = e.target.getBoundingClientRect().left + 'px'; m.classList.toggle('hidden'); };
$('#lymenu').addEventListener('click', e => { if (e.target.dataset.ly) layoutAction(e.target.dataset.ly); });
$('#layout').onchange = e => switchLayout(e.target.value);
document.addEventListener('click', () => { $('#menu').classList.add('hidden'); $('#lymenu').classList.add('hidden'); });
$('#install').onchange = e => { S.profile.install = e.target.value; saveProfile().then(pollState); };
$('#modal').addEventListener('click', e => { if (e.target.id === 'modal' || e.target.dataset.close) closeModal(); });
$('#banner').addEventListener('click', e => {
  if (e.target.dataset.b === 'import') doImport(false);
  if (e.target.dataset.b === 'write') doWrite();
  if (e.target.dataset.b === 'savefile') doSaveFile();
  if (e.target.dataset.b === 'autoorder') { S.profile.orderManual = false; saveProfile().then(pollState).then(render); }
  if (e.target.dataset.b === 'useinst') { const u = S.installs.find(i => i.launcher); if (u) { S.profile.install = u.path; saveProfile().then(pollState).then(render); } }
  if (e.target.id === 'rootBrowse') browseRoot();
  if (e.target.id === 'rootSave') { S.profile.customRoot = $('#rootInput').value.trim(); saveProfile().then(pollState); }
});
$('#menu').addEventListener('click', async e => {
  const a = e.target.dataset.act; if (!a) return;
  const inst = curInstall();
  try {
    if (a === 'preview') { const r = await api('/api/preview'); openModal(`<h2>Anteprima di actionmaps.xml</h2><pre>${esc(r.xml)}</pre><div class="btns"><button data-close="1">Chiudi</button></div>`); }
    if (a === 'savefile') doSaveFile();
    if (a === 'opengame') { if (inst) await api('/api/open', { what: 'game', path: inst.path }); }
    if (a === 'opendata') await api('/api/open', { what: 'dati' });
    if (a === 'log') { const r = await api('/api/log'); openModal(`<h2>Diagnostica periferiche</h2><div class="en">Se una periferica non viene letta bene, copia questo testo e mandalo a chi ti ha fatto l'app.</div><pre>${esc(r.join('\n'))}\n\n${esc(JSON.stringify(S.devices, null, 1))}</pre><div class="btns"><button data-close="1">Chiudi</button></div>`); }
    if (a === 'about') openModal(`<h2>ScEd Binder v${esc(S.version)}</h2><div>Star Citizen + Elite Dangerous · github.com/Toskurim/ScEd-Binder</div><div>Comandi del gioco: ${esc(S.gameData)}</div><div>Cartella dati: ${esc(S.dataDir)}</div><div class="en">Schemi delle periferiche: progetto open source Joystick Diagrams (github.com/Rexeh/joystick-diagrams, licenza GPL-2.0). Traduzione di base dei comandi: comunità italiana di Star Citizen (progetto StarCitizen-Localization), riveduta. Regole di numerazione dei joystick: osservazioni del progetto BindSight. Star Citizen è un marchio di Cloud Imperium Games, Elite Dangerous di Frontier Developments; questa app non è affiliata a nessuno dei due.</div><div class="btns"><button data-close="1">Chiudi</button></div>`);
    if (a === 'options') {
      openModal(`<h2>Opzioni</h2>
        <label class="chk"><input type="checkbox" id="oclr" ${S.profile.clearJsDefaults ? 'checked' : ''}> <span><b>Disattiva i tasti joystick predefiniti del gioco</b> per i comandi a cui non hai assegnato nulla. Consigliato con più periferiche: evita che il primo joystick abbia comandi "fantasma" decisi dal gioco.</span></label>
        <label>Cartella di Star Citizen (solo se non viene trovata da sola):<br><input type="text" id="oroot" style="width:100%" value="${esc(S.profile.customRoot || '')}" placeholder="es. D:\\Giochi\\StarCitizen"> <button class="small" id="obrowse">Sfoglia…</button></label>
        <div class="btns"><button data-close="1">Annulla</button><button class="primary" id="osave">Salva</button></div>`);
      $('#obrowse').onclick = async () => { try { const r = await api('/api/browse', {}); if (r.path) $('#oroot').value = r.path; } catch (err) { toast(err.message, true); } };
      $('#osave').onclick = () => { S.profile.clearJsDefaults = $('#oclr').checked; S.profile.customRoot = $('#oroot').value.trim(); saveProfile().then(pollState); $('#modal').classList.add('hidden'); render(); };
    }
  } catch (err) { toast(err.message, true); }
});
$('#navAll').onclick = () => { S.view = { type: 'all' }; render(); };
$('#navShip').onclick = () => { S.view = { type: 'ship' }; render(); };
$('#devlist').addEventListener('click', e => {
  const el = e.target.closest('.dev'); if (!el) return;
  const key = el.dataset.key;
  if (e.target.dataset.mv) {
    e.stopPropagation();
    const on = S.profile.order.filter(k => liveDev(k)), i = on.indexOf(key), j = i + (+e.target.dataset.mv);
    if (j < 0 || j >= on.length) return;
    const o = S.profile.order, a = o.indexOf(key), b = o.indexOf(on[j]);
    [o[a], o[b]] = [o[b], o[a]]; S.profile.orderManual = true;
    // aggiorno subito i numeri mostrati
    const on2 = o.filter(k => liveDev(k)); on2.forEach((k, n) => { liveDev(k).js = n + 1; });
    saveProfile().then(pollState); render(); return;
  }
  S.view = { type: 'dev', key }; S.placing = null; S.placingMode = false; S.learn = null; render();
});
$('#view').addEventListener('input', e => {
  if (e.target.id === 'edq') { S.edFilter.q = e.target.value; const pos = e.target.selectionStart; renderAllED(); const q = $('#edq'); q.focus(); q.setSelectionRange(pos, pos); }
  if (e.target.id === 'q') { S.filter.q = e.target.value; const pos = e.target.selectionStart; renderAll(); const q = $('#q'); q.focus(); q.setSelectionRange(pos, pos); }
});
$('#view').addEventListener('change', async e => {
  const t = e.target;
  if (t.id === 'fmap') { S.filter.map = t.value; renderAll(); }
  if (t.id === 'fmode') { S.filter.mode = t.value; renderAll(); }
  if (t.id === 'fhid') { S.filter.hidden = t.checked; renderAll(); }
  if (t.id === 'edgroup') { S.edFilter.group = t.value; renderAllED(); }
  if (t.id === 'edmode') { S.edFilter.mode = t.value; renderAllED(); }
  if (t.dataset.edinv) { const b = (edB()[t.dataset.edinv] = edB()[t.dataset.edinv] || {}); b.inv = t.checked; saveProfile().then(pollState); return; }
  if (t.id === 'follow') S.follow = t.checked;
  if (t.dataset.inv) { setInvert(S.view.type === 'ship' ? t.dataset.dev : S.view.key, t.dataset.inv, t.checked); if (S.view.type === 'dev') renderDevice(); return; }
  if (t.id === 'schfile' && t.files[0]) {
    try {
      const txt = await t.files[0].text();
      const r = await fetch('/api/schema', { method: 'POST', headers: { 'X-SCB': '1', 'Content-Type': 'application/json' }, body: txt });
      const j = await r.json(); if (!r.ok) throw new Error(j.error);
      const di = devInfo(S.view.key); di.image = j.image; di.hotspots = j.hotspots || []; S.placingMode = false;
      if (j.photoMap && j.image.startsWith('foto:')) { di.photoMap = di.photoMap || {}; di.photoMap[j.image.slice(5)] = j.photoMap; }
      saveProfile(); renderDevice(); toast('Schema importato' + (j.name ? ': ' + j.name : ''));
    } catch (err) { toast(err.message || 'Schema non valido', true); }
  }
  if (t.id === 'imgsel') { devInfo(S.view.key).image = t.value; S.placingMode = false; S.learn = null; saveProfile(); renderDevice(); }
  if (t.id === 'imgfile' && t.files[0]) {
    const f = t.files[0], ext = (f.name.split('.').pop() || '').toLowerCase();
    try {
      const r = await fetch('/api/upload?ext=' + encodeURIComponent(ext), { method: 'POST', headers: { 'X-SCB': '1' }, body: f });
      const j = await r.json(); if (!r.ok) throw new Error(j.error);
      const di = devInfo(S.view.key); di.image = j.image; di.hotspots = []; S.placingMode = true;
      saveProfile(); renderDevice();
    } catch (err) { toast(err.message || 'Caricamento non riuscito', true); }
  }
});
$('#view').addEventListener('contextmenu', e => {
  const pr = e.target.closest('.pr[data-ctl]');
  if (pr && S.view.type === 'dev') { e.preventDefault(); S.learn = { ctl: pr.dataset.ctl, idx: +pr.dataset.idx }; renderImage(); return; }
  const hs = e.target.closest('.hs'); if (!hs) return;
  e.preventDefault(); devInfo(S.view.key).hotspots.splice(+hs.dataset.hs, 1); saveProfile(); renderImage();
});
$('#view').addEventListener('click', e => {
  const t = e.target;
  if (t.dataset.edinv) return;
  if (t.dataset.edrm) { const [tag, sl] = t.dataset.edrm.split('|'); edClear(tag, sl); return; }
  const ecell = t.closest('td[data-edcap]');
  if (ecell) { if (t.closest('.inv')) return; startCaptureED(ecell.parentElement.dataset.ed, ecell.dataset.edcap); return; }
  // tabella di tutti i comandi
  const cell = t.closest('td.c');
  if (cell) { const act = S.actions.get(cell.parentElement.dataset.a); if (act) startCapture(act, cell.dataset.cap); return; }
  if (S.view.type === 'ship') { const r = t.closest('[data-bind]'); if (r && !t.closest('.inv')) { const act = S.actions.get(r.dataset.bind); if (act) startCapture(act, 'js'); } return; }
  if (S.view.type !== 'dev') return;
  const key = S.view.key, di = devInfo(key);
  if (t.id === 'rename') { const v = prompt('Nome da mostrare per questa periferica:', devName(key)); if (v !== null) { di.alias = v.trim(); saveProfile(); render(); } return; }
  if (t.id === 'forget') { if (confirm('Dimentico "' + devName(key) + '" e tutte le sue assegnazioni?')) { S.profile.bindings = S.profile.bindings.filter(b => b.dev !== key); delete S.profile.devices[key]; S.profile.order = S.profile.order.filter(k => k !== key); S.view = { type: 'all' }; saveProfile(); render(); } return; }
  if (t.id === 'imgup') { $('#imgfile').click(); return; }
  if (t.id === 'imgweb') { window.open('https://www.bing.com/images/search?q=' + encodeURIComponent(devName(key) + ' button layout diagram'), '_blank'); return; }
  if (t.id === 'schimp') { $('#schfile').click(); return; }
  if (t.id === 'schexp') {
    S.saving.then(async () => {
      try {
        const r = await fetch('/api/schema?key=' + encodeURIComponent(key)); if (!r.ok) throw new Error((await r.json()).error);
        const a = document.createElement('a'); a.href = URL.createObjectURL(await r.blob());
        a.download = 'schema_' + devName(key).replace(/[^A-Za-z0-9]+/g, '_') + '.json'; a.click();
      } catch (err) { toast(err.message, true); }
    });
    return;
  }
  if (t.id === 'placemode') { S.placingMode = !S.placingMode; S.placing = null; renderDevice(); return; }
  const row = t.closest('.row[data-input]');
  if (row) {
    const input = row.dataset.input;
    if (t.dataset.rm !== undefined) { const bs = S.profile.bindings.filter(b => b.dev === key && b.input === input); removeBinding(bs[+t.dataset.rm]); return; }
    if (t.dataset.add) { pickCommand(key, input); return; }
    if (t.dataset.ign) { di.ignored = di.ignored || []; di.ignored = di.ignored.includes(input) ? di.ignored.filter(x => x !== input) : [...di.ignored, input]; saveProfile(); renderDevice(); return; }
    if (t.dataset.inv) return;
    if (t.dataset.place) { S.placingMode = true; S.placing = input; renderDevice(); return; }
    return;
  }
  const pr = t.closest('.pr[data-ctl]');
  if (pr) {
    if (pr.dataset.input && !S.learn) { pickCommand(key, pr.dataset.input); return; }
    S.learn = S.learn && S.learn.ctl === pr.dataset.ctl && S.learn.idx === +pr.dataset.idx ? null : { ctl: pr.dataset.ctl, idx: +pr.dataset.idx };
    renderImage(); return;
  }
  const ph = t.closest('.ph[data-input]');
  if (ph) { pickCommand(key, ph.dataset.input); return; }
  const hs = t.closest('.hs');
  if (hs && !S.placingMode) { pickCommand(key, di.hotspots[+hs.dataset.hs].input); return; }
  const box = t.closest('#custimg');
  if (box && S.placingMode && S.placing) {
    const r = box.getBoundingClientRect();
    const x = +((e.clientX - r.left) / r.width * 100).toFixed(2), y = +((e.clientY - r.top) / r.height * 100).toFixed(2);
    di.hotspots = (di.hotspots || []).filter(h => h.input !== S.placing);
    di.hotspots.push({ input: S.placing, x, y });
    S.placing = null; saveProfile(); renderDevice();
  }
});
window.addEventListener('pagehide', () => { try { navigator.sendBeacon('/api/bye'); } catch (e) { } });

boot().catch(e => { document.body.innerHTML = '<p style="padding:30px">Errore di avvio: ' + esc(e.message) + '</p>'; });
