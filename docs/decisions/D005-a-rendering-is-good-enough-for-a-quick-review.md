# D5. A rendering is good enough for a quick review

Status: Decided.

ivy does not try to render a document exactly as the program that made it.
A rendering is good when a person can look at it and see what the document
is and what it holds. That is the quality of a preview in a file manager, at
the size of a page.

This changes the cost of most formats:

1. An embedded preview is a valid result. Many formats carry one, such as the
   thumbnail of an OpenDocument file (D11).
2. A simple layout is a valid result. A Word document can render as its text,
   its headings, its tables and its images in order, with page breaks that
   differ from Word.
3. A feature that changes little of what a person sees can be left out, or
   drawn in a simpler way. Examples are the knockout groups and the blending
   of CMYK in PDF, hyphenation, and the hinting of fonts.
4. A file must never crash ivy or make it hang. Malformed and hostile input
   is still in scope. A page that ivy cannot draw in full is drawn in part.
   Each decoder has limits on memory and on time, and a fuzz test.

The test module compares ivy with the outside programs that iv runs today
(D13). The comparison has a tolerance that fits a review, not the bytes of
the oracle.
