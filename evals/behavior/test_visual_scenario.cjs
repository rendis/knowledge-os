// Execute the template's real render function with minimal DOM stand-ins.
// Checks numeric SVG output, not browser layout or interactions.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const html = fs.readFileSync(process.argv[2] || path.resolve(__dirname, '../../kernel/.agents/skills/explain-visually/assets/explorer.html'), 'utf8');
const days = Number(process.argv[3] || 5);
const formatted = value => new Intl.NumberFormat('en-US').format(value);
const elements = new Map();
const document = {querySelector(selector) {
  if (!elements.has(selector)) elements.set(selector, {value: '', defaultValue: selector === '#volume' ? '275' : '80', listeners: {}, addEventListener(type, fn) { this.listeners[type] = fn; }});
  return elements.get(selector);
}};
const code = html.match(/<script>([\s\S]*?)<\/script>/)[1];
const ticks = [...html.matchAll(/class="axis-label"[^>]*>([\d,]+)<\/text>/g)].map(m => Number(m[1].replaceAll(',', '')));
let count = 0;
for (const volume of [100, 275, 500]) for (const rate of [60, 80, 95]) {
  document.querySelector('#volume').value = volume;
  document.querySelector('#rate').value = rate;
  document.querySelector('#case').value = 'expected';
  vm.runInNewContext(code, {document, Intl}, {timeout: 1000});
  const bars = [...elements.get('#chart-bars').innerHTML.matchAll(/<rect[^>]*y="([\d.-]+)"[^>]*height="([\d.-]+)"/g)];
  assert.equal(bars.length, 4);
  for (const [, y, height] of bars) {
    assert.ok(Number(y) >= 28, 'bar stays within the plot, including maximum inputs');
    assert.equal(Number(y) + Number(height), 256);
  }
  const units = Math.round(volume * rate / 100 * days);
  assert.equal(elements.get('#weekly-output').textContent, new Intl.NumberFormat('en-US').format(units));
  assert.ok(Math.abs(Number(bars[3][2]) - units / ticks[0] * 228) <= 0.5, 'axis agrees with bar scale');
  assert.ok(elements.get('#comparison-rows').innerHTML.includes(`<td>${Math.round(rate)}%</td><td>${new Intl.NumberFormat('en-US').format(units)}</td>`), 'text table includes the actual projection');
  count++;
}
assert.deepEqual(ticks, [ticks[0], ticks[0] * .75, ticks[0] * .5, ticks[0] * .25, 0]);
console.log(`${count} scenario boundary combinations passed (browser unverified)`);

document.querySelector('#case').value = 'stretch';
document.querySelector('#case').listeners.change();
assert.equal(elements.get('#weekly-output').textContent, formatted(500 * .95 * days), 'reference selection does not change projection');
assert.equal(elements.get('#result-title').textContent, 'Your projection');
document.querySelector('#reset').listeners.click();
assert.equal(elements.get('#volume').value, '275');
assert.equal(elements.get('#rate').value, '80');
assert.equal(elements.get('#case').value, 'expected');
assert.equal(elements.get('#weekly-output').textContent, formatted(275 * .8 * days));
console.log('Reference independence and reset passed (DOM stand-ins, browser unverified)');
