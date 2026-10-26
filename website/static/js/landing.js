/* ── Scroll reveal ───────────────────────────────────────────── */
(function () {
  if (!window.IntersectionObserver) return;
  const observer = new IntersectionObserver(
    (entries) => {
      entries.forEach((e) => {
        if (e.isIntersecting) {
          e.target.classList.add('revealed');
          observer.unobserve(e.target);
        }
      });
    },
    { threshold: 0.08, rootMargin: '0px 0px -50px 0px' }
  );
  document.querySelectorAll('[data-reveal]').forEach((el) => observer.observe(el));
})();

/* ── Quick Start ─────────────────────────────────────────────── */
const S = { os: 'mac', mt: 'brew' };

function qsContent(os, mt) {

  if (mt === 'helm') return `<pre><span class="c"># Add the Inrun Helm repo</span>
<span class="pr">$ </span><span class="kw">helm repo add</span> <span class="s">inrun https://intentd.github.io/inrun</span>
<span class="pr">$ </span><span class="kw">helm repo update</span>

<span class="c"># Runtime, gateway and console in your cluster</span>
<span class="pr">$ </span><span class="kw">helm upgrade --install</span> <span class="s">inrun inrun/inrun \\
    --namespace inrun-system \\
    --create-namespace</span>

<span class="c"># Verify</span>
<span class="pr">$ </span><span class="kw">kubectl get pods</span> <span class="s">-n inrun-system</span></pre>`;

  const install = mt === 'brew'
    ? `<span class="c"># Install the inrun CLI and the console</span>
<span class="pr">$ </span><span class="kw">brew install</span> <span class="s">intentd/tap/inrun intentd/tap/inrun-console</span>`
    : `<span class="c"># Install the inrun CLI and the console</span>
<span class="pr">$ </span><span class="kw">curl -sSL</span> <span class="s">https://get.inrun.dev</span> | bash`;

  return `<pre>${install}

<span class="c"># Scaffold a Blueprint, a CRD and a CR</span>
<span class="pr">$ </span><span class="kw">inrun init</span> <span class="s">my-app</span> && cd my-app

<span class="c"># Check it with no cluster</span>
<span class="pr">$ </span><span class="kw">inrun simulate</span>

<span class="c"># Run it (--dev creates a kind cluster if none is reachable)</span>
<span class="pr">$ </span><span class="kw">inrun</span> <span class="s">--dev --apply-cr</span></pre>`;
}

function qs(key, val, el) {
  el.closest('.chips').querySelectorAll('.chip').forEach((c) => c.classList.remove('on'));
  el.classList.add('on');

  if (key === 'os' && S.os !== val) {
    S.mt = val === 'mac' ? 'brew' : 'curl';
    document.getElementById('method-chips').querySelectorAll('.chip').forEach((c) => {
      c.classList.toggle('on', c.textContent.trim() === S.mt);
    });
  }

  S[key] = val;
  renderQS();
}

function renderQS() {
  document.getElementById('qs-out').innerHTML = qsContent(S.os, S.mt);
  document.getElementById('qs-label').textContent = S.os === 'mac' ? 'zsh' : 'bash';
  document.getElementById('qs-note').textContent = 'inrun init · inrun simulate · inrun --dev';
}

/* ── Code tabs ───────────────────────────────────────────────── */
function switchTab(panelId, btn) {
  const cw = btn.closest('.cw');
  cw.querySelectorAll('.cw-tab').forEach((t) => t.classList.remove('on'));
  cw.querySelectorAll('.cw-panel').forEach((p) => (p.hidden = true));
  btn.classList.add('on');
  cw.querySelector('#' + panelId).hidden = false;
}

/* ── Modal ───────────────────────────────────────────────────── */
function openIncModal() {
  const m = document.getElementById('incModal');
  m.classList.add('open');
  document.body.style.overflow = 'hidden';
}

function closeIncModal(e) {
  if (e && e.target !== document.getElementById('incModal') && !e.target.closest('.modal-close')) return;
  document.getElementById('incModal').classList.remove('open');
  document.body.style.overflow = '';
}

document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') closeIncModal({ target: document.getElementById('incModal') });
});

/* ── Copy button ─────────────────────────────────────────────── */
const COPY_ICON = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>`;
const CHECK_ICON = `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>`;

function copycmd(btn) {
  const text = btn.parentElement.querySelector('.cmd')?.textContent.trim() ?? '';
  navigator.clipboard?.writeText(text).then(() => {
    btn.innerHTML = CHECK_ICON;
    btn.style.color = 'var(--intent)';
    setTimeout(() => {
      btn.innerHTML = COPY_ICON;
      btn.style.color = '';
    }, 1500);
  });
}

/* ── Latest release ──────────────────────────────────────────── */
fetch('https://api.github.com/repos/intentd/inrun/releases/latest')
  .then((r) => r.ok ? r.json() : null)
  .then((d) => {
    if (d?.tag_name) {
      const el = document.getElementById('inrun-version');
      if (el) el.textContent = d.tag_name;
    }
  })
  .catch(() => {});

/* ── Back to top ─────────────────────────────────────────────── */
(function () {
  const btn = document.getElementById('back-to-top');
  if (!btn) return;
  window.addEventListener('scroll', () => {
    btn.classList.toggle('visible', window.scrollY > 400);
  }, { passive: true });
})();

/* ── Init ────────────────────────────────────────────────────── */
renderQS();
