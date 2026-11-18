# D49. A PDF document keeps its decoded images

Status: Decided.

Each render of a PDF page decoded each of its images again: the filters,
the color conversion and the soft mask. Since D47, `RenderImage` renders a
large page in bands, and each band decoded the images of the page again.
`pdfjs/pdf/a1-plans.pdf` spent about 2 s on its four images of 2480 by 2630
pixels for each render. The maintainers asked on 2026-11-18 to keep the
decoded images of a document between the renders.

The maintainers decided:

1. A PDF document keeps its decoded images, and the pixels of each image
   resized to the size that a page draws, in its render cache. The bands of
   a page and the pages of the document share the cache.
2. The cache holds at most a quarter of `Limits.MaxMemory` (D39), 128 MiB by
   default, and drops the image that it used least recently. An image larger
   than that budget is not kept, and still draws.
3. The key of a decoded image is the object of its stream, the kind of the
   image (an image, a soft mask or a stencil mask) and, for an image that
   `JPXDecode` downsamples, the requested size. The dictionary of the stream
   fixes the rest. The key of the resized pixels adds their size.
4. Two renders that need the same image at the same time decode it once:
   the second one waits for the first. If the first one fails, the second
   one decodes the image itself.
5. The cache does not keep an inline image, an image with a transfer
   function or a color space that the resources of a page name, a decode
   that failed, or the result of a render that a context stopped.

The pixels of each PDF and Illustrator document of the corpus of the review
stay the same. On 2026-11-18 each render after the first of
`pdfjs/pdf/a1-plans.pdf` went from about 2.1 to 3.8 s to 0.2 to 0.6 s, and
its allocations went from 8 GB to 0.9 GB.
