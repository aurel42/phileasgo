---
trigger: always_on
---

Never abbreviate Wikipedia as "Wiki"; you can abbreviate it as "WP" or "wp".
Comments: Explain why complex logic exists, not just what it does.
ONLY RELEASE WHEN PROMPTED BY THE USER. Permission can never be given implicitly. When prompted for a release without a specific version number, only ever increase the patch level.
Create temporary test and debug scripts only in cmd/experiments.
The shell is a bash on a Windows system. Pick appropriate commands.
NEVER run or terminate the server app. Ask the user to do it.
Don't touch TODO.md, it's the user's TODO list; do not react to changes or additions to the file.
Put all temporary files (coverage.out etc.) in tmp/.
We never check for the null island. This only ever hides actual issues. Instead, if necessary, check the sim state to see if the telemetry is valid.
