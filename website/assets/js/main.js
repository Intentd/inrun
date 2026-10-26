/* ============================================================
   ORKESTRA — Main JavaScript
   - Dark mode toggle with localStorage persistence
   - Copy-to-clipboard for code blocks
   - Mobile sidebar drawer toggle
   - Active TOC item on scroll (IntersectionObserver)
   - Mobile navbar menu
   - Hero code tab switching
   ============================================================ */

(function () {
  'use strict';

  /* ── 1. Dark Mode ──────────────────────────────────────── */
  var THEME_KEY = 'theme';
  var html = document.documentElement;

  function getTheme() {
    // Respect saved preference; otherwise follow system preference; fallback to light
    var saved = localStorage.getItem(THEME_KEY);
    if (saved) return saved;
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }

  function applyTheme(theme) {
    html.setAttribute('data-theme', theme === 'dark' ? 'dark' : 'light');
    localStorage.setItem(THEME_KEY, theme === 'dark' ? 'dark' : 'light');
  }

  function toggleTheme() {
    var isDark = html.getAttribute('data-theme') === 'dark';
    applyTheme(isDark ? 'light' : 'dark');
  }

  // Apply on load
  applyTheme(getTheme());

  document.addEventListener('DOMContentLoaded', function () {
    var toggleBtn = document.getElementById('theme-toggle');
    if (toggleBtn) {
      toggleBtn.addEventListener('click', toggleTheme);
    }

    // Sync with system preference changes
    var mq = window.matchMedia('(prefers-color-scheme: dark)');
    mq.addEventListener('change', function (e) {
      if (!localStorage.getItem(THEME_KEY)) {
        applyTheme(e.matches ? 'dark' : 'light');
      }
    });
  });

  /* ── 2. Copy-to-Clipboard for Code Blocks ──────────────── */
  function addCopyButtons() {
    var preBlocks = document.querySelectorAll('pre.code-block, .prose pre, .highlight pre');

    preBlocks.forEach(function (pre) {
      // Skip if button already exists
      if (pre.querySelector('.code-copy-btn')) return;

      // Make the pre relative for button positioning
      var existingPosition = window.getComputedStyle(pre).position;
      if (existingPosition === 'static') {
        pre.style.position = 'relative';
      }

      var btn = document.createElement('button');
      btn.className = 'code-copy-btn';
      btn.setAttribute('aria-label', 'Copy code');
      btn.innerHTML =
        '<svg viewBox="0 0 24 24" width="13" height="13" fill="none" stroke="currentColor" stroke-width="2">' +
        '<rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>' +
        '<path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>' +
        '</svg>' +
        '<span>Copy</span>';

      btn.addEventListener('click', function () {
        var code = pre.querySelector('code');
        var text = code ? code.innerText : pre.innerText;

        navigator.clipboard.writeText(text).then(function () {
          btn.classList.add('copied');
          btn.querySelector('span').textContent = 'Copied!';
          btn.querySelector('svg').innerHTML =
            '<path d="M20 6L9 17l-5-5"/>';
          btn.querySelector('svg').setAttribute('stroke-width', '2.5');

          setTimeout(function () {
            btn.classList.remove('copied');
            btn.querySelector('span').textContent = 'Copy';
            btn.querySelector('svg').innerHTML =
              '<rect x="9" y="9" width="13" height="13" rx="2" ry="2"/>' +
              '<path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/>';
            btn.querySelector('svg').setAttribute('stroke-width', '2');
          }, 2000);
        }).catch(function () {
          // Fallback for browsers without clipboard API
          var ta = document.createElement('textarea');
          ta.value = text;
          ta.style.position = 'fixed';
          ta.style.opacity = '0';
          document.body.appendChild(ta);
          ta.select();
          document.execCommand('copy');
          document.body.removeChild(ta);
          btn.querySelector('span').textContent = 'Copied!';
          setTimeout(function () {
            btn.querySelector('span').textContent = 'Copy';
          }, 2000);
        });
      });

      pre.appendChild(btn);
    });
  }

  /* ── 3. Language Label for Code Blocks ─────────────────── */
  function addLangLabels() {
    var highlights = document.querySelectorAll('.prose .highlight');
    highlights.forEach(function (wrap) {
      if (wrap.querySelector('.code-lang-label')) return;
      var code = wrap.querySelector('code');
      if (!code) return;
      var lang = '';
      code.classList.forEach(function (cls) {
        if (cls.startsWith('language-')) {
          lang = cls.replace('language-', '');
        }
      });
      if (!lang || lang === 'plaintext' || lang === 'text') return;
      var label = document.createElement('div');
      label.className = 'code-lang-label';
      label.textContent = lang;
      wrap.insertBefore(label, wrap.firstChild);
    });
  }

  /* ── 4. Mobile Sidebar Drawer ──────────────────────────── */
  function initSidebar() {
    var sidebar = document.getElementById('docs-sidebar');
    var overlay = document.getElementById('sidebar-overlay');

    if (!sidebar) return;

    function openSidebar() {
      sidebar.classList.add('open');
      if (overlay) {
        overlay.classList.add('visible');
        overlay.setAttribute('aria-hidden', 'false');
      }
      document.body.style.overflow = 'hidden';
    }

    function closeSidebar() {
      sidebar.classList.remove('open');
      if (overlay) {
        overlay.classList.remove('visible');
        overlay.setAttribute('aria-hidden', 'true');
      }
      document.body.style.overflow = '';
    }

    // Mobile toggle button (injected below)
    var toggleBtn = document.getElementById('sidebar-mobile-toggle');
    if (toggleBtn) {
      toggleBtn.addEventListener('click', function () {
        sidebar.classList.contains('open') ? closeSidebar() : openSidebar();
      });
    }

    // Close on overlay click
    if (overlay) {
      overlay.addEventListener('click', closeSidebar);
    }

    // Close on escape key
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && sidebar.classList.contains('open')) {
        closeSidebar();
      }
    });

    // Inject mobile toggle button if we're on a docs page
    if (document.body.classList.contains('docs-body')) {
      var fab = document.createElement('button');
      fab.id = 'sidebar-mobile-toggle';
      fab.className = 'sidebar-mobile-toggle';
      fab.setAttribute('aria-label', 'Open navigation');
      fab.innerHTML =
        '<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2">' +
        '<line x1="3" y1="12" x2="21" y2="12"/>' +
        '<line x1="3" y1="6" x2="21" y2="6"/>' +
        '<line x1="3" y1="18" x2="21" y2="18"/>' +
        '</svg>' +
        'Menu';
      document.body.appendChild(fab);
      fab.addEventListener('click', function () {
        sidebar.classList.contains('open') ? closeSidebar() : openSidebar();
      });
    }
  }

  /* ── 4. Collapsible Sidebar Sections ───────────────────── */
  function initSidebarSections() {
    var toggles = document.querySelectorAll('.sidebar-section-toggle');


    toggles.forEach(function (toggle) {
      var section = toggle.closest('.sidebar-section');

      // Sync initial aria state
      var isOpen = section ? section.classList.contains('open') : false;
      toggle.setAttribute('aria-expanded', String(isOpen));

      toggle.addEventListener('click', function (e) {
        e.preventDefault();
        e.stopPropagation();
        if (!section) return;

        var willOpen = !section.classList.contains('open');
        section.classList.toggle('open', willOpen);
        toggle.setAttribute('aria-expanded', String(willOpen));
      });
    });

    // Also toggle section when clicking the section title link
    var sectionLinks = document.querySelectorAll('.sidebar-section-link');
    sectionLinks.forEach(function(link) {
      link.addEventListener('click', function() {
        var section = link.closest('.sidebar-section');
        if (!section) return;
        var toggle = section.querySelector('.sidebar-section-toggle');
        section.classList.add('open');
        if (toggle) toggle.setAttribute('aria-expanded', 'true');
      });
    });

    // Second-level subsection toggles
    var subToggles = document.querySelectorAll('.sidebar-subsection-toggle');
    subToggles.forEach(function(toggle) {
      var subsection = toggle.closest('.sidebar-subsection');

      toggle.addEventListener('click', function(e) {
        e.preventDefault();
        e.stopPropagation();
        if (!subsection) return;
        var willOpen = !subsection.classList.contains('open');
        subsection.classList.toggle('open', willOpen);
        toggle.setAttribute('aria-expanded', String(willOpen));
      });
    });

    // Third-level sub-subsection toggles
    var subSubToggles = document.querySelectorAll('.sidebar-subsubsection-toggle');
    subSubToggles.forEach(function(toggle) {
      var subsubsection = toggle.closest('.sidebar-subsubsection');

      toggle.addEventListener('click', function(e) {
        e.preventDefault();
        e.stopPropagation();
        if (!subsubsection) return;
        var willOpen = !subsubsection.classList.contains('open');
        subsubsection.classList.toggle('open', willOpen);
        toggle.setAttribute('aria-expanded', String(willOpen));
      });
    });

    var l4Toggles = document.querySelectorAll('.sidebar-l4section-toggle');
    l4Toggles.forEach(function(toggle) {
      var l4section = toggle.closest('.sidebar-l4section');

      toggle.addEventListener('click', function(e) {
        e.preventDefault();
        e.stopPropagation();
        if (!l4section) return;
        var willOpen = !l4section.classList.contains('open');
        l4section.classList.toggle('open', willOpen);
        toggle.setAttribute('aria-expanded', String(willOpen));
      });
    });
  }

  /* ── 5. Active TOC on Scroll ────────────────────────────── */
  function initTOC() {
    var tocLinks = document.querySelectorAll('.docs-toc #TableOfContents a');
    if (!tocLinks.length) return;

    var headings = [];
    tocLinks.forEach(function (link) {
      var id = link.getAttribute('href');
      if (id && id.startsWith('#')) {
        var el = document.getElementById(id.slice(1));
        if (el) headings.push(el);
      }
    });

    if (!headings.length) return;

    function setActive(id) {
      tocLinks.forEach(function (link) {
        var isActive = link.getAttribute('href') === '#' + id;
        link.classList.toggle('active', isActive);
      });
    }

    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (entry.isIntersecting) {
          setActive(entry.target.id);
        }
      });
    }, {
      rootMargin: '-' + (parseInt(getComputedStyle(document.documentElement)
        .getPropertyValue('--navbar-height') || '64') + 16) + 'px 0px -70% 0px',
      threshold: 0
    });

    headings.forEach(function (h) { observer.observe(h); });
  }

  /* ── 6. Mobile Navbar Menu ──────────────────────────────── */
  function initMobileMenu() {
    var toggle = document.getElementById('mobile-menu-toggle');
    var menu = document.getElementById('mobile-menu');
    var navbar = document.querySelector('.navbar');

    if (!toggle || !menu || !navbar) return;

    toggle.addEventListener('click', function () {
      var isOpen = navbar.classList.contains('mobile-open');
      navbar.classList.toggle('mobile-open', !isOpen);
      toggle.setAttribute('aria-expanded', String(!isOpen));
      menu.setAttribute('aria-hidden', String(isOpen));

      // Animate hamburger bars
      var bars = toggle.querySelectorAll('.hamburger-bar');
      if (!isOpen) {
        // Opening: X transform
        if (bars[0]) bars[0].style.transform = 'translateY(7px) rotate(45deg)';
        if (bars[1]) bars[1].style.opacity = '0';
        if (bars[2]) bars[2].style.transform = 'translateY(-7px) rotate(-45deg)';
      } else {
        // Closing: reset
        if (bars[0]) bars[0].style.transform = '';
        if (bars[1]) bars[1].style.opacity = '';
        if (bars[2]) bars[2].style.transform = '';
      }
    });

    // Close when clicking a link
    menu.querySelectorAll('a').forEach(function (link) {
      link.addEventListener('click', function () {
        navbar.classList.remove('mobile-open');
        toggle.setAttribute('aria-expanded', 'false');
        menu.setAttribute('aria-hidden', 'true');
        var bars = toggle.querySelectorAll('.hamburger-bar');
        bars.forEach(function (bar) {
          bar.style.transform = '';
          bar.style.opacity = '';
        });
      });
    });
  }

  /* ── 7. Hero Code Tabs ──────────────────────────────────── */
  function initHeroTabs() {
    var tabs = document.querySelectorAll('.code-tab');
    if (!tabs.length) return;

    tabs.forEach(function (tab) {
      tab.addEventListener('click', function () {
        var targetId = tab.getAttribute('data-target');
        if (!targetId) return;

        // Deactivate all tabs and panels
        tabs.forEach(function (t) { t.classList.remove('active'); });
        document.querySelectorAll('.code-panel').forEach(function (p) {
          p.classList.remove('active');
        });

        // Activate clicked tab and corresponding panel
        tab.classList.add('active');
        var panel = document.getElementById(targetId);
        if (panel) panel.classList.add('active');
      });
    });
  }

  /* ── 8. Smooth scroll for anchor links ─────────────────── */
  function initSmoothScroll() {
    document.querySelectorAll('a[href^="#"]').forEach(function (anchor) {
      anchor.addEventListener('click', function (e) {
        var id = this.getAttribute('href').slice(1);
        if (!id) return;
        var target = document.getElementById(id);
        if (!target) return;
        e.preventDefault();
        var navHeight = parseInt(
          getComputedStyle(document.documentElement)
            .getPropertyValue('--navbar-height') || '64'
        );
        var top = target.getBoundingClientRect().top + window.scrollY - navHeight - 16;
        window.scrollTo({ top: top, behavior: 'smooth' });
        // Update URL without scroll jump
        history.pushState(null, '', '#' + id);
      });
    });
  }

  /* ── 9. YAML show-more toggle ───────────────────────────── */
  function initYamlToggle() {
    var btn = document.getElementById('yaml-toggle');
    var section = document.getElementById('yaml-expand');
    if (!btn || !section) return;

    btn.addEventListener('click', function () {
      var expanded = btn.getAttribute('aria-expanded') === 'true';
      if (expanded) {
        section.hidden = true;
        btn.setAttribute('aria-expanded', 'false');
        btn.querySelector('span') && (btn.querySelector('span').textContent = 'Show more — CRD 2: application with dependsOn');
      } else {
        section.hidden = false;
        btn.setAttribute('aria-expanded', 'true');
        btn.querySelector('span') && (btn.querySelector('span').textContent = 'Show less');
      }
    });
  }

  /* ── 10. Scroll sidebar to active item ─────────────────────── */
  function scrollSidebarToActive() {
    var sidebar = document.getElementById('docs-sidebar');
    if (!sidebar) return;
    var active = sidebar.querySelector('.sidebar-link.active, .sidebar-standalone-link.active');
    if (!active) return;
    var sidebarRect = sidebar.getBoundingClientRect();
    var activeRect = active.getBoundingClientRect();
    var targetScrollTop = sidebar.scrollTop + (activeRect.top - sidebarRect.top) - (sidebarRect.height / 2) + (activeRect.height / 2);
    sidebar.scrollTop = Math.max(0, targetScrollTop);
  }

  /* ── 11. Init all ─────────────────────────────────────────── */
  document.addEventListener('DOMContentLoaded', function () {
    // DOMContentLoaded fires before the first paint, so removing immediately
    // still lets transitions fire on the first render. Double rAF defers the
    // removal until after the browser has committed the first frame.
    requestAnimationFrame(function() {
      requestAnimationFrame(function() {
        document.documentElement.classList.remove('no-transition');
      });
    });

    addCopyButtons();
    addLangLabels();
    initSidebar();
    initSidebarSections();
    scrollSidebarToActive();
    initTOC();
    initMobileMenu();
    initHeroTabs();
    initSmoothScroll();
    initYamlToggle();
  });

})();
