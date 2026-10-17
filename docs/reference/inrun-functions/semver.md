# Semver Notes

Parse, compare, and manipulate semantic version strings in Catalog templates. Useful for conditional logic based on operator image versions, library versions in CR specs, or version fields propagated through status.

All notes return a safe zero value (`""`, `0`, `false`) for invalid input — never an error.

## Reference

| Note | Description |
|------|-------------|
| `semverMajor` | Extract a single version component from a semver string. |
| `semverMinor` | Extract a single version component from a semver string. |
| `semverPatch` | Extract a single version component from a semver string. |
| `semverValid` | Return `true` when the string is a valid semantic version. |
| `semverCompare` | Compare two semver strings. |
| `semverBump` | Increment a version component (`"major"`, `"minor"`, or `"patch"`) and return the new version string. |
| `semverConstraint` | Return `true` when a version satisfies a constraint expression. |

## Examples

```yaml
# semverMajor
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

# semverMinor
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

# semverPatch
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

# semverValid
# Validation rule — deny CR if version is not a valid semver
spec:
  crds:
    myApp:
      validate:
        - message: "spec.version must be a valid semantic version (e.g. v1.2.3)"
          deny:
            - field: "{{ semverValid .spec.version }}"
              equals: "false"

# semverCompare
# Gate upgrade resource on version bump
when:
  - field: "{{ semverCompare .spec.version .status.runningVersion }}"
    equals: "1"   # spec.version is newer than running version

# semverBump
# Derive the next patch version for an upgrade annotation
metadata:
  annotations:
    myorg.io/next-version: "{{ semverBump .spec.version \"patch\" }}"
    # "1.2.3" → "1.2.4"
    # "2.1.0-rc1" → "2.1.1"  (prerelease dropped, patch bumped)

# semverConstraint
# Only create the legacy-compat resource for v1.x operators
when:
  - field: "{{ semverConstraint .spec.version \"^1.0\" }}"
    equals: "true"

# Gate on a range
when:
  - field: "{{ semverConstraint .spec.version \">=1.2.0,<2.0.0\" }}"
    equals: "true"
```
