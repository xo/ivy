# kurboext

The package `kurboext` holds the items of the Rust crate kurbo that the port
of hayro needs, and that the subset of kurbo in okage does not hold (D16).
The package `github.com/xo/okage/kurbo` holds the rest. The items follow
kurbo 0.13, which okage ports. No source of kurbo is in the checkout of
ivy, so each item follows the behavior of kurbo 0.13 as its doc comment
states it.

kurbo is under the MIT license or the Apache-2.0 license. `LICENSE` is a
copy of the file `LICENSE` of `github.com/xo/okage/kurbo`, which holds the
MIT license of kurbo. Each file opens with the notice of kurbo, "Copyright
2018 the Kurbo Authors".

The items of `bezpath.go` came from `internal/hayro/pdffont/kurbo_extra.go`,
which held them for hayro-font. That file no longer exists.

## The items

| kurbo item | Go | Used by |
| --- | --- | --- |
| `BezPath` | `BezPath` | hayro-font, hayro-interpret |
| `BezPath::new`, `BezPath::from_vec` | `NewBezPath`, `BezPathFromElements` | hayro-font, hayro-interpret |
| `BezPath::move_to`, `line_to`, `quad_to`, `curve_to`, `close_path`, `push` | the methods of the same names | hayro-font, hayro-interpret |
| `impl Extend<PathEl> for BezPath` | `BezPath.Extend` | hayro-interpret |
| `BezPath::truncate`, `elements`, `is_empty`, `segments` | `BezPath.Truncate`, `Elements`, `IsEmpty`, `Segments` | hayro-font, hayro-interpret |
| `BezPath::elements().len()` | `BezPath.Len` | hayro-font, hayro-interpret |
| the derived `Clone` of `BezPath` | `BezPath.Clone` | hayro-interpret |
| `segments().next().is_some()` | `BezPath.HasSegments` | hayro-interpret |
| `impl Shape for BezPath`: `bounding_box` | `BezPath.BoundingBox` | hayro-interpret |
| `impl Mul<BezPath> for Affine` | `MulBezPath` | hayro-font, hayro-interpret |
| `ParamCurveExtrema::bounding_box` for `PathSeg` | `PathSegBoundingBox` | for the package |
| `ParamCurveExtrema::extrema` for `CubicBez` | `CubicBezExtrema` | for the package |
| `ParamCurveExtrema::bounding_box` for `CubicBez` | `CubicBezBoundingBox` | for the package |
| `Rect::ZERO` | `RectZero` | hayro-interpret |
| `Rect::min_x`, `max_x`, `min_y`, `max_y` | `RectMinX`, `RectMaxX`, `RectMinY`, `RectMaxY` | hayro-interpret |
| `Rect::area`, `is_zero_area` | `RectArea`, `RectIsZeroArea` | hayro-interpret |
| `Rect::intersect`, `overlaps` | `RectIntersect`, `RectOverlaps` | hayro-interpret |
| `Rect::contains`, `contains_rect` | `RectContains`, `RectContainsRect` | hayro-interpret, hayro |
| `Rect::inflate`, `expand` | `RectInflate`, `RectExpand` | hayro-interpret, hayro |
| `impl Shape for Rect`: `to_path` | `RectToPath` | hayro-interpret |
| `Cap` | `Cap`, `CapButt`, `CapSquare`, `CapRound` | hayro-interpret |
| `Join` | `Join`, `JoinBevel`, `JoinMiter`, `JoinRound` | hayro-interpret |

Go cannot add a method to a type of another package, so the items of
`Rect` are functions that take the rectangle first.

## The differences from kurbo

1. `BezPath.Segments` returns a slice, through `kurbo.Segments` of okage,
   where kurbo returns an iterator.
2. `BezPath.Clone` is explicit, because a Go slice shares its memory where
   the `Clone` of Rust copies it.
3. The helpers `rustMin` and `rustMax` give `f64::min` and `f64::max` of
   Rust, which ignore a NaN.
