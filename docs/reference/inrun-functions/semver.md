# Semver

Work with semantic versions ("1.2.3", "v1.2.3-rc.1"): read their parts, compare them, bump them, and check them against constraints such as ">= 1.20". A version that does not parse is an error.

## Reference

| Function | Description |
|----------|-------------|
| [`semverBump`](#semverbump) | Increments component ("major", "minor" or "patch") of version v, resetting the lower components and dropping prerelease and build metadata. |
| [`semverCompare`](#semvercompare) | Compares versions a and b: -1 when a is older, 0 when they are equal, 1 when a is newer. |
| [`semverConstraint`](#semverconstraint) | Reports whether version v satisfies constraint, written as "&gt;=1.2.0,&lt;2.0.0", "^2.0", "~1.2" or "1.x". |
| [`semverMajor`](#semvermajor) | Returns the major component of version v; an error when v is not a valid version. |
| [`semverMinor`](#semverminor) | Returns the minor component of version v; an error when v is not a valid version. |
| [`semverPatch`](#semverpatch) | Returns the patch component of version v; an error when v is not a valid version. |
| [`semverValid`](#semvervalid) | Reports whether v is a valid semantic version; a leading "v" is accepted. |

## Functions

### semverBump

`semverBump` increments component ("major", "minor" or "patch") of version v, resetting the lower components and dropping prerelease and build metadata.

```text
{{ semverBump "1.2.3" "minor" }}  → 1.3.0
```

### semverCompare

`semverCompare` compares versions a and b: -1 when a is older, 0 when they are equal, 1 when a is newer. An error when either is not a valid version.

```text
{{ semverCompare .spec.version .status.runningVersion }}  → 1
```

See also: [`semverConstraint`](#semverconstraint)

### semverConstraint

`semverConstraint` reports whether version v satisfies constraint, written as "&gt;=1.2.0,&lt;2.0.0", "^2.0", "~1.2" or "1.x". An error when either does not parse.

```text
{{ semverConstraint .spec.version "^1.0" }}  → true
```

See also: [`semverCompare`](#semvercompare)

### semverMajor

`semverMajor` returns the major component of version v; an error when v is not a valid version.

```text
{{ semverMajor "1.2.3" }}  → 1
```

See also: [`semverMinor`](#semverminor), [`semverPatch`](#semverpatch)

### semverMinor

`semverMinor` returns the minor component of version v; an error when v is not a valid version.

```text
{{ semverMinor "1.2.3" }}  → 2
```

See also: [`semverMajor`](#semvermajor), [`semverPatch`](#semverpatch)

### semverPatch

`semverPatch` returns the patch component of version v; an error when v is not a valid version.

```text
{{ semverPatch "1.2.3" }}  → 3
```

See also: [`semverMajor`](#semvermajor), [`semverMinor`](#semverminor)

### semverValid

`semverValid` reports whether v is a valid semantic version; a leading "v" is accepted.

```text
{{ semverValid .spec.version }}  → true
```
