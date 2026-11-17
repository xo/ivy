#!/bin/bash
# ivy-vm.sh drives the two VMs that give the review the references of
# Microsoft Office, Windows and Apple iWork. The VMs are in the incus project
# ivy: ivy-win11 (Windows 11 with Office Professional Plus 2024 and Visio
# Professional 2024) and ivy-macos (macOS 15 with Pages, Keynote and
# Numbers). README.md in this folder says how they were made.
#
#   ivy-vm.sh start win|mac
#   ivy-vm.sh stop win|mac
#   ivy-vm.sh setup win|mac
#   ivy-vm.sh windows SCRIPT NAME EXTENSIONS [ARGS]
#   ivy-vm.sh iwork
#
# `windows` copies the corpus files of the given kinds (such as
# docx,doc,xlsx), under their manifest names and with passwords.tsv, from the
# bundle subcommand of the review into C:\ivy\files of ivy-win11, runs SCRIPT there in the
# desktop of the user with `-Source C:\ivy\files -Out C:\ivy\out\NAME` and
# ARGS, and copies C:\ivy\out\NAME back into the cache of the review, under
# test/review/cache/NAME. NAME can hold a slash, such as windows/xps.
# `iwork` does the same for the .pages, .key and
# .numbers files on ivy-macos, into test/review/cache/iwork.
#
# The outputs stay in the cache, which git ignores, and the review only reads
# them: it never starts a VM. Each export sends only the files whose output is
# missing or whose content changed, as $CACHE/NAME/sources.tsv records, and
# starts no VM when the cache is up to date. FORCE=1 redoes every file, and
# RETRY=1 redoes the files that failed.
#
# Run one VM at a time. The host also runs the topi VMs, which must not run
# at the same time as their copies. `start` refuses to start a VM while
# another VM runs, in any project, unless ALLOW_MORE_VMS=1.
set -euo pipefail

HERE=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$HERE/../../../.." && pwd)
CACHE=${CACHE:-$REPO/test/review/cache}
PROJECT=ivy
WIN=ivy-win11
MAC=ivy-macos

die() {
	echo "ivy-vm: $*" >&2
	exit 1
}

vm() {
	case "$1" in
		win) echo "$WIN" ;;
		mac) echo "$MAC" ;;
		*) die "unknown VM $1, want win or mac" ;;
	esac
}

wexec() {
	incus exec "$WIN" --project "$PROJECT" -- powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command "$1" < /dev/null
}

mexec() {
	incus exec "$MAC" --project "$PROJECT" -- "$@" < /dev/null
}

# wait_agent waits until the incus agent of the VM answers.
wait_agent() {
	local name=$1
	for _ in $(seq 1 120); do
		if incus exec "$name" --project "$PROJECT" -- true < /dev/null 2> /dev/null ||
			incus exec "$name" --project "$PROJECT" -- cmd /c ver < /dev/null > /dev/null 2>&1; then
			return 0
		fi
		sleep 5
	done
	die "the agent of $name did not answer"
}

start() {
	local name others
	name=$(vm "$1")
	if [ "$(incus list "$name" --project "$PROJECT" -f csv -c s)" != RUNNING ]; then
		# Two or three VMs at a time took all the memory of the host and
		# stopped it, so start a VM only when no other VM runs, in any project.
		others=$(incus list --all-projects type=virtual-machine status=running -f csv -c en < /dev/null | tr '\n' ' ')
		if [ -n "$others" ] && [ "${ALLOW_MORE_VMS:-}" != 1 ]; then
			die "other VMs run ($others), so $name does not start. Stop them, or set ALLOW_MORE_VMS=1"
		fi
		incus start "$name" --project "$PROJECT" < /dev/null
	fi
	wait_agent "$name"
	echo "$name is running"
}

stop() {
	local name
	name=$(vm "$1")
	case "$1" in
		# incus stop waits for Windows, which can ignore the ACPI request.
		win) wexec 'Stop-Computer -Force' || true ;;
		mac) mexec shutdown -h now || true ;;
	esac
	for _ in $(seq 1 60); do
		[ "$(incus list "$name" --project "$PROJECT" -f csv -c s)" = STOPPED ] && return 0
		sleep 5
	done
	incus stop --force "$name" --project "$PROJECT" < /dev/null
}

# out_of NAME FILE gives the path in the cache of the output that the export
# NAME makes for the corpus file FILE.
out_of() {
	case "$1" in
		windows/xps) echo "$CACHE/$1/$2.p1.png" ;;
		windows/gdiplus) echo "$CACHE/$1/$2.png" ;;
		*) echo "$CACHE/$1/$2.pdf" ;;
	esac
}

# bundle KINDS NAME makes a folder `files` with the corpus files of these
# kinds under their manifest names, and passwords.tsv, as the bundle
# subcommand of the review does, and prints the path of its parent. It reads
# corpus.tsv itself, so that it works while the Go code does not build.
#
# The outputs stay in the cache. A file is left out when
# $CACHE/NAME/sources.tsv says that the export already ran on the same
# content (the same sha256): with a result, or with a failure. FORCE=1 sends
# every file again, and RETRY=1 sends again the files that failed. The list
# of the files sent is in $tmp/sent.
bundle() {
	local kinds=",$1," name=$2 tmp sources
	tmp=$(mktemp -d)
	sources=$CACHE/$name/sources.tsv
	mkdir -p "$tmp/files"
	: > "$tmp/files/passwords.tsv"
	: > "$tmp/sent"
	tail -n +2 "$REPO/test/review/corpus.tsv" | while IFS=$'\t' read -r file kind url _ _ _ _ note; do
		[[ $kinds == *",$kind,"* ]] || continue
		local src=$CACHE/files/$file
		[[ $url == repo:* ]] && src=$REPO/${url#repo:}
		[ -f "$src" ] || continue
		local sum line
		sum=$(sha256sum "$src" | cut -d' ' -f1)
		if [ "${FORCE:-}" != 1 ] && [ -f "$sources" ]; then
			line=$(awk -F'\t' -v f="$file" -v s="$sum" '$1 == f && $2 == s { r = $3 } END { print r }' "$sources")
			if [ "$line" = ok ] && [ -f "$(out_of "$name" "$file")" ]; then
				continue
			fi
			if [ "$line" = error ] && [ "${RETRY:-}" != 1 ]; then
				continue
			fi
		fi
		mkdir -p "$tmp/files/$(dirname "$file")"
		cp "$src" "$tmp/files/$file"
		printf '%s\t%s\n' "$file" "$sum" >> "$tmp/sent"
		local pw
		pw=$(grep -o 'password=[^ ;,]*' <<< "$note" | head -1 | cut -d= -f2-)
		if [ -n "$pw" ]; then
			printf '%s\t%s\n' "$file" "$pw" >> "$tmp/files/passwords.tsv"
		fi
	done
	echo "$tmp"
}

# record NAME SENT writes to $CACHE/NAME/sources.tsv, for each file of SENT,
# its sha256 and whether the export made its output.
record() {
	local name=$1 sent=$2 sources=$CACHE/$1/sources.tsv file sum result
	touch "$sources"
	while IFS=$'\t' read -r file sum; do
		result=error
		[ -f "$(out_of "$name" "$file")" ] && result=ok
		awk -F'\t' -v f="$file" '$1 != f' "$sources" > "$sources.tmp"
		printf '%s\t%s\t%s\t%s\n' "$file" "$sum" "$result" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >> "$sources.tmp"
		mv "$sources.tmp" "$sources"
	done < "$sent"
	echo "$(grep -c $'\tok\t' "$sources") ok and $(grep -c $'\terror\t' "$sources") failed in $sources"
}

setup() {
	case "$1" in
		win)
			start win
			wexec 'New-Item -ItemType Directory -Force C:\ivy | Out-Null'
			incus file push "$HERE/windows/setup.ps1" "$HERE/windows/office.xml" "$HERE/windows/run-as-user.ps1" "$WIN/ivy/" --project "$PROJECT" < /dev/null
			wexec '& C:\ivy\setup.ps1'
			;;
		mac)
			start mac
			mexec mkdir -p /Users/user/ivy
			incus file push --uid 501 --gid 20 --mode 0755 "$HERE/macos/setup.sh" "$HERE/macos/run-export.sh" "$MAC/Users/user/ivy/" --project "$PROJECT" < /dev/null
			incus file push --uid 501 --gid 20 --mode 0644 "$HERE/macos/export.applescript" "$MAC/Users/user/ivy/" --project "$PROJECT" < /dev/null
			mexec /Users/user/ivy/setup.sh
			;;
		*) die "unknown VM $1" ;;
	esac
}

windows() {
	local script=$1 name=$2 exts=$3
	shift 3
	[ -f "$script" ] || die "no script $script"
	local tmp n
	tmp=$(bundle "$exts" "$name")
	n=$(wc -l < "$tmp/sent")
	if [ "$n" = 0 ]; then
		rm -rf "$tmp"
		echo "the cache $CACHE/$name is up to date; FORCE=1 or RETRY=1 runs the export again"
		return 0
	fi
	start win
	wexec "Remove-Item -Recurse -Force -ErrorAction SilentlyContinue C:\\ivy\\files, 'C:\\ivy\\out\\$name'; New-Item -ItemType Directory -Force C:\\ivy\\scripts | Out-Null"
	incus file push "$HERE/windows/run-as-user.ps1" "$WIN/ivy/" --project "$PROJECT" < /dev/null
	incus file push "$script" "$WIN/ivy/scripts/" --project "$PROJECT" < /dev/null
	echo "copying $n files into $WIN"
	incus file push -r "$tmp/files" "$WIN/ivy/" --project "$PROJECT" < /dev/null
	local base
	base=$(basename "$script")
	wexec "& C:\\ivy\\run-as-user.ps1 -Script C:\\ivy\\scripts\\$base -Done C:\\ivy\\out\\$name\\.done -Arguments '-Source C:\\ivy\\files -Out C:\\ivy\\out\\$name $*'" || true
	mkdir -p "$CACHE/$name"
	local out
	out=$(mktemp -d)
	incus file pull -r "$WIN/ivy/out/$name" "$out/" --project "$PROJECT" < /dev/null
	cp -r "$out/$(basename "$name")/." "$CACHE/$name/"
	rm -rf "$out"
	record "$name" "$tmp/sent"
	rm -rf "$tmp"
}

iwork() {
	local tmp n
	tmp=$(bundle pages,key,numbers iwork)
	n=$(wc -l < "$tmp/sent")
	if [ "$n" = 0 ]; then
		rm -rf "$tmp"
		echo "the cache $CACHE/iwork is up to date; FORCE=1 or RETRY=1 runs the export again"
		return 0
	fi
	start mac
	mexec rm -rf /Users/user/ivy/files /Users/user/ivy/out/iwork
	incus file push --uid 501 --gid 20 --mode 0644 "$HERE/macos/export.applescript" "$MAC/Users/user/ivy/" --project "$PROJECT" < /dev/null
	incus file push --uid 501 --gid 20 --mode 0755 "$HERE/macos/run-export.sh" "$MAC/Users/user/ivy/" --project "$PROJECT" < /dev/null
	echo "copying $n files into $MAC"
	incus file push -r --uid 501 --gid 20 "$tmp/files" "$MAC/Users/user/ivy/" --project "$PROJECT" < /dev/null
	mexec /Users/user/ivy/run-export.sh || true
	mkdir -p "$CACHE/iwork"
	local out
	out=$(mktemp -d)
	incus file pull -r "$MAC/Users/user/ivy/out/iwork" "$out/" --project "$PROJECT" < /dev/null
	cp -r "$out/iwork/." "$CACHE/iwork/"
	rm -rf "$out"
	record iwork "$tmp/sent"
	rm -rf "$tmp"
}

cmd=${1:-}
shift || true
case "$cmd" in
	start) start "${1:?win or mac}" ;;
	stop) stop "${1:?win or mac}" ;;
	setup) setup "${1:?win or mac}" ;;
	windows) windows "$@" ;;
	iwork) iwork ;;
	*) die "usage: ivy-vm.sh start|stop|setup win|mac, windows SCRIPT NAME EXTENSIONS [ARGS], or iwork" ;;
esac
