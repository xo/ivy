# assets

`CGATS001Compat-v2-micro.icc` is the CMYK profile that ivy uses for CMYK
without a profile (D61). It is a compact profile of CGATS TR 001 (SWOP) from
[Compact-ICC-Profiles](https://github.com/saucecontrol/Compact-ICC-Profiles),
under CC0 1.0 Universal. `LICENSE.txt` is the text of CC0 1.0 Universal.

The file is the same file, byte for byte, as `assets/CGATS001Compat-v2-micro.icc`
of hayro-interpret and of hayro-jpeg2000 at the commit `61f50b42` of hayro.
The ports of the two crates, `internal/hayro/interpret` and
`internal/jpeg2000`, read it from here. Its SHA-256 is
`73e1ba37d2bad5bab2a964f40a9eed96209666efc067c3322626214bbef234a0`.
