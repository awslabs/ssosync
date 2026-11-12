# Prefer Established Tools

- Before writing a bash, python or awk script, look for an established tool or pattern that already does the job: standard Unix tools (`diff`, `patch`, `cmp`, `grep`, `jq`), the project's own tooling (`make` targets, existing buildspecs, `cfn-lint`, `sam`, `goreleaser`), and how the repo already solves the same problem.
- Use that tool directly, in plain readable steps. Transparency and portability (GNU on CodeBuild, BSD on macOS) beat clever scripting.
- Only write a script when no existing tool fits, keep it short, and say why the existing tools were not enough. A script that works around tool differences (regex dialects, newline handling, `sed -i`) is a sign the wrong approach was chosen.
- When proposing options to the user, present the established-tool option first and do not recommend a custom script over it without a clear reason.
- Example: the SAR variants (#406) are produced with `patch -N -F 0` and checked with `diff`, not with a custom script that edits the template.
