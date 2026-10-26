#!/usr/bin/env bash
# sync-docs.sh — Copy ../docs/ into content/docs/, injecting Hugo front matter
# and converting MkDocs admonitions (!!!  note/warning/etc.) to callout shortcodes.
# Performs a clean sync: destination directories are wiped before each run so stale
# files from deleted source pages never survive.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SRC_DIR="$(realpath "$SCRIPT_DIR/../../docs")"
DST_DIR="$SCRIPT_DIR/../content/docs"
BLOG_DIR="$SCRIPT_DIR/../content/blog"

if [[ ! -d "$SRC_DIR" ]]; then
  echo "ERROR: docs source not found at $SRC_DIR" >&2
  exit 1
fi

# ── Helpers ──────────────────────────────────────────────────────────────────

inject_frontmatter() {
  local file="$1"
  local title="$2"
  local weight="$3"
  local src_file="${4:-}"

  # Skip if front matter already present
  if head -1 "$file" | grep -q '^---'; then
    return
  fi

  # Derive title from first H1 heading if present, then strip it from content.
  # The Hugo template renders the title as its own <h1> — keeping the markdown
  # H1 produces a duplicate heading on every page.
  local h1
  h1="$(grep -m1 '^# ' "$file" | sed 's/^# //' || true)"
  if [[ -n "$h1" ]]; then
    title="$h1"
    # Remove the first H1 line (and the blank line immediately after it, if any)
    sed -i '0,/^# /{/^# /d}' "$file"
    # Remove leading blank lines left behind
    sed -i '/./,$!d' "$file"
  fi

  # Get date from the source file's git history; fall back to today
  local date_str=""
  if [[ -n "$src_file" ]]; then
    date_str="$(git log --follow -1 --pretty=format:"%as" -- "$src_file" 2>/dev/null || true)"
  fi
  date_str="${date_str:-$(date +%Y-%m-%d)}"

  local tmp
  tmp="$(mktemp)"
  printf -- '---\ntitle: "%s"\ndate: %s\nweight: %s\n---\n\n' "$title" "$date_str" "$weight" > "$tmp"
  cat "$file" >> "$tmp"
  mv "$tmp" "$file"
}

slugify_title() {
  # Convert filename to Title Case, stripping leading numbers and hyphens
  basename "$1" .md \
    | sed 's/^[0-9]*[-_]*//' \
    | sed 's/[-_]/ /g' \
    | awk '{for(i=1;i<=NF;i++) $i=toupper(substr($i,1,1)) substr($i,2); print}'
}

convert_admonitions() {
  local file="$1"
  # MkDocs: !!! note "Optional title"
  #           body line (indented 4 spaces)
  # Hugo callout: {{< callout type="note" title="Optional title" >}} body {{< /callout >}}

  python3 - "$file" <<'PYEOF'
import sys, re

path = sys.argv[1]
with open(path, 'r') as f:
    text = f.read()

# Match admonition blocks: !!! type "optional title"\n(    indented lines)+
admonition_re = re.compile(
    r'^!!![ \t]+(\w+)(?:[ \t]+"([^"]*)")?[ \t]*\n((?:[ \t]{4}[^\n]*\n?)*)',
    re.MULTILINE
)

def replace_admonition(m):
    kind = m.group(1).lower()
    title = m.group(2) or ''
    body_raw = m.group(3)
    body = re.sub(r'^    ', '', body_raw, flags=re.MULTILINE).rstrip()
    title_attr = f' title="{title}"' if title else ''
    return f'{{{{< callout type="{kind}"{title_attr} >}}}}\n{body}\n{{{{< /callout >}}}}\n'

text = admonition_re.sub(replace_admonition, text)

with open(path, 'w') as f:
    f.write(text)
PYEOF
}

clean_navigation() {
  local file="$1"
  python3 - "$file" <<'PYEOF'
import sys, re

path = sys.argv[1]
with open(path, 'r') as f:
    text = f.read()

# Remove lines that start with "→ Next:" or "→ Previous:"
text = re.sub(r'^→ Next:.*\n?', '', text, flags=re.MULTILINE)
text = re.sub(r'^→ Previous:.*\n?', '', text, flags=re.MULTILINE)

# Remove lines where the entire line is "→ See [something](something)"
# (starts with "→ See [") — avoids removing inline mid-paragraph cross-refs
text = re.sub(r'^→ See \[.*\n?', '', text, flags=re.MULTILINE)

# Remove bare "## Next" sections (heading alone, not "## Next Steps" etc.)
text = re.sub(r'^## Next[ \t]*\n.*?(?=^##|\Z)', '', text, flags=re.MULTILINE | re.DOTALL)

# Remove entire "## See also" sections (heading + all lines until next ## or EOF)
text = re.sub(r'^## See also\b.*?(?=^##|\Z)', '', text, flags=re.MULTILINE | re.DOTALL)

# Strip .md from markdown link display text: [something.md](url) → [something](url)
text = re.sub(r'\[([^\]]+?)\.md\](\([^)]*\))', r'[\1]\2', text)

with open(path, 'w') as f:
    f.write(text)
PYEOF
}

rewrite_links() {
  local file="$1"
  # Rewrite markdown cross-references to Hugo URL format.
  #
  # Hugo serves leaf pages at /section/page/ and index pages at /section/.
  # Leaf pages have one extra URL path level vs the source file directory.
  # The correct relative links therefore differ by page type:
  #
  #   Source link       Index page result   Leaf page result
  #   ./sibling.md      sibling/            ../sibling/
  #   ../parent/x.md    ../parent/x/        ../../parent/x/
  #   bare-sibling.md   bare-sibling/       ../bare-sibling/
  #
  # Numeric prefixes (01-, 02-, ...) are stripped — they exist only for ordering.
  # Absolute URLs and fragment-only links (#anchor) are left untouched.

  local base
  base="$(basename "$file")"
  local is_index=false
  [[ "$base" == "_index.md" || "$base" == "index.md" ]] && is_index=true

  python3 - "$file" "$is_index" <<'PYEOF'
import re, sys

path = sys.argv[1]
is_index_page = (sys.argv[2] == 'true')
content = open(path).read()

def rewrite_url(url):
    if not url:
        return url
    # Leave absolute URLs (any scheme), root-relative, and fragment-only links alone
    if '://' in url or url.startswith('/') or url.startswith('#'):
        return url
    # Skip prose in parentheses (contains spaces or newlines)
    if ' ' in url or '\n' in url:
        return url

    # Split off fragment
    anchor = ''
    if '#' in url:
        url, frag = url.split('#', 1)
        anchor = '#' + frag
        if not url:
            return anchor  # pure #anchor

    # Determine relative prefix and the path portion after it
    if url.startswith('./'):
        levels = 0   # 0 = same dir  (represented as ./)
        has_dot_slash = True
        rest = url[2:]
    elif url.startswith('../'):
        rest = url
        levels = 0
        has_dot_slash = False
        while rest.startswith('../'):
            levels += 1
            rest = rest[3:]
    else:
        levels = -1  # bare link — no explicit prefix
        has_dot_slash = False
        rest = url
        # Only treat as a path if it looks like one: ends with .md or contains /.
        # Bare words like (IPC), (default), (metrics.*), (1) are prose, not links.
        if not (rest.endswith('.md') or '/' in rest):
            return rest + anchor

    # Normalise index.md → trailing slash, strip .md extension
    if rest == 'index.md':
        rest = ''
    elif rest.endswith('/index.md'):
        rest = rest[:-len('index.md')]  # keep trailing slash from dir/
    elif rest.endswith('.md'):
        rest = rest[:-3] + '/'

    # Strip numeric ordering prefix from every path segment
    rest = re.sub(r'(?:^|(?<=[/]))\d+-(?=[a-zA-Z])', '', rest)

    # Build the correct relative prefix for the target Hugo URL model.
    # Index pages live at /section/ — same depth as the source directory.
    # Leaf pages live at /section/page/ — one level deeper than source directory.
    if is_index_page:
        if levels == -1:          # bare → bare (index page resolves correctly)
            prefix = ''
        elif has_dot_slash:       # ./ → strip it (same dir in both models)
            prefix = ''
        else:                     # N×../ → same N×../
            prefix = '../' * levels
    else:
        if levels == -1:          # bare sibling → ../
            prefix = '../'
        elif has_dot_slash:       # ./ → ../  (same-dir in FS = one up in URL)
            prefix = '../'
        else:                     # N×../ → (N+1)×../
            prefix = '../' * (levels + 1)

    return prefix + rest + anchor

def process(m):
    new_url = rewrite_url(m.group(1))
    return '(' + new_url + ')'

content = re.sub(r'\(([^)]+)\)', process, content)
open(path, 'w').write(content)
PYEOF
}

sync_file() {
  local src_file="$1"
  local dst_file="$2"
  local weight="$3"
  local src_for_date="${4:-$src_file}"

  mkdir -p "$(dirname "$dst_file")"
  cp "$src_file" "$dst_file"
  rewrite_links "$dst_file"
  clean_navigation "$dst_file"
  convert_admonitions "$dst_file"
  local title
  title="$(slugify_title "$src_file")"
  inject_frontmatter "$dst_file" "$title" "$weight" "$src_for_date"
}

# ── Clean destinations ────────────────────────────────────────────────────────
# Wipe and recreate docs so stale files from deleted pages are removed.
# For the blog, preserve the manually maintained _index.md.

echo "Cleaning content/docs ..."
rm -rf "${DST_DIR:?}"
mkdir -p "$DST_DIR"

# Only clear the blog when there is a source to sync it from.
if [[ -d "$SRC_DIR/blog" ]]; then
  echo "Cleaning content/blog (preserving _index.md) ..."
  find "${BLOG_DIR:?}" -name '*.md' ! -name '_index.md' -delete 2>/dev/null || true
fi

# ── Sync docs ─────────────────────────────────────────────────────────────────

echo "Syncing docs: $SRC_DIR → $DST_DIR"

weight=1
find "$SRC_DIR" -name '*.md' | sort | while read -r src_file; do
  rel_path="${src_file#$SRC_DIR/}"

  # The blog is synced separately below
  if [[ "$rel_path" == blog/* ]]; then
    continue
  fi

  # Strip numeric ordering prefix (NN-) from each path segment so Hugo generates
  # clean URLs. The prefix is only for file-system ordering in the source tree.
  dst_rel_path="$(echo "$rel_path" | sed 's|/[0-9][0-9]*-|/|g' | sed 's|^[0-9][0-9]*-||')"
  dst_file="$DST_DIR/$dst_rel_path"

  # Hugo requires _index.md (not index.md) for section branch bundles.
  if [[ "$(basename "$dst_file")" == "index.md" ]]; then
    dst_file="${dst_file%index.md}_index.md"
  fi

  sync_file "$src_file" "$dst_file" "$weight" "$src_file"
  ((weight++)) || true
  echo "  docs: $rel_path"
done

# ── Auto-create missing section _index.md files ───────────────────────────────

find "$DST_DIR" -mindepth 1 -maxdepth 4 -type d | while read -r dir; do
  index="$dir/_index.md"
  if [[ ! -f "$index" ]]; then
    section_title="$(basename "$dir" | sed 's/[-_]/ /g' | awk '{for(i=1;i<=NF;i++) $i=toupper(substr($i,1,1)) substr($i,2); print}')"
    printf -- '---\ntitle: "%s"\nweight: 1\n---\n' "$section_title" > "$index"
    echo "  created index: ${index#$DST_DIR/}"
  fi
done

# ── Sync blog ─────────────────────────────────────────────────────────────────

if [[ -d "$SRC_DIR/blog" ]]; then
echo "Syncing blog: $SRC_DIR/blog → $BLOG_DIR"

weight=1
find "$SRC_DIR/blog" -name '*.md' 2>/dev/null | sort | while read -r src_file; do
  dst_file="$BLOG_DIR/$(basename "$src_file" | sed 's/^[0-9][0-9]*-//')"
  sync_file "$src_file" "$dst_file" "$weight" "$src_file"
  ((weight++)) || true
  echo "  blog: $(basename "$src_file")"
done
fi

# ── Generate changelog pages ──────────────────────────────────────────────────
# Reads CHANGELOG.md from the repo root, splits on ## v* headers, and writes
# one page per release to content/docs/changelog/. Also writes _index.md.

CHANGELOG_SRC="$(realpath "$SCRIPT_DIR/../../CHANGELOG.md")"
CHANGELOG_DST="$DST_DIR/changelog"

if [[ -f "$CHANGELOG_SRC" ]]; then
  echo "Generating changelog pages: $CHANGELOG_SRC → $CHANGELOG_DST"
  rm -rf "$CHANGELOG_DST"
  mkdir -p "$CHANGELOG_DST"

  python3 - "$CHANGELOG_SRC" "$CHANGELOG_DST" <<'PYEOF'
import sys, re, os

src = sys.argv[1]
dst_dir = sys.argv[2]

with open(src) as f:
    content = f.read()

# Split on lines starting with "## v"
sections = re.split(r'(?=^## v)', content, flags=re.MULTILINE)
sections = [s.strip() for s in sections if s.strip().startswith('## v')]

index_entries = []

for i, section in enumerate(sections):
    first_line = section.splitlines()[0]  # e.g. "## v0.7.10 [UNRELEASED] — ..."
    # Extract version and headline
    m = re.match(r'^## (v[\d.]+)\s*(?:\[UNRELEASED\])?\s*(?:—\s*(.+))?', first_line)
    if not m:
        continue
    version = m.group(1)          # v0.7.10
    headline = (m.group(2) or '').strip()

    # Strip the ## heading line from the body
    body = '\n'.join(section.splitlines()[1:]).strip()

    title = version
    if headline:
        title = f'{version} — {headline}'

    # Weight: earlier in file = higher version = lower weight number (newest first)
    weight = i + 1

    date_line = ''
    # Try to pull a date from the section (not critical)

    filename = f'{version}.md'
    filepath = os.path.join(dst_dir, filename)

    with open(filepath, 'w') as out:
        out.write(f'---\ntitle: "{title}"\nlinkTitle: "{version}"\nweight: {weight}\n---\n\n')
        out.write(body + '\n')

    index_entries.append((version, headline, weight))
    print(f'  changelog: {filename}')

# Write _index.md
index_path = os.path.join(dst_dir, '_index.md')
with open(index_path, 'w') as out:
    out.write('---\ntitle: "Changelog"\nweight: 1\n---\n\n')
    for version, headline, _ in index_entries:
        if headline:
            out.write(f'- [{version}]({version}/) — {headline}\n')
        else:
            out.write(f'- [{version}]({version}/)\n')

print(f'  changelog: _index.md ({len(index_entries)} releases)')
PYEOF
fi

echo "Done."
