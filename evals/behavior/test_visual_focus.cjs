// Exercise the exact pure neighborhood calculation embedded in the offline template.
// This is a logic test, not a browser or rendered-UI verification.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const files = process.argv.slice(2);
if (!files.length) files.push(path.resolve(__dirname, '../../kernel/.agents/skills/explain-visually/assets/path-explorer.html'));
for (const file of files) {
  const html = fs.readFileSync(file, 'utf8');
  const begin = '// BEGIN focusForNode';
  const end = '// END focusForNode';
  assert.equal(html.split(begin).length, 2, 'one shared focus function marker');
  const code = html.split(begin)[1].split(end)[0];
  assert.ok(html.includes(end), 'focus function end marker');
  const focus = vm.runInNewContext(code + '\nfocusForNode;', {}, { timeout: 1000 });
  const edges = [{id:'ab',from:'a',to:'b'}, {id:'ca',from:'c',to:'a'}, {id:'bc',from:'b',to:'c'}, {id:'de',from:'d',to:'e'}];
  const normalize = result => ({nodeIds:[...result.nodeIds].sort(), edgeIds:[...result.edgeIds].sort()});
  assert.deepEqual(normalize(focus('a', edges)), {nodeIds:['a','b','c'],edgeIds:['ab','ca']}, 'incoming and outgoing only, no neighbor-to-neighbor or unrelated edge');
  assert.deepEqual(normalize(focus('isolated', edges)), {nodeIds:['isolated'],edgeIds:[]}, 'disconnected node stays active alone');
  assert.deepEqual(normalize(focus('a', [{id:'aa',from:'a',to:'a'}])), {nodeIds:['a'],edgeIds:['aa']}, 'self loop does not duplicate node');
  assert.deepEqual(normalize(focus('d', edges)), {nodeIds:['d','e'],edgeIds:['de']}, 'second selection replaces the neighborhood');
  console.log(`${file}: 4 focus logic cases passed (browser unverified)`);
}
