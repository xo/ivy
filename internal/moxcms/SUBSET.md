# The subset of moxcms

ivy ports a subset of moxcms 0.9.2, from the commit `1e2befc3` (D16). The
subset holds each item of moxcms that hayro calls, and each item that those
items need. Since D61 it also holds each item that the package `icc` of ivy
calls, and the items that they need. The column "Used by" names that
package as "package `icc`". moxcms is under the BSD-3-Clause license or the Apache-2.0
license. `LICENSE.md` and `LICENSE-APACHE.md` are its license files.

`icc.rs` and `lab.rs` mean the files `src/color/icc.rs` and
`src/color/lab.rs` of hayro-interpret. `integration.rs` and `tests/main.rs`
mean the files `src/integration.rs` and `tests/main.rs` of hayro-jpeg2000.
They are in hayro at the commit `61f50b42`. hayro asks for moxcms 0.8.1, and
it enables the feature `in_place` and the default features. The port follows
0.9.2, the base commit. Version 0.9.2 adapts the colorants of sRGB to the
PCS illuminant (0.9642, 1, 0.8249), where 0.8.1 adapts them to the D50 of
`Chromaticity`. So a sample of the port can differ by one from hayro with
0.8.1, as in the references of the tests of hayro-jpeg2000. A use that is
marked "for the subset" comes from another item of the subset, not from
hayro.

The port differs from upstream in these places. Each one has a comment that
starts with "Port:".

1. The port has the scalar code only. It has no path of AVX, SSE, NEON or
   SVE.
2. The port has the transforms of 8 bits only, so it has no type parameter
   for the type of a sample.
3. The port leaves out three transforms that hayro never builds, because
   hayro transforms to sRGB or to gray only: gray to an RGB profile with a
   different curve for each channel, RGB to gray, and RGB or Lab to CMYK.
   They return `UnsupportedProfileConnection`.
4. The port leaves out the features `options`, `extended_range` and
   `any_to_any`, which hayro does not enable. So the interpolation is
   always trilinear or quadlinear, and the weights are always those of
   `BarycentricWeightScale::Low`.
5. The reader skips the tags of text, the measurement, the viewing
   conditions, the technology and the calibration date. No transform uses
   them. Upstream reads them, so a profile with one of them malformed is an
   error in upstream and not in the port.
6. Where upstream panics on malformed data, the port returns an error or
   clamps an index. A CLUT whose last node is not whole in its array is
   `MalformedClut`.
7. The port computes the powers, roots, exponents and logarithms with the
   package math, where upstream uses the crate pxfm. The last bit of a
   result can differ.
8. `CmsError` is a struct with a kind, and it wraps `ivy.ErrFormat`,
   `ivy.ErrUnsupported` or `ivy.ErrLimit`.

The tables leave out the small helpers of the port that have no item in
moxcms, such as `checkLanes` and the functions of `math_mod.go` that give Go
the semantics of the arithmetic of Rust. Each helper names the code of
moxcms that it serves.

`cmyk.go` has no item in moxcms. It holds `CmykProfile`, the CMYK profile of
hayro-interpret and hayro-jpeg2000, which `assets/` holds with its license
(D61). The ports of the two crates and the package `icc` read it from there.

`testdata/golden.txt` holds the output of moxcms for 17 transforms. The
test `TestGolden` compares the port with it. On 2026-11-16 the two are
equal, byte for byte.

## transform.rs, in transform.go

| Item | Go | Used by |
| --- | --- | --- |
| `TransformExecutor<u8>` | `TransformExecutor`, `Transform` | `icc.rs`, `integration.rs`, `tests/main.rs` (`transform`) |
| `Transform8BitExecutor` | `Transform8BitExecutor` | `icc.rs` |
| `InPlaceTransformExecutor<u8>` | `InPlaceTransformExecutor`, `Transform` | `icc.rs` |
| `Stage`, `InPlaceStage` | `Stage`, `InPlaceStage` | for the subset: the stages of the LUTs |
| `BarycentricWeightScale::Low` | `BarycentricWeightScaleLow` | for the subset |
| `InterpolationMethod::Linear` | `InterpolationMethodLinear` | for the subset |
| `TransformOptions` | `TransformOptions` | `icc.rs`, `integration.rs`, `tests/main.rs`, package `icc` |
| `impl Default for TransformOptions` | `DefaultTransformOptions` | `icc.rs`, `integration.rs`, `tests/main.rs`, package `icc` |
| `Layout` | `Layout`, `LayoutRgb`, `LayoutRgba`, `LayoutGray`, `LayoutGrayAlpha`, `LayoutCmyka`, `LayoutInks5` to `LayoutInks15` | `icc.rs` (`Gray`, `Rgb`, `Rgba`), `integration.rs` (`Rgb`, `Rgba`), `tests/main.rs` (`Gray`, `GrayAlpha`, `Rgb`, `Rgba`), package `icc` (`Gray`, `Rgb`, `Rgba`) |
| `Layout::r_i`, `g_i`, `b_i`, `a_i` | `Layout.rI`, `gI`, `bI`, `aI` | for the subset |
| `Layout::has_alpha`, `Layout::channels` | `Layout.HasAlpha`, `Layout.Channels` | for the subset |
| `ColorProfile::is_matrix_shaper` | `ColorProfile.IsMatrixShaper` | package `icc` |
| `ColorProfile::create_transform_8bit` | `ColorProfile.CreateTransform8Bit` | `icc.rs`, `integration.rs`, `tests/main.rs`, package `icc` |
| `ColorProfile::create_in_place_transform_8bit` | `ColorProfile.CreateInPlaceTransform8Bit` | `icc.rs` |
| `ColorProfile::create_transform_nbit` | `ColorProfile.createTransformNbit` | for the subset |
| `ColorProfile::create_transform_in_place_nbit` | `ColorProfile.createTransformInPlaceNbit` | for the subset |
| `ColorProfile::get_device_to_pcs`, `get_pcs_to_device` | `ColorProfile.getDeviceToPcs`, `getPcsToDevice` | for the subset |
| `ColorProfile::device_to_pcs_is_lut16`, `pcs_to_device_is_lut16` | `ColorProfile.deviceToPcsIsLut16`, `pcsToDeviceIsLut16` | for the subset |

## profile.rs, in profile.go

| Item | Go | Used by |
| --- | --- | --- |
| `MAX_PROFILE_SIZE` | `maxProfileSize` | for the subset |
| `ProfileSignature` | `profileSignature` | for the subset |
| `ProfileVersion`, its `TryFrom<u32>` | `ProfileVersion`, `profileVersionFrom` | for the subset |
| `DataColorSpace`, its `TryFrom<u32>` | `DataColorSpace`, `DataColorSpaceXyz` to `DataColorSpaceColor15`, `dataColorSpaceFrom` | `icc.rs` (`Lab`), package `icc` |
| `DataColorSpace::check_layout`, `is_three_channels` | `DataColorSpace.CheckLayout`, `isThreeChannels` | for the subset |
| `ProfileClass`, its `TryFrom<u32>` | `ProfileClass`, `profileClassFrom` | for the subset |
| `LutStore` | `LutStore`, `LutStore8`, `LutStore16` | for the subset |
| `LutType`, its `TryFrom<u32>` | `LutType`, `lutTypeFrom` | for the subset |
| `LutWarehouse`, `LutDataType`, `LutMultidimensionalType` | `LutWarehouse`, `LutDataType`, `LutMultidimensionalType` | for the subset |
| `RenderingIntent`, its `TryFrom<u32>` | `RenderingIntent`, `renderingIntentFrom` | package `icc` (`RenderingIntent`) |
| `ProfileHeader`, `ProfileHeader::new_from_slice` | `profileHeader`, `newProfileHeaderFromSlice` | for the subset |
| `CicpProfile` | `CicpProfile` | package `icc` |
| `ColorProfile` | `ColorProfile` | `icc.rs` (`color_space`, `gray_trc`, `red_trc`), `lab.rs` (`white_point`), `integration.rs` |
| `impl Default for ColorProfile` | `defaultColorProfile` | for the subset |
| `ParsingOptions`, its `Default` | `ParsingOptions`, `DefaultParsingOptions` | for the subset |
| `ColorProfile::version` | `ColorProfile.Version` | for the subset |
| `ColorProfile::new_from_slice` | `NewFromSlice` | `icc.rs`, `lab.rs`, `integration.rs`, `tests/main.rs`, package `icc` |
| `ColorProfile::new_from_slice_with_options` | `NewFromSliceWithOptions`, `ColorProfile.readTags` | for the subset |
| `ColorProfile::colorant_matrix`, `colorants_matrix` | `ColorProfile.colorantMatrix`, `colorantsMatrix` | for the subset |
| `ColorProfile::update_chromatic_adaptation`, `update_colorants` | `ColorProfile.updateChromaticAdaptation`, `updateColorants` | for the subset |
| `ColorProfile::update_rgb_colorimetry`, `update_rgb_colorimetry_triplet`, `update_rgb_colorimetry_from_cicp` | `ColorProfile.updateRgbColorimetry`, `updateRgbColorimetryTriplet`, `updateRgbColorimetryFromCicp` | for the subset |
| `ColorProfile::rgb_to_xyz_d`, `rgb_to_xyz_matrix`, `transform_matrix` | `rgbToXyzD`, `ColorProfile.RgbToXyzMatrix`, `TransformMatrix` | for the subset |
| `ColorProfile::has_device_to_pcs_lut`, `has_pcs_to_device_lut` | `ColorProfile.hasDeviceToPcsLut`, `hasPcsToDeviceLut` | for the subset |

## reader.rs, in reader.go

| Item | Go | Used by |
| --- | --- | --- |
| `s15_fixed16_number_to_float`, `s15_fixed16_number_to_double` | `s15Fixed16NumberToFloat`, `s15Fixed16NumberToDouble` | for the subset |
| `uint16_number_to_float_fast`, `uint8_number_to_float_fast` | `uint16NumberToFloatFast`, `uint8NumberToFloatFast` | for the subset |
| `ColorProfile::read_lut_type`, `read_lut_table_f32`, `read_nested_tone_curves` | `readLutType`, `readLutTableF32`, `readNestedToneCurves` | for the subset |
| `ColorProfile::read_lut_abm_type`, `read_lut_a_to_b_type`, `read_lut_tag` | `readLutAbmType`, `readLutAToBType`, `readLutTag` | for the subset |
| `ColorProfile::read_trc_tag_s`, `read_trc_tag` | `readTrcTagS`, `readTrcTag` | for the subset |
| `ColorProfile::read_chad_tag`, `read_xyz_tag`, `read_cicp_tag` | `readChadTag`, `readXyzTag`, `readCicpTag` | for the subset |

## tag.rs, in tag.go

| Item | Go | Used by |
| --- | --- | --- |
| `TAG_SIZE` | `tagSize` | for the subset |
| `Tag`, the tags that the subset reads | `tag`, `tagRedXyz` and the others | for the subset |
| `TagTypeDefinition`, the types that the subset reads | `tagTypeDefinition`, `tagTypeMabLut` and the others | for the subset |

## trc.rs, in trc.go

| Item | Go | Used by |
| --- | --- | --- |
| `ToneReprCurve`, its `PartialEq` | `ToneReprCurve`, `ToneReprCurveKind`, `CurveLut`, `CurveParametric`, `ToneReprCurve.equal` | `icc.rs` (`gray_trc`, `red_trc`) |
| `curve_from_gamma` | `curveFromGamma` | for the subset |
| `build_trc_table` | `buildTrcTable` | for the subset |
| `ParametricCurve`, `new`, `is_linear`, `eval`, `invert` | `ParametricCurve`, `newParametricCurve`, `isLinear`, `Eval`, `invert` | for the subset |
| `u8_fixed_8number_to_float` | `u8Fixed8NumberToFloat` | for the subset |
| `passthrough_table`, `linear_forward_table`, `linear_lut_interpolate` | `passthroughTable`, `linearForwardTable`, `linearLutInterpolate` | for the subset |
| `lut_interp_linear_float`, `lut_interp_linear`, `lut_interp_linear16` | `lutInterpLinearFloat`, `lutInterpLinear`, `lutInterpLinear16` | for the subset |
| `linear_curve_parametric`, `linear_curve_parametric_s` | `linearCurveParametric`, `linearCurveParametricS` | for the subset |
| `make_gamma_linear_table`, `make_gamma_pow_table`, `make_gamma_parametric_table` | `makeGammaLinearTable`, `makeGammaPowTable`, `makeGammaParametricTable` | for the subset |
| `lut_interp_linear_gamma_impl`, `make_gamma_lut` | `lutInterpLinearGammaImpl`, `makeGammaLut` | for the subset |
| `compare_parametric`, `lut_inverse_interp16`, `invert_lut` | `compareParametric`, `lutInverseInterp16`, `invertLut` | for the subset |
| `ToneReprCurve::build_linearize_table`, `build_gamma_table` | `ToneReprCurve.buildLinearizeTable`, `buildGammaTable` | for the subset |
| `ColorProfile::build_gray_linearize_table`, `build_r_linearize_table`, `build_g_linearize_table`, `build_b_linearize_table`, `build_gamma_table` | `ColorProfile.buildGrayLinearizeTable`, `buildRLinearizeTable`, `buildGLinearizeTable`, `buildBLinearizeTable`, `buildGammaTable` | for the subset |
| `ColorProfile::are_all_trc_the_same`, `is_linear_matrix_shaper` | `ColorProfile.areAllTrcTheSame`, `isLinearMatrixShaper` | for the subset |

## gamma.rs, in gamma.go

| Item | Go | Used by |
| --- | --- | --- |
| The functions to and from linear of each transfer characteristic | `srgbToLinear`, `srgbFromLinear` and the others | for the subset |
| `make_linear_table_impl`, `make_gamma_table_impl` | `makeLinearTableImpl`, `makeGammaTableImpl` | for the subset |
| `TransferCharacteristics::linearize`, `gamma` | `TransferCharacteristics.Linearize`, `Gamma` | for the subset |
| `TransferCharacteristics::make_linear_table`, `make_gamma_table` | `TransferCharacteristics.makeLinearTable`, `makeGammaTable` | for the subset |

## cicp.rs, in cicp.go

| Item | Go | Used by |
| --- | --- | --- |
| `CicpColorPrimaries`, its `TryFrom<u8>` | `CicpColorPrimaries`, `CicpColorPrimariesFrom` | package `icc` |
| `CicpColorPrimaries::has_chromaticity`, `white_point` | `CicpColorPrimaries.hasChromaticity`, `whitePoint` | for the subset |
| `ColorPrimaries`, and its constants `BT_709`, `BT_470M`, `BT_470BG`, `BT_601`, `SMPTE_240`, `GENERIC_FILM`, `BT_2020`, `XYZ`, `DCI_P3`, `SMPTE_431`, `SMPTE_432` and `EBU_3213` | `ColorPrimaries`, `ColorPrimariesBt709`, `ColorPrimariesBt470M`, `ColorPrimariesBt470Bg`, `ColorPrimariesBt601`, `ColorPrimariesGenericFilm`, `ColorPrimariesBt2020`, `ColorPrimariesXyz`, `ColorPrimariesDciP3`, `ColorPrimariesEbu3213` | for the subset |
| `impl TryFrom<CicpColorPrimaries> for ColorPrimaries` | `colorPrimariesFrom` | for the subset |
| `TransferCharacteristics`, its `TryFrom<u8>`, `has_transfer_curve` | `TransferCharacteristics`, `TransferCharacteristicsFrom`, `hasTransferCurve` | package `icc` |
| `impl TryFrom<TransferCharacteristics> for ToneReprCurve` | `toneReprCurveFrom`, `numTrcTableEntries` | for the subset |
| `create_rec709_parametric` | `createRec709Parametric` | for the subset |
| `MatrixCoefficients`, its `TryFrom<u8>` | `MatrixCoefficients`, `MatrixCoefficientsFrom` | package `icc` |

## defaults.rs, in defaults.go

| Item | Go | Used by |
| --- | --- | --- |
| `white_point_from_temperature`, `white_point_d65` | `whitePointFromTemperature`, `whitePointD65` | for the subset |
| `srgb_colorants` | `srgbColorants` | for the subset |
| `ColorProfile::basic_rgb_profile` | `basicRgbProfile` | for the subset |
| `ColorProfile::new_from_cicp` | `NewFromCicp` | package `icc` |
| `ColorProfile::new_srgb` | `NewSrgb` | `icc.rs`, `integration.rs`, `tests/main.rs`, package `icc` |
| `ColorProfile::new_gray_with_gamma` | `NewGrayWithGamma` | `icc.rs` |

## matrix.rs, in matrix.go

| Item | Go | Used by |
| --- | --- | --- |
| `Vector3f`, `Vector3d`, `From<T>`, `PartialEq`, `cast`, `mla`, `neg_mla` | `Vector3f`, `Vector3d`, `vector3fFrom`, `Equal`, `toF32`, `mla`, `negMla` | for the subset |
| `Matrix3f`, `Matrix3d`, `IDENTITY`, `test_equality` | `Matrix3f`, `Matrix3d`, `Matrix3fIdentity`, `Matrix3dIdentity`, `TestEquality` | for the subset |
| `Matrix3d::determinant`, `inverse`, `mul_row_vector`, `mul_vector`, `mat_mul`, `mat_mul_const`, `to_f32` | `Matrix3d.Determinant`, `Inverse`, `mulRowVector`, `MulVector`, `MatMul`, `matMulConst`, `ToF32` | for the subset |
| `Xyz`, `Xyz::new`, `PartialEq`, `to_xyzd` | `Xyz`, `NewXyz`, `Equal`, `ToXyzd` | for the subset |
| `Xyzd`, `Xyzd::new` | `Xyzd`, `NewXyzd` | `lab.rs` |
| `Xyzd`, `PartialEq`, `to_vector_d`, `to_xyz` | `Xyzd.Equal`, `ToVectorD`, `ToXyz` | for the subset |

## The small files

| File | Item | Go | Used by |
| --- | --- | --- | --- |
| `lib.rs`, in `lib.go` | the crate | the package documentation | for the subset |
| `err.rs`, in `err.go` | `CmsError`, `MalformedSize` | `CmsError`, `CmsErrorKind` and its kinds, `MalformedSize` | `integration.rs` (`UnsupportedChannelConfiguration`) |
| `mlaf.rs`, in `mlaf.go` | `mlaf`, `neg_mlaf`, `fmla` | `mlaf`, `mlafF64`, `negMlaf`, `fmla`, `fmlaF64` | for the subset |
| `math/mod.rs`, in `math_mod.go` | `m_clamp` | `mClamp` | for the subset |
| `safe_math.rs`, in `safe_math.go` | `SafeAdd`, `SafeMul`, `SafePowi` | `safeAdd`, `safeMul`, `safeMulU32`, `safeMulU16`, `safePowi`, `safePowiU32` | for the subset |
| `helpers.rs`, in `helpers.go` | `LutStore::to_clut_f32`, `ToneReprCurve::is_linear`, `read_matrix_3d`, `read_vector_3d` | `LutStore8.ToClutF32`, `LutStore16.ToClutF32`, `ToneReprCurve.isLinear`, `readMatrix3d`, `readVector3d` | for the subset |
| `matan/curve_shape.rs`, in `matan_curve_shape.go` | `is_curve_linear16` | `isCurveLinear16` | for the subset |
| `writer.rs`, in `writer.go` | `FloatToFixedU8Fixed8::to_u8_fixed8` | `toU8Fixed8` | for the subset |
| `dat.rs`, in `dat.go` | `ColorDateTime`, `new_from_slice` | `ColorDateTime`, `newColorDateTimeFromSlice` | for the subset |
| `chad.rs`, in `chad.go` | `PCS_D50`, `Matrix3d::bradford`, `compute_chromatic_adaption_d`, `adaption_matrix_d`, `adapt_to_d50_d`, `adapt_to_illuminant_d` | `pcsD50`, `bradfordD`, `computeChromaticAdaptionD`, `adaptionMatrixD`, `adaptToD50D`, `adaptToIlluminantD` | for the subset |
| `chromaticity.rs`, in `chromaticity.go` | `Chromaticity`, `D65`, `D50`, `to_xyz`, `to_xyzd`, `to_xyyb` | `Chromaticity`, `ChromaticityD65`, `ChromaticityD50`, `ToXyz`, `ToXyzd`, `ToXyYb` | for the subset |
| `xyy.rs`, in `xyy.go` | `XyY`, `new`, `to_xyz`, `to_xyzd` | `XyY`, `NewXyY`, `ToXyz`, `ToXyzd` | for the subset |
| `lab.rs`, in `lab.go` | `Lab`, `new`, `f_1`, `f`, `from_pcs_xyz`, `to_pcs_xyz` | `Lab`, `NewLab`, `labF1`, `labF`, `FromPcsXyz`, `Lab.ToPcsXyz` | for the subset |
| `nd_array.rs`, in `nd_array.go` | `lerp`, `Hypercube`, `new`, `new_hypercube`, `quadlinear_vec3`, `Cube`, `new`, `new_cube`, `trilinear_vec3` | `lerpVec3`, `Hypercube`, `NewHypercube`, `NewHypercubeGrid`, `QuadlinearVec3`, `Cube`, `NewCube`, `NewCubeGrid`, `TrilinearVec3` | for the subset |

## The files of conversions

Each file `src/conversions/x.rs` is in `conversions_x.go`.

| File | Item | Go | Used by |
| --- | --- | --- | --- |
| `rgb_xyz_factory.rs` | `FIXED_POINT_SCALE`, `RgbXyzFactory`, `RgbXyzFactoryOpt`, for `u8` | `fixedPointScale`, `makeTransform`, `makeInPlaceTransform`, `makeOptimizedTransform`, `makeInPlaceOptimizedTransform` | for the subset |
| `rgbxyz.rs` | `TransformMatrixShaper`, `TransformMatrixShaperOptimized`, `to_q2_13_n` | `transformMatrixShaper`, `transformMatrixShaperOptimized`, `toQ213N` | for the subset |
| `rgbxyz.rs` | `TransformMatrixShaperScalar`, `TransformMatrixShaperOptScalar`, and their executors | `transformMatrixShaperScalar`, `transformMatrixShaperOptScalar`, `inPlace` | for the subset |
| `rgbxyz.rs` | `make_rgb_xyz_rgb_transform`, `make_rgb_xyz_rgb_transform_opt`, `make_in_place_rgb_xyz_transform`, `make_rgb_xyz_in_place_transform_opt` | `makeRgbXyzRgbTransform`, `makeRgbXyzRgbTransformOpt`, `makeInPlaceRgbXyzTransform`, `makeRgbXyzInPlaceTransformOpt` | for the subset |
| `rgbxyz_fixed.rs` | `TransformMatrixShaperFixedPointOpt`, `TransformMatrixShaperQ2_13Optimized`, `make_rgb_xyz_q2_13_opt` | `transformMatrixShaperFixedPointOpt`, `transformMatrixShaperQ213Optimized`, `makeRgbXyzQ213Opt` | for the subset |
| `gray2rgb.rs` | `TransformGray2RgbFusedExecutor`, `make_gray_to_x`, `make_gray_to_gray_in_place` | `transformGray2RgbFusedExecutor`, `makeGrayToX`, `makeGrayToGrayInPlace` | for the subset |
| `lut_transforms.rs` | `MatrixStage`, `LUT_SAMPLING`, `Grid4x3` | `matrixStage`, `lutSampling`, `grid4x3` | for the subset |
| `lut_transforms.rs` | `pcs_lab_v4_to_v2`, `pcs_lab_v2_to_v4`, `source_pcs_to_dest_pcs`, `grid_4x3_by_planes`, `make_lut_transform` | `pcsLabV4ToV2`, `pcsLabV2ToV4`, `sourcePcsToDestPcs`, `grid4x3ByPlanes`, `makeLutTransform`, `makeLut4Whole`, `bpcScaling` | for the subset |
| `lut3x3.rs` | `Lut3x3`, `make_lut_3x3`, `stage_lut_3x3`, `create_lut3x3` | `lut3x3`, `makeLut3x3`, `stageLut3x3`, `createLut3x3` | for the subset |
| `lut3x4.rs` | `create_lut3_samples`, `create_lut3_samples_norm` | `createLut3Samples`, `createLut3SamplesNorm` | for the subset |
| `lut4.rs` | `Lut4x3`, `make_lut_4x3`, `stage_lut_4x3`, `create_lut4_norm_samples`, `create_lut4` | `lut4x3`, `makeLut4x3`, `stageLut4x3`, `createLut4NormSamples`, `createLut4` | for the subset |
| `mab.rs` | `ACurves3`, `ACurves3Optimized`, `ACurves3Inverse`, `MCurves3`, `BCurves3`, `prepare_mab_3x3`, `prepare_mba_3x3` | `aCurves3`, `aCurves3Optimized`, `aCurves3Inverse`, `mCurves3`, `bCurves3`, `prepareMab3x3`, `prepareMba3x3` | for the subset |
| `mab4x3.rs` | `ACurves4x3`, `ACurves4x3Optimized`, `prepare_mab_4x3` | `aCurves4x3`, `aCurves4x3Optimized`, `prepareMab4x3` | for the subset |
| `prelude_lut_xyz_rgb.rs` | `XyzToRgbStage`, `RgbLinearizationStage`, `create_rgb_lin_lut`, `prepare_inverse_lut_rgb_xyz`, `inverse_lut_rgb_xyz_stage` | `xyzToRgbStage`, `rgbLinearizationStage`, `createRgbLinLut`, `prepareInverseLutRgbXyz`, `inverseLutRgbXyzStage` | for the subset |
| `transform_lut3_to_3.rs` | `TransformLut3x3`, `DefaultLut3x3Factory` | `transformLut3x3`, `makeTransform3x3` | for the subset |
| `transform_lut4_to_3.rs` | `DefaultVector3fLerp`, `TransformLut4To3`, `TransformLut4To3Q0_15`, `q0_15_mul`, `q0_15_lerp`, `q0_15_lerp3`, `DefaultLut4x3Factory` | `defaultVector3fLerp`, `transformLut4To3`, `transformLut4To3Q015`, `q015Mul`, `q015Lerp`, `q015Lerp3`, `makeTransform4x3` | for the subset |
| `interpolator.rs` | `BarycentricWeight`, `create_ranged_256`, `Trilinear` | `barycentricWeight`, `barycentricWeightI16`, `createRanged256`, `createRanged256I16`, `trilinear` | for the subset |
| `bpc.rs` | `TransformDirection`, `ColorProfile::detect_black_point`, `compensate_bpc_in_lut` | `transformDirection`, `ColorProfile.detectBlackPoint`, `compensateBpcInLut` | for the subset |
| `xyz_lab.rs` | `StageLabToXyz`, `StageXyzToLab` | `stageLabToXyz`, `stageXyzToLab` | for the subset |

## What the subset leaves out

The subset leaves out the transforms of 10, 12 and 16 bits and of f32 and
f64, the writer of profiles, the text tags, the other color models (Oklab,
Jzazbz, ICtCp and the others), the profiles other than sRGB and gray, the
interceptors of CMYK with alpha, the transforms of more than four inks,
and each path of SIMD.
