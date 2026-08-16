# Porting conventions (spine-libgdx 4.2 → Go)

This project is a faithful port of the official
[spine-libgdx](https://github.com/EsotericSoftware/spine-runtimes) runtime,
branch `4.2`, to Go. spine-c may be consulted when the Java is ambiguous.

## Layout

- `core/` — renderer-agnostic runtime, single Go package `spine`
  (Java's `spine`, `spine.attachments`, `spine.utils` packages are merged into
  one Go package because Go forbids the import cycles the Java packages have).
- `ebiten/` — Ebitengine rendering layer, package `spineebiten`.

## Type mapping

| Java | Go |
|---|---|
| `float` | `float32` — **never** `float64` in runtime state. Intermediate math via `math.*` casts back to `float32` exactly like Java's `(float)Math.xxx(...)`. |
| `Array<T>` | `[]*T` (or `[]T` for interfaces) |
| `FloatArray` | `[]float32` (grow with append; `x.setSize(n)` → `ensureSize(&buf, n)` helper in skeleton.go) |
| `IntArray` | `[]int` |
| `short[]` (triangles/edges) | `[]uint16` |
| `int[]` (vertex bones) | `[]int32` |
| inheritance | struct embedding + interfaces |
| `null` | `nil` pointer / empty string for names |
| enum | `type Foo int` + `const (FooBar Foo = iota ...)` — value names are `Foo` + UpperCamel of the Java constant (e.g. `Inherit.noScale` → `InheritNoScale`, `MixBlend.first` → `MixBlendFirst`) |
| `Pool<T>` | plain free-list slice, not `sync.Pool` |

## Naming

- Java package-private/private/public **fields** become **exported Go fields**
  whenever the Java accessors are trivial (`getX`/`setX` pairs). Fields whose
  setters have side effects keep an unexported field plus methods
  (e.g. `Slot.Attachment()`/`SetAttachment`, `MeshAttachment.Region()`/`SetRegion`).
- Java methods → same name, exported: `updateWorldTransform()` →
  `UpdateWorldTransform()`. Overloads get suffixes: `updateWorldTransform(x,…)`
  → `UpdateWorldTransformWith(x,…)`, `getBounds(...clipper)` stays one method
  with a nillable parameter.
- `toString` → `String() string` (omit when a field named `String` exists).
- Static methods on a class → package-level functions (`IkConstraint.apply`
  → `ApplyIk1`, `ApplyIk2`).

## Error handling

- Loaders (`SkeletonJson`, `SkeletonBinary`, atlas parser, attachment loaders)
  return `error` (`fmt.Errorf("spine: ...")`) instead of throwing.
- Runtime methods panic only for programmer errors where Java throws
  `IllegalArgumentException` (nil arguments etc).

## Math helpers (core/mathutils.go, unexported)

`piF, pi2, invPI2, radDeg, degRad`; `cos, sin, cosDeg, sinDeg, atan2,
atan2Deg, sqrt, pow, abs, maxF, minF, clamp, signum, isNaN32, ceilInt,
floorInt`. All take/return `float32`.

## Fidelity rules

- Port **verbatim**: same operation order, same epsilons, same branch
  structure. Do not simplify or "clean up" algorithms — `Skeleton.updateCache`,
  `AnimationState.apply`, curve evaluation and constraint solvers must match
  the reference numerically (float32 epsilon).
- Keep the deferred event queue semantics of `AnimationState` (events fire on
  drain, not during apply).
- Per-frame allocation is the enemy: reuse buffers held by the owning object,
  as the Java code does with its scratch `FloatArray`s.

## Licensing

Derived from the Spine Runtimes; see LICENSE (Spine Runtimes License
Agreement). Users must have a Spine Editor license.
