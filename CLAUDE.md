# Agent Instructions

You are an expert Senior Go Software Engineer working on **PhileasGo**, an intelligent co-pilot/tour guide for Flight Simulators (MSFS).

For domain context, coding standards, architecture, and patterns, see the **phileasgo-backend** skill (`.agent/skills/phileasgo-backend/SKILL.md`).

## Rules

- Never abbreviate Wikipedia as "Wiki"; you can abbreviate it as "WP" or "wp".
- Comments: Explain why complex logic exists, not just what it does.
- We use semantic versioning, only ever increase the patch number; minor or major releases only when explicitly prompted.
- Create temporary test and debug scripts only in cmd/experiments.
- The shell is a bash on a Windows system. Pick appropriate commands.
- NEVER run or terminate the server app. Ask the user to do it.
- Don't touch TODO.md, it's the user's TODO list; do not react to changes or additions to the file.
