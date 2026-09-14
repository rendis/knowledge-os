// Execute template handlers with DOM stand-ins. Does not verify browser layout/CSS.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const html = fs.readFileSync(path.resolve(__dirname, '../../kernel/.agents/skills/explain-visually/assets/path-explorer.html'), 'utf8');
function element(attrs = {}) {
  const classes = new Set((attrs.class || '').split(/\s+/));
  return { id:attrs.id, attrs, dataset:Object.fromEntries(Object.entries(attrs).filter(([k])=>k.startsWith('data-')).map(([k,v])=>[k.slice(5),v])), textContent:'', innerHTML:'', listeners:{},
    classList:{add(...xs){xs.forEach(x=>classes.add(x));},remove(...xs){xs.forEach(x=>classes.delete(x));},contains(x){return classes.has(x);},toggle(x,force){const yes=force ?? !classes.has(x); if(yes)classes.add(x);else classes.delete(x);return yes;}},
    setAttribute(k,v){attrs[k]=v;},addEventListener(k,fn){this.listeners[k]=fn;} };
}
const all = [...html.matchAll(/<(?:g|path|text|button|div|h2|p|ol)\b([^>]*)>/g)].map(m=>element(Object.fromEntries([...m[1].matchAll(/([\w-]+)="([^"]*)"/g)].map(a=>[a[1],a[2]]))));
const document = {documentElement:element(),querySelector(q){return all.find(e=>e.id===q.slice(1));},querySelectorAll(q){return all.filter(e=>e.classList.contains(q.slice(1)));}};
vm.runInNewContext(html.match(/<script>([\s\S]*?)<\/script>/)[1], {document, localStorage:{getItem(){return null;},setItem(){}}}, {timeout:1000});
const nodes = document.querySelectorAll('.node'), edges = document.querySelectorAll('.edge'), labels = document.querySelectorAll('.edge-label'), routes = document.querySelectorAll('.route');
const node = id => nodes.find(n=>n.dataset.node===id);
const ids = (elements,key) => elements.filter(e=>e.classList.contains('lit')).map(key).sort();
let checks=0;
for (const selected of nodes) {
  selected.listeners.click();
  const incident = edges.filter(e=>e.dataset.from===selected.dataset.node || e.dataset.to===selected.dataset.node);
  assert.deepEqual(ids(edges,e=>e.id),incident.map(e=>e.id).sort());
  assert.deepEqual(ids(labels,e=>e.dataset.edge),incident.map(e=>e.id).sort());
  assert.deepEqual(ids(nodes,e=>e.dataset.node),[...new Set([selected.dataset.node,...incident.flatMap(e=>[e.dataset.from,e.dataset.to])])].sort());
  assert.deepEqual(nodes.filter(n=>n.attrs['aria-pressed']==='true'),[selected]);
  checks++;
}
const expectedRoutes={read:['e-reader-gateway','e-gateway-docs','e-docs-renderer','e-renderer-reader'],publish:['e-editor-gateway','e-gateway-queue','e-queue-docs','e-queue-audit'],index:['e-docs-indexer','e-indexer-search','e-search-gateway']};
for(const route of routes) {
  node('gateway').listeners.click();
  route.listeners.click();
  assert.deepEqual(ids(edges,e=>e.id),expectedRoutes[route.dataset.route].sort());
  assert.equal(document.querySelector('#node-title').textContent,'Select a node');
  assert.ok(nodes.every(n=>n.attrs['aria-pressed']==='false'));
  node('media').listeners.click();
  assert.ok(routes.every(r=>r.attrs['aria-pressed']==='false'));
  assert.equal(document.querySelector('#steps').innerHTML,'');
  checks++;
}
for(const key of ['Enter',' ']) {
  let prevented=false;
  node('gateway').listeners.keydown({key,preventDefault(){prevented=true;}});
  assert.ok(prevented);
  assert.equal(node('gateway').attrs['aria-pressed'],'true');
  checks++;
}
for(const control of ['all','reset']) {
  node('gateway').listeners.click();
  document.querySelector('#'+control).listeners.click();
  assert.ok(!document.querySelector('#stage').classList.contains('focused'));
  assert.ok([...nodes,...edges,...labels].every(e=>!e.classList.contains('lit')));
  assert.equal(document.querySelector('#node-title').textContent,'Select a node');
  checks++;
}
const theme=document.querySelector('#theme');theme.listeners.click();assert.equal(theme.attrs['aria-pressed'],'true');theme.listeners.click();assert.equal(theme.attrs['aria-pressed'],'false');checks++;
console.log(`${checks} spatial handler cases passed (DOM stand-ins; browser unverified)`);
