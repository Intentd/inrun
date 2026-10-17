# 25 — Semver Notes

Parse, compare, and manipulate semantic version strings in Catalog templates. Useful for conditional logic based on operator image versions, library versions in CR specs, or version fields propagated through status.

All notes return a safe zero value (`""`, `0`, `false`) for invalid input — never an error.

## Reference

### `semverMajor` / `semverMinor` / `semverPatch`

Extract a single version component from a semver string. The leading `v` is stripped automatically.

Keywords: semver, version, major, minor, patch, component, parse, string

```yaml
# Extract components for display or comparison
status:
  fields:
    - path: majorVersion
      value: "{{ semverMajor .spec.version }}"   # "1.2.3" → "1"
    - path: minorVersion
      value: "{{ semverMinor .spec.version }}"   # "1.2.3" → "2"
    - path: patchVersion
      value: "{{ semverPatch .spec.version }}"   # "1.2.3" → "3"

# Gate a resource on the major version
when:
  - field: "{{ semverMajor .spec.version }}"
    equals: "2"
```

---

### `semverValid`

Return `true` when the string is a valid semantic version. Use in validation rules to reject malformed version fields before reconciliation begins.

Keywords: semver, version, valid, validate, boolean, parse, check

```yaml
# Validation rule — deny CR if version is not a valid semver
spec:
  crds:
    myApp:
      validate:
        - message: "spec.version must be a valid semantic version (e.g. v1.2.3)"
          deny:
            - field: "{{ semverValid .spec.version }}"
              equals: "false"
```

---

### `semverCompare`

Compare two semver strings. Returns `-1` when `a < b`, `0` when equal, `1` when `a > b`. Returns `0` for invalid input.

Keywords: semver, version, compare, comparison, order, int, greater, less

```yaml
# Gate upgrade resource on version bump
when:
  - field: "{{ semverCompare .spec.version .status.runningVersion }}"
    equals: "1"   # spec.version is newer than running version
```

---

### `semverBump`

Increment a version component (`"major"`, `"minor"`, or `"patch"`) and return the new version string. Any prerelease or build metadata is dropped. Returns the original string for invalid input or unknown component.

Keywords: semver, version, bump, increment, next, major, minor, patch, string

```yaml
# Derive the next patch version for an upgrade annotation
metadata:
  annotations:
    myorg.io/next-version: "{{ semverBump .spec.version \"patch\" }}"
    # "1.2.3" → "1.2.4"
    # "2.1.0-rc1" → "2.1.1"  (prerelease dropped, patch bumped)
```

---

### `semverConstraint`

Return `true` when a version satisfies a constraint expression. Uses Masterminds semver constraint syntax: `>=1.0.0`, `^2.0`, `~1.2`, `1.x`, comma for AND, `||` for OR.

Keywords: semver, version, constraint, range, compatible, boolean, check

```yaml
# Only create the legacy-compat resource for v1.x operators
when:
  - field: "{{ semverConstraint .spec.version \"^1.0\" }}"
    equals: "true"

# Gate on a range
when:
  - field: "{{ semverConstraint .spec.version \">=1.2.0,<2.0.0\" }}"
    equals: "true"
```

---

## Quick reference

| Note | Accepts | Returns | Notes |
|------|---------|---------|-------|
| `semverMajor` | `version string` | `string` | `"v"` prefix stripped |
| `semverMinor` | `version string` | `string` | `"v"` prefix stripped |
| `semverPatch` | `version string` | `string` | `"v"` prefix stripped |
| `semverValid` | `version string` | `bool` | safe for validation rules |
| `semverCompare` | `a, b string` | `int` | -1 / 0 / 1 |
| `semverBump` | `version, component string` | `string` | component: major / minor / patch |
| `semverConstraint` | `version, constraint string` | `bool` | Masterminds syntax |
