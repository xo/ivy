#!/bin/bash
# run-export.sh exports the iWork documents in ~/ivy/files to PDF, in
# ~/ivy/out/iwork, with the same relative paths and .pdf added. Run it as
# root through `incus exec`. It writes the job, compiles the applet, opens it
# in the desktop of the user `user`, and waits for it.
set -euo pipefail

USER_NAME=user
USER_ID=$(id -u "$USER_NAME")
HOME_DIR=/Users/$USER_NAME
IVY=$HOME_DIR/ivy
APPLET=$IVY/ivy-export.app
TIMEOUT=${TIMEOUT:-7200}

job=$IVY/job.tsv
: > "$job"
cd "$IVY/files"
find . -type f \( -name '*.pages' -o -name '*.key' -o -name '*.numbers' \) | sort | while read -r f; do
	rel=${f#./}
	case "$rel" in
		*.pages) app=Pages ;;
		*.key) app=Keynote ;;
		*.numbers) app=Numbers ;;
	esac
	printf '%s\t%s\t%s\n' "$app" "$IVY/files/$rel" "$IVY/out/iwork/$rel.pdf" >> "$job"
done
echo "$(wc -l < "$job") documents"

# The applet keeps its path and its ad hoc signature, so the permission that
# the user gives once stays valid. It is compiled again only when the script
# changes.
if [ ! -d "$APPLET" ] || [ "$IVY/export.applescript" -nt "$APPLET" ]; then
	rm -rf "$APPLET"
	osacompile -o "$APPLET" "$IVY/export.applescript"
fi
chown -R "$USER_NAME":staff "$IVY"

rm -f "$IVY/out/iwork/.done"
launchctl asuser "$USER_ID" sudo -u "$USER_NAME" open -W "$APPLET" &
start=$(date +%s)
while [ ! -f "$IVY/out/iwork/.done" ]; do
	if [ $(( $(date +%s) - start )) -gt "$TIMEOUT" ]; then
		echo "the export did not finish within $TIMEOUT seconds" >&2
		exit 1
	fi
	sleep 5
done
echo "done: $(grep -c $'\tok\t' "$IVY/out/iwork/export.tsv") documents exported"
