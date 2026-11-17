#!/bin/bash
# setup.sh prepares the macOS VM of the review. Run it as root through
# `incus exec`. It names the machine ivy-macos, installs mas, the command
# line tool of the App Store, with Homebrew as the user `user`, and then
# installs Pages, Keynote and Numbers if an Apple ID is signed in to the App
# Store. Without one, it says what the maintainer must do, and exits with 2.
set -euo pipefail

USER_NAME=user
USER_ID=$(id -u "$USER_NAME")

scutil --set ComputerName ivy-macos
scutil --set HostName ivy-macos
scutil --set LocalHostName ivy-macos

as_user() {
	launchctl asuser "$USER_ID" sudo -u "$USER_NAME" -i "$@"
}

BREW=/usr/local/bin/brew
[ -x "$BREW" ] || BREW=/opt/homebrew/bin/brew
if ! as_user bash -lc 'command -v mas' > /dev/null; then
	as_user "$BREW" install mas
fi

# app_id gives the App Store ID of an application. macOS has bash 3.2,
# which has no associative arrays.
app_id() {
	case "$1" in
		Pages) echo 409201541 ;;
		Keynote) echo 409183694 ;;
		Numbers) echo 409203825 ;;
	esac
}
missing=0
for app in Pages Keynote Numbers; do
	if [ -d "/Applications/$app.app" ]; then
		echo "$app: $(defaults read "/Applications/$app.app/Contents/Info" CFBundleShortVersionString)"
		continue
	fi
	if ! as_user bash -lc "mas install $(app_id "$app")"; then
		missing=1
	fi
done
if [ "$missing" = 1 ]; then
	cat >&2 <<'EOF'
Pages, Keynote or Numbers is not installed. mas installs only for an Apple
ID that is signed in to the App Store. Open the App Store in the desktop of
the VM, sign in, and run this script again.
EOF
	exit 2
fi
mkdir -p /Users/$USER_NAME/ivy/files /Users/$USER_NAME/ivy/out
chown -R "$USER_NAME":staff /Users/$USER_NAME/ivy
echo "the macOS VM is ready"
