# D20. The crate hayro is ported onto the display list

Status: Decided. Amends D14. Amends D16.

D14 and D16 said that ivy does not port the crate `hayro`, and that the
device of the display list takes its place. The crate `hayro` is the renderer of
hayro. It implements the `Device` of hayro-interpret and draws with
`vello_cpu`.

The device of the display list cannot take the calls of hayro-interpret by
itself. Those calls carry soft masks, shadings, tiling patterns, glyphs of
Type 3 and image masks. Code must turn each call into the commands of the
display list. In hayro, the crate `hayro` does this work, in about 2,400
lines of Rust.

So `pdf/internal/render` ports the crate `hayro`, file by file, as the other
crates of hayro are ported. Where upstream calls `vello_cpu`, the port calls
the `Device` of `internal/display`, and the device of okage draws. The port
does not hint glyphs (D19).

`internal/display` grows the commands that PDF needs: groups with a blend
mode and a soft mask, images that a transform places, image masks, and
shaders for patterns and shadings. `xps` uses the same commands.
