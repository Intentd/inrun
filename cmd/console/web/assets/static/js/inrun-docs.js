// Inrun Console — docs page JS
(function () {
  // Theme toggle
  var themeBtn = document.getElementById('inrun-theme-toggle');
  if (themeBtn) {
    themeBtn.addEventListener('click', function () {
      var t = document.documentElement.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', t);
      localStorage.setItem('console-theme', t);
    });
  }

  // Sidebar group toggles
  document.querySelectorAll('.inrun-sidebar-group-toggle').forEach(function (btn) {
    btn.addEventListener('click', function () {
      var group = btn.closest('.inrun-sidebar-group');
      if (group) group.classList.toggle('is-open');
    });
  });

  // Highlight active sidebar link based on scroll position
  var headings = document.querySelectorAll('.inrun-doc-body h2[id], .inrun-doc-body h3[id]');
  var sidebarLinks = document.querySelectorAll('.inrun-sidebar-item a[href^="#"]');
  if (headings.length && sidebarLinks.length) {
    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          sidebarLinks.forEach(function (a) { a.closest('.inrun-sidebar-item').classList.remove('active'); });
          var match = document.querySelector('.inrun-sidebar-item a[href="#' + entry.target.id + '"]');
          if (match) match.closest('.inrun-sidebar-item').classList.add('active');
        }
      });
    }, { rootMargin: '-20% 0px -70% 0px' });
    headings.forEach(function (h) { observer.observe(h); });
  }
})();
