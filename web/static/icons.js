// Iconos locales: sustituye los <i data-lucide> por el sprite SVG de index.html.
// Sin CDN: el visor debe funcionar sin Internet y la CSP solo permite scripts propios.
(function () {
  function createIcons() {
    document.querySelectorAll('i[data-lucide]').forEach(function (el) {
      var name = el.getAttribute('data-lucide');
      var svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
      svg.setAttribute('class', ((el.getAttribute('class') || '') + ' lucide').trim());
      svg.setAttribute('aria-hidden', 'true');
      svg.innerHTML = '<use href="#i-' + name + '"></use>';
      el.replaceWith(svg);
    });
  }
  window.lucide = { createIcons: createIcons };
  document.addEventListener('DOMContentLoaded', createIcons);
})();
