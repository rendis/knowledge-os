/* Run in a permitted browser context after loading the artifact. No navigation,
 * network or browser control. In Node, only the pure geometry tests are available. */
(function (root) {
  function containsText(box, text, padding = 8) {
    return text.left >= box.left + padding && text.top >= box.top + padding &&
      text.right <= box.right - padding && text.bottom <= box.bottom - padding;
  }
  function svgBounds(element) {
    const matrix = element.getScreenCTM();
    if (!matrix || Math.abs(matrix.b) > 1e-6 || Math.abs(matrix.c) > 1e-6 || matrix.a <= 0 || matrix.d <= 0) {
      throw new Error('Rotated, skewed or reflected geometry needs manual inspection.');
    }
    const bounds = element.getBBox();
    return {left: bounds.x * matrix.a + matrix.e, top: bounds.y * matrix.d + matrix.f,
      right: (bounds.x + bounds.width) * matrix.a + matrix.e,
      bottom: (bounds.y + bounds.height) * matrix.d + matrix.f};
  }
  async function checkTextFit(doc = document) {
    if (doc.fonts) await doc.fonts.ready;
    const errors = [], unverified = [], results = [];
    const boxes = [...doc.querySelectorAll('[data-text-box]')];
    const labels = [...doc.querySelectorAll('[data-fit-box]')];
    if (!boxes.length) unverified.push('No annotated text containers.');
    for (const box of boxes) {
      if (!box.getClientRects().length) continue; // Inactive responsive variant.
      const owned = labels.filter(label => label.getAttribute('data-fit-box') === box.id);
      if (!box.id || !owned.length) errors.push('Visible text container lacks an id or linked labels.');
    }
    for (const label of labels) {
      if (!label.getClientRects().length) continue;
      const ref = label.getAttribute('data-fit-box');
      const box = doc.getElementById(ref);
      if (!box || !box.hasAttribute('data-text-box')) { errors.push(`Unknown text container: ${ref}`); continue; }
      const padding = Number(box.getAttribute('data-fit-padding') ?? 8);
      if (!Number.isFinite(padding) || padding < 0) { errors.push(`Invalid padding: ${ref}`); continue; }
      try {
        let outer, inner;
        if (typeof label.getBBox === 'function' && typeof box.getBBox === 'function') {
          outer = svgBounds(box); inner = svgBounds(label);
        } else {
          outer = box.getBoundingClientRect();
          const range = doc.createRange(); range.selectNodeContents(label);
          inner = range.getBoundingClientRect();
        }
        const valid = [outer.left, outer.top, outer.right, outer.bottom, inner.left, inner.top, inner.right, inner.bottom].every(Number.isFinite);
        if (!valid || inner.right <= inner.left || inner.bottom <= inner.top) throw new Error('No measurable text bounds.');
        const pass = containsText(outer, inner, padding);
        results.push({box: ref, text: label.textContent.trim(), pass, bounds: inner, container: outer, padding});
        if (!pass) errors.push(`Text exceeds usable bounds of ${ref}: ${label.textContent.trim()}`);
      } catch (error) { unverified.push(`${ref}: ${error.message}`); }
    }
    if (!results.length) unverified.push('No visible text measured.');
    return {text_fit_pass: !errors.length && !unverified.length, results, errors, unverified};
  }
  if (typeof module !== 'undefined' && module.exports) module.exports = {containsText, checkTextFit};
  else root.checkTextFit = checkTextFit;
})(globalThis);
