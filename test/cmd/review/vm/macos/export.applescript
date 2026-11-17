-- export.applescript exports Pages, Keynote and Numbers documents to PDF.
-- run-export.sh compiles it into the applet ivy-export.app and opens it in
-- the desktop of the user. The first time, macOS asks the user once for each
-- application whether ivy-export may control it.
--
-- The applet reads the job from ~/ivy/job.tsv: one line for each document,
-- with the application, the input path and the output path, separated by
-- tabs. It writes one line for each document to ~/ivy/out/iwork/export.tsv,
-- and the file ~/ivy/out/iwork/.done at the end.

on run
	set home to POSIX path of (path to home folder)
	set jobFile to home & "ivy/job.tsv"
	set outDir to home & "ivy/out/iwork/"
	set logFile to outDir & "export.tsv"
	do shell script "mkdir -p " & quoted form of outDir & " && rm -f " & quoted form of (outDir & ".done") & " && printf 'file\\tapplication\\tresult\\terror\\n' > " & quoted form of logFile
	set jobText to read (POSIX file jobFile) as «class utf8»
	set AppleScript's text item delimiters to tab
	repeat with jobLine in paragraphs of jobText
		if (length of jobLine) > 0 then
			set appName to text item 1 of jobLine
			set inPath to text item 2 of jobLine
			set outPath to text item 3 of jobLine
			set outcome to "ok"
			set errText to ""
			try
				my exportOne(appName, inPath, outPath)
			on error msg
				set outcome to "error"
				set errText to msg
			end try
			do shell script "printf '%s\\t%s\\t%s\\t%s\\n' " & quoted form of inPath & " " & quoted form of appName & " " & quoted form of outcome & " " & quoted form of errText & " >> " & quoted form of logFile
		end if
	end repeat
	do shell script "touch " & quoted form of (outDir & ".done")
end run

on exportOne(appName, inPath, outPath)
	do shell script "mkdir -p \"$(dirname " & quoted form of outPath & ")\""
	set inFile to POSIX file inPath
	set outFile to POSIX file outPath
	with timeout of 300 seconds
		if appName is "Pages" then
			tell application "Pages"
				set d to open inFile
				try
					export d to outFile as PDF
				end try
				close d saving no
			end tell
		else if appName is "Keynote" then
			tell application "Keynote"
				set d to open inFile
				try
					export d to outFile as PDF with properties {PDF image quality:Best, skipped slides:false}
				end try
				close d saving no
			end tell
		else if appName is "Numbers" then
			tell application "Numbers"
				set d to open inFile
				try
					export d to outFile as PDF with properties {image quality:Best}
				end try
				close d saving no
			end tell
		else
			error "no application for " & appName
		end if
	end timeout
	do shell script "test -s " & quoted form of outPath
end exportOne
