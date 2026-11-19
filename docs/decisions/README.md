# Decisions

Every decision of this project is a file in this folder, named by its number
and its title. This table is the index. Find the number here, then open the
file.

Each file opens with its status. `Decided` means that the maintainers chose
it. `Proposed` means that it waits for the maintainers. A decision that
changes an earlier one says so in its status, as `Amends D3`, and the earlier
one says it back, as `Amended by D9`. Read the status before the decision.

A bare number, such as D3, names a decision in this folder. A decision of
another repository names that repository, such as dbmeta D110 or okage D16.

A new decision gets the next number and a file of its own. Add its row here.
An open question is not a decision. It goes at the end of
[`../PLAN.md`](../PLAN.md) until the maintainers answer it.

The decisions were consolidated on 2026-11-16, before the first commit (D4).

| # | Decision | Status |
| --- | --- | --- |
| D1 | [ivy decodes, renders and encodes file formats in pure Go](D001-ivy-decodes-renders-and-encodes-file-formats-in-pure-go.md) | Decided |
| D2 | [ivy is set up for coding agents as dbmeta D110 says](D002-ivy-is-set-up-for-coding-agents-as-dbmeta-d110-says.md) | Decided |
| D3 | [The plan and the rules come before any code](D003-the-plan-and-the-rules-come-before-any-code.md) | Decided |
| D4 | [Each decision is a file of its own](D004-each-decision-is-a-file-of-its-own.md) | Decided |
| D5 | [A rendering is good enough for a quick review](D005-a-rendering-is-good-enough-for-a-quick-review.md) | Decided |
| D6 | [Each decoder and each encoder is a package of its own](D006-each-decoder-and-each-encoder-is-a-package-of-its-own.md) | Decided |
| D7 | [ivy reuses okage and magic, then copies permissive code](D007-ivy-reuses-okage-and-magic-then-copies-permissive-code.md) | Decided. Amended by D42 |
| D8 | [okage draws, and magic identifies and decompresses](D008-okage-draws-and-magic-identifies-and-decompresses.md) | Decided. Amended by D50 |
| D9 | [ivy uses generics and the features of Go 1.27](D009-ivy-uses-generics-and-the-features-of-go-1-27.md) | Decided |
| D10 | [The formats share four layers](D010-the-formats-share-four-layers.md) | Decided |
| D11 | [The document API is a small document with capabilities](D011-the-document-api-is-a-small-document-with-capabilities.md) | Decided |
| D12 | [Encrypted files take a password, and DRM is decoded where it is possible](D012-encrypted-files-take-a-password-and-drm-is-decoded-where-it-is-possible.md) | Decided |
| D13 | [The test module holds the oracles, and runs by hand](D013-the-test-module-holds-the-oracles-and-runs-by-hand.md) | Decided |
| D14 | [ivy reads PDF as a port of hayro and moxcms](D014-ivy-reads-pdf-as-a-port-of-hayro-and-moxcms.md) | Decided. Amended by D16. Amended by D20 |
| D15 | [The layout is a port of WeasyPrint on the parsers of okage](D015-the-layout-is-a-port-of-weasyprint-on-the-parsers-of-okage.md) | Decided |
| D16 | [The port of hayro uses Go, and ports a subset of moxcms](D016-the-port-of-hayro-uses-go-and-ports-a-subset-of-moxcms.md) | Decided. Amends D14. Amended by D17. Amended by D20 |
| D17 | [ivy ports all of hayro-ccitt, and decodes JPEG with Go](D017-ivy-ports-all-of-hayro-ccitt-and-decodes-jpeg-with-go.md) | Decided. Amends D16 |
| D18 | [A port can guard against hostile input](D018-a-port-can-guard-against-hostile-input.md) | Decided |
| D19 | [ivy does not hint glyphs, for now](D019-ivy-does-not-hint-glyphs-for-now.md) | Decided |
| D20 | [The crate hayro is ported onto the display list](D020-the-crate-hayro-is-ported-onto-the-display-list.md) | Decided. Amends D14. Amends D16 |
| D21 | [Phase 4 builds its parsers in okage and its decoders in magic](D021-phase-4-builds-its-parsers-in-okage-and-its-decoders-in-magic.md) | Decided |
| D22 | [A program can give the folder of a Markdown or HTML file](D022-a-program-can-give-the-folder-of-a-markdown-or-html-file.md) | Decided |
| D23 | [A program can ask for the fonts of the system](D023-a-program-can-ask-for-the-fonts-of-the-system.md) | Decided |
| D24 | [Phase 5 copies gowemf and draws charts with zuzu](D024-phase-5-copies-gowemf-and-draws-charts-with-zuzu.md) | Decided. Amended by D27 |
| D25 | [ivy derives the keys of OpenDocument with Argon2id](D025-ivy-derives-the-keys-of-opendocument-with-argon2id.md) | Decided |
| D26 | [ivy reads DjVu with a copy of the decoders of dejapdf](D026-ivy-reads-djvu-with-a-copy-of-the-decoders-of-dejapdf.md) | Decided. Amended by D38 |
| D27 | [ivy draws the charts of Office itself](D027-ivy-draws-the-charts-of-office-itself.md) | Decided. Amends D24 |
| D28 | [ivy reads fonts with the typesetting of okage, and fills small glyphs finely](D028-ivy-reads-fonts-with-the-typesetting-of-okage-and-fills-small-glyphs-finely.md) | Decided. Amended by D46 |
| D29 | [ivy interprets PostScript](D029-ivy-interprets-postscript.md) | Decided |
| D30 | [vsdx draws the pages of Visio](D030-vsdx-draws-the-pages-of-visio.md) | Decided |
| D31 | [pdf can render another page box](D031-pdf-can-render-another-page-box.md) | Decided |
| D32 | [A program chooses the paper of a document without one](D032-a-program-chooses-the-paper-of-a-document-without-one.md) | Decided. Amended by D33 |
| D33 | [An ebook takes the paper of the configuration](D033-an-ebook-takes-the-paper-of-the-configuration.md) | Decided. Amends D32 |
| D34 | [A text file joins the lines of a paragraph](D034-a-text-file-joins-the-lines-of-a-paragraph.md) | Decided |
| D35 | [ivy.Info has the publisher and the identifier](D035-ivy-info-has-the-publisher-and-the-identifier.md) | Decided |
| D36 | [csv has an option for its separator, and does not guess dates](D036-csv-has-an-option-for-its-separator-and-does-not-guess-dates.md) | Decided |
| D37 | [The text and the outline of a document wait for one capability](D037-the-text-and-the-outline-of-a-document-wait-for-one-capability.md) | Decided |
| D38 | [An indirect DjVu document opens from a folder that a program gives](D038-an-indirect-djvu-document-opens-from-a-folder-that-a-program-gives.md) | Decided. Amends D26 |
| D39 | [pdf refuses a file larger than the memory limit](D039-pdf-refuses-a-file-larger-than-the-memory-limit.md) | Decided |
| D40 | [msppt does not read PowerPoint 95](D040-msppt-does-not-read-powerpoint-95.md) | Decided |
| D41 | [A fuzz test makes an input smaller for 2 seconds at most](D041-a-fuzz-test-makes-an-input-smaller-for-2-seconds-at-most.md) | Decided |
| D42 | [A rewrite of the bytes of HTML is not a second reader of HTML](D042-a-rewrite-of-the-bytes-of-html-is-not-a-second-reader-of-html.md) | Decided. Amends D7 |
| D43 | [The layout lays out the whole document when a page needs the later pages](D043-the-layout-lays-out-the-whole-document-when-a-page-needs-the-later-pages.md) | Decided |
| D44 | [ivy does not aim at TinyGo](D044-ivy-does-not-aim-at-tinygo.md) | Decided |
| D45 | [The backlog drops ivy.Cache, keeps JPEG XR, and waits on JPEG and SVM](D045-the-backlog-drops-ivy-cache-keeps-jpeg-xr-and-waits-on-jpeg-and-svm.md) | Decided |
| D46 | [ivy fills small glyphs with the coverage of okage](D046-ivy-fills-small-glyphs-with-the-coverage-of-okage.md) | Decided. Amends D28 |
| D47 | [A large page renders in bands on goroutines](D047-a-large-page-renders-in-bands-on-goroutines.md) | Decided |
| D48 | [A book counts its documents at the same time](D048-a-book-counts-its-documents-at-the-same-time.md) | Decided. Amended by D55 |
| D49 | [A PDF document keeps its decoded images](D049-a-pdf-document-keeps-its-decoded-images.md) | Decided |
| D50 | [ivy changes okage and magic itself](D050-ivy-changes-okage-and-magic-itself.md) | Decided. Amends D8 |
| D51 | [msdoc draws its shapes with internal/odraw](D051-msdoc-draws-its-shapes-with-internal-odraw.md) | Decided |
| D52 | [A line of a word processor can pass the page by its descent](D052-a-line-of-a-word-processor-can-pass-the-page-by-its-descent.md) | Decided |
| D53 | [A metafile draws glyph indexes through their text](D053-a-metafile-draws-glyph-indexes-through-their-text.md) | Decided |
| D54 | [The text bodies of a scene share a font configuration](D054-the-text-bodies-of-a-scene-share-a-font-configuration.md) | Decided |
| D55 | [A long sheet is laid out in parts that end at a probe row](D055-a-long-sheet-is-laid-out-in-parts-that-end-at-a-probe-row.md) | Decided. Amends D48 |
| D56 | [The layout follows Word in three places for word processors](D056-the-layout-follows-word-in-three-places-for-word-processors.md) | Decided. Amended by D59 |
| D57 | [The lines of a large document are laid out ahead on goroutines](D057-the-lines-of-a-large-document-are-laid-out-ahead-on-goroutines.md) | Decided |
| D58 | [pdfenc writes the pages of a document as PDF through okage](D058-pdfenc-writes-the-pages-of-a-document-as-pdf-through-okage.md) | Decided |
| D59 | [The space before is left out at the top of a column](D059-the-space-before-is-left-out-at-the-top-of-a-column.md) | Decided. Amends D56 |
