# Security Policy

## Reporting a vulnerability

Please do not open a public issue for security problems.

Report a vulnerability privately through GitHub Security Advisories:

1. Go to the repository's **Security** tab.
2. Choose **Report a vulnerability** to open a private advisory.

Include as much detail as you can: what the issue is, how to reproduce it, and
the impact you see. We will acknowledge the report, investigate, and keep you
updated on a fix.

## Scope

This repository is the terminal UI only. It does not analyze code itself; it
spawns the lachesis engine over MCP and renders what the engine reports. Issues
in graph construction or analysis belong in the
[engine repo](https://github.com/UnboundCompute/lachesis). Report issues here
when they concern the UI: how it launches the engine, how it handles input, or
how it renders results.
