# Test files of msppt

`plain.ppt` and `rc4cryptoapi_password.ppt` come from the folder
`tests/inputs` of nolze/msoffcrypto-tool, under the MIT license, at the
commit `bdb71ea5dfbdee2f3cd86f8852e74be7a6f15667`. The license is in
`LICENSE-msoffcrypto-tool.txt`. PowerPoint for the Mac wrote them, with one
slide. RC4 CryptoAPI encrypts `rc4cryptoapi_password.ppt`, with the password
`Password1234_`.

LibreOffice 26.8 wrote `slides.ppt` through its API, with two slides. The
first slide holds a title and a subtitle. The second slide holds a title, a
body of three points and a text box. LibreOffice writes a thumbnail of 442 KB
into the summary information of a .ppt file. To keep the test file small,
`slides.ppt` holds only the streams "Current User" and "PowerPoint Document"
of that file, and a new summary information with the title "Ivy test slides"
and the author "Ivy Tester". The writer of Compound Files of
`internal/fixture` wrote it.

LibreOffice 26.8 converted a flat OpenDocument presentation into
`shapes.ppt`, with one slide. The slide has a background of the color
#003366, a green rectangle with a magenta outline and red bold text of 28
points, an ellipse with a gradient from red at the top to yellow at the
bottom, a white triangle of a custom geometry, and a blue picture in PNG.
The file holds only the streams "Current User", "PowerPoint Document" and
"Pictures" of that file, because LibreOffice writes a large thumbnail. The
writer of Compound Files of `internal/fixture` wrote it.
