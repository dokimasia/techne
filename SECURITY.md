# Security policy

## Reporting a vulnerability

Email **security@dokimi.dev**. Do not open an issue. Keep the report private until a release
contains the fix.

State the version, from `techne --version`, or the commit. State the call you made, what
happened, and what you expected. Send a proof of concept if you have one.

We reply within 3 working days. If you have not heard back by then, the mail did not arrive.
Send it again.

## Supported versions

techne has no release yet. Report a vulnerability against the `main` branch.

## What to report

techne reads and writes the files of a workspace for an agent, and it starts the language servers
of the workspace. Report these bugs privately, because they have security consequences:

- A change that writes outside the workspace of its call.
- A write that follows a symbolic link out of the workspace.
- A refused change that techne wrote in part.
- A call that techne serves although its `wd` is under no folder of `--trust`.
- A program that techne starts and that neither a declared language server nor the build of the
  workspace runs.

techne starts the language server of a language at the first call about it. A server runs the
build of its project: rust-analyzer runs `cargo check`, and jdtls imports a Gradle or a Maven
build. Serve only a workspace whose build you would run.

Report a wrong answer, a wrong refusal and a missing reference as an ordinary issue.
