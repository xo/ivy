# unopdf.py converts a file with a password, or a CSV file on a given page,
# to PDF with LibreOffice. The command line of soffice cannot take a
# password or a page, but its UNO API can.
#
# Usage: python unopdf.py SOFFICE PROFILE PIPE IN OUT
#
# SOFFICE is the program soffice. PROFILE is the URL of the profile of
# LibreOffice. PIPE is a name for the pipe of UNO that no other soffice
# uses. IN is the file and OUT is the PDF file. The environment variable
# IVY_REVIEW_PASSWORD holds the password, so that the list of processes
# does not show it.
#
# For a CSV file, the environment variable IVY_REVIEW_CSV holds the options
# of the CSV filter of LibreOffice, such as "44,34,76,1", and the variable
# IVY_REVIEW_PAGE holds the page of each sheet: the width, the height, and
# the margins at the top, the bottom, the left and the right, in hundredths
# of a millimeter, separated by spaces. The sheets then have no header and
# no footer, and they print their grid, as in ivy.
#
# The script starts soffice without a window, opens IN with the password,
# sets the page, stores the document as PDF with the filter of its type of
# document, and stops soffice. It exits with 2 when LibreOffice does not
# open the file.

import os
import subprocess
import sys
import time

import uno
from com.sun.star.beans import PropertyValue
from com.sun.star.connection import NoConnectException

# FILTERS maps a service of a document to its export filter of PDF.
FILTERS = [
    ("com.sun.star.text.TextDocument", "writer_pdf_Export"),
    ("com.sun.star.sheet.SpreadsheetDocument", "calc_pdf_Export"),
    ("com.sun.star.presentation.PresentationDocument", "impress_pdf_Export"),
    ("com.sun.star.drawing.DrawingDocument", "draw_pdf_Export"),
]

# CONNECT_SECONDS bounds the wait for soffice to accept a connection.
CONNECT_SECONDS = 120


def prop(name, value):
    p = PropertyValue()
    p.Name = name
    p.Value = value
    return p


def connect(proc, pipe):
    local = uno.getComponentContext()
    resolver = local.ServiceManager.createInstanceWithContext(
        "com.sun.star.bridge.UnoUrlResolver", local)
    url = "uno:pipe,name=%s;urp;StarOffice.ComponentContext" % pipe
    end = time.monotonic() + CONNECT_SECONDS
    while True:
        if proc.poll() is not None:
            raise RuntimeError("soffice stopped with the status %d before it accepted a connection. "
                               "Another soffice can use the same profile" % proc.returncode)
        try:
            return resolver.resolve(url)
        except NoConnectException:
            if time.monotonic() > end:
                raise RuntimeError("soffice did not accept a connection in %d seconds" % CONNECT_SECONDS)
            time.sleep(0.1)


def set_page(doc, spec):
    """Sets the page of each sheet of doc to spec, IVY_REVIEW_PAGE."""
    width, height, top, bottom, left, right = [int(v) for v in spec.split()]
    styles = doc.StyleFamilies.getByName("PageStyles")
    names = set()
    for i in range(doc.Sheets.Count):
        names.add(doc.Sheets.getByIndex(i).PageStyle)
    for name in names:
        style = styles.getByName(name)
        # IsLandscape turns the size of the page, so the size comes after it
        style.IsLandscape = width > height
        style.Width = width
        style.Height = height
        style.TopMargin = top
        style.BottomMargin = bottom
        style.LeftMargin = left
        style.RightMargin = right
        style.HeaderIsOn = False
        style.FooterIsOn = False
        style.PrintGrid = True


def convert(desktop, src, dst, password):
    props = [prop("Hidden", True), prop("ReadOnly", True)]
    if password:
        props.append(prop("Password", password))
    csv = os.environ.get("IVY_REVIEW_CSV", "")
    if csv:
        props += [prop("FilterName", "Text - txt - csv (StarCalc)"), prop("FilterOptions", csv)]
    what = "with the password" if password else "at all"
    try:
        doc = desktop.loadComponentFromURL(uno.systemPathToFileUrl(src), "_blank", 0, tuple(props))
    except Exception as e:
        print("LibreOffice did not open the file %s: %s" % (what, e), file=sys.stderr)
        return 2
    if doc is None:
        if password:
            print("LibreOffice did not open the file with the password: the password is wrong, "
                  "or LibreOffice cannot read the encryption", file=sys.stderr)
        else:
            print("LibreOffice did not open the file", file=sys.stderr)
        return 2
    try:
        name = next((f for s, f in FILTERS if doc.supportsService(s)), None)
        if name is None:
            print("LibreOffice opened the file as a document that has no export filter of PDF", file=sys.stderr)
            return 1
        page = os.environ.get("IVY_REVIEW_PAGE", "")
        if page:
            if name != "calc_pdf_Export":
                print("LibreOffice opened the file as a document that is not a sheet, so it has no page to set",
                      file=sys.stderr)
                return 1
            set_page(doc, page)
        doc.storeToURL(uno.systemPathToFileUrl(dst), (prop("FilterName", name),))
    finally:
        doc.close(True)
    return 0


def main():
    soffice, profile, pipe, src, dst = sys.argv[1:6]
    password = os.environ.get("IVY_REVIEW_PASSWORD", "")
    proc = subprocess.Popen([
        soffice, "--headless", "--invisible", "--norestore", "--nologo", "--nodefault", "--nolockcheck",
        "-env:UserInstallation=" + profile,
        "--accept=pipe,name=%s;urp;StarOffice.ComponentContext" % pipe,
    ], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        ctx = connect(proc, pipe)
        desktop = ctx.ServiceManager.createInstanceWithContext("com.sun.star.frame.Desktop", ctx)
        try:
            status = convert(desktop, os.path.abspath(src), os.path.abspath(dst), password)
        finally:
            try:
                desktop.terminate()
            except Exception:
                # soffice can stop before the call returns
                pass
        try:
            proc.wait(timeout=30)
        except subprocess.TimeoutExpired:
            pass
        return status
    finally:
        # the program that runs this script kills soffice.bin, the child of
        # soffice, with the process group
        if proc.poll() is None:
            proc.kill()
            proc.wait()


if __name__ == "__main__":
    sys.exit(main())
