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
| D7 | [ivy reuses okage and magic, then copies permissive code](D007-ivy-reuses-okage-and-magic-then-copies-permissive-code.md) | Decided |
| D8 | [okage draws, and magic identifies and decompresses](D008-okage-draws-and-magic-identifies-and-decompresses.md) | Decided |
| D9 | [ivy uses generics and the features of Go 1.27](D009-ivy-uses-generics-and-the-features-of-go-1-27.md) | Decided |
| D10 | [The formats share four layers](D010-the-formats-share-four-layers.md) | Decided |
| D11 | [The document API is a small document with capabilities](D011-the-document-api-is-a-small-document-with-capabilities.md) | Decided |
| D12 | [Encrypted files take a password, and DRM is decoded where it is possible](D012-encrypted-files-take-a-password-and-drm-is-decoded-where-it-is-possible.md) | Decided |
| D13 | [The test module holds the oracles, and runs by hand](D013-the-test-module-holds-the-oracles-and-runs-by-hand.md) | Decided |
| D14 | [ivy reads PDF as a port of hayro and moxcms](D014-ivy-reads-pdf-as-a-port-of-hayro-and-moxcms.md) | Decided |
| D15 | [The layout is a port of WeasyPrint on the parsers of okage](D015-the-layout-is-a-port-of-weasyprint-on-the-parsers-of-okage.md) | Decided |
