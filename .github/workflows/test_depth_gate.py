#!/usr/bin/env python3
"""Fast test-depth enforcement and its mutation checks."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github/workflows/conformance.yml"
FLOORS = ROOT / "testing/coverage-floor.json"
CHECK_COMMAND = "python3 .github/workflows/test_depth_gate.py check"
RACE_COMMAND = "go test -race -count=1 ./..."
FUZZ_COMMAND = 'go test -run \'^$\' -fuzz "^${{ matrix.target }}$" -fuzztime=20s .'
REQUIRED_FUZZ_TARGETS = {"FuzzParseCaptureDate", "FuzzReadSidecar"}
AGGREGATOR_COMMANDS = (
    'test "$FAST_RESULT" = success',
    'test "$RACE_RESULT" = success',
)
SDK_OVERRIDE_IF = "${{ inputs.sdk_ref != '' }}"
SDK_OVERRIDE_COMMAND = (
    "go mod edit -replace github.com/beamfall/plugin-sdk=../beamfall-plugin-sdk"
)


class GateError(RuntimeError):
    """A deterministic gate failure."""


def run(args: list[str], *, cwd: Path = ROOT) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(args, cwd=cwd, capture_output=True, text=True, check=False)
    if result.returncode != 0:
        raise GateError(
            f"command failed ({result.returncode}): {' '.join(args)}\n"
            f"{result.stdout}{result.stderr}"
        )
    return result


def load_floors(path: Path = FLOORS) -> dict[str, float]:
    try:
        document = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise GateError(f"cannot read coverage floors: {error}") from error
    if document.get("schema") != 1 or not isinstance(document.get("packages"), dict):
        raise GateError("coverage-floor.json must have schema 1 and a packages object")
    floors: dict[str, float] = {}
    for package, value in document["packages"].items():
        if (
            not isinstance(package, str)
            or not package
            or isinstance(value, bool)
            or not isinstance(value, (int, float))
        ):
            raise GateError("coverage floors must map package strings to numbers")
        floor = float(value)
        if floor <= 0 or floor > 100:
            raise GateError(f"coverage floor for {package} must be in (0, 100]")
        floors[package] = floor
    if not floors:
        raise GateError("coverage floors must not be empty")
    return floors


def discover_packages() -> set[str]:
    return {line for line in run(["go", "list", "./..."]).stdout.splitlines() if line}


def discover_fuzz_targets() -> set[str]:
    output = run(["go", "test", "-list", r"^Fuzz[A-Za-z0-9_]+$", "."]).stdout
    return set(re.findall(r"^Fuzz[A-Za-z0-9_]+$", output, flags=re.MULTILINE))


def coverage_errors(
    floors: dict[str, float],
    packages: set[str],
    measured: dict[str, float],
) -> list[str]:
    errors: list[str] = []
    for package in sorted(packages - floors.keys()):
        errors.append(f"discovered package has no coverage floor: {package}")
    for package in sorted(floors.keys() - packages):
        errors.append(f"coverage floor targets missing package: {package}")
    for package in sorted(packages & floors.keys()):
        if package not in measured:
            errors.append(f"package has no measured coverage: {package}")
        elif measured[package] < floors[package]:
            errors.append(
                f"{package} coverage {measured[package]:.1f}% is below "
                f"{floors[package]:.1f}%"
            )
    return errors


def measure_coverage(packages: set[str]) -> dict[str, float]:
    measured: dict[str, float] = {}
    with tempfile.TemporaryDirectory(prefix="familyarchive-coverage-") as tmp:
        for index, package in enumerate(sorted(packages)):
            profile = Path(tmp) / f"coverage-{index}.out"
            test = run(
                [
                    "go",
                    "test",
                    "-count=1",
                    "-covermode=set",
                    f"-coverprofile={profile}",
                    package,
                ]
            )
            sys.stdout.write(test.stdout)
            summary = run(["go", "tool", "cover", f"-func={profile}"]).stdout
            match = re.search(r"^total:.*?([0-9]+(?:\.[0-9]+)?)%$", summary, re.MULTILINE)
            if not match:
                raise GateError(f"cannot parse total coverage for {package}\n{summary}")
            measured[package] = float(match.group(1))
    return measured


def active(text: str) -> str:
    """Strip full-line comments before matching this workflow's fixed shape."""
    return "\n".join(
        line for line in text.splitlines() if not line.lstrip().startswith("#")
    )


def job_block(workflow: str, name: str) -> str:
    match = re.search(
        rf"^  {re.escape(name)}:\n(?P<body>.*?)(?=^  [A-Za-z0-9_-]+:\n|\Z)",
        active(workflow),
        flags=re.MULTILINE | re.DOTALL,
    )
    return match.group("body") if match else ""


def workflow_steps(job: str) -> list[str]:
    return re.findall(
        r"^      - .*?(?=^      - |\Z)",
        active(job),
        flags=re.MULTILINE | re.DOTALL,
    )


def scalar_values(block: str, indent: int, key: str) -> list[str]:
    prefix = " " * indent
    pattern = re.compile(rf"^{re.escape(prefix + key)}:\s*(.*?)\s*$")
    return [
        match.group(1)
        for line in active(block).splitlines()
        if (match := pattern.match(line))
    ]


def named_steps(job: str, name: str) -> list[str]:
    pattern = re.compile(r"^      - name:\s*(.*?)\s*$")
    return [
        step
        for step in workflow_steps(job)
        if (match := pattern.match(step.splitlines()[0])) and match.group(1) == name
    ]


def step_commands(step: str) -> list[str]:
    lines = active(step).splitlines()
    commands: list[str] = []
    for index, line in enumerate(lines):
        match = re.match(r"^        run:\s*(.*?)\s*$", line)
        if not match:
            continue
        value = match.group(1)
        if value not in {"|", ">", "|-", ">-"}:
            if value:
                commands.append(value)
            continue
        for body_line in lines[index + 1 :]:
            if body_line and len(body_line) - len(body_line.lstrip()) <= 8:
                break
            if body_line.startswith("          ") and body_line.strip():
                commands.append(body_line[10:])
        break
    return commands


def nested_mapping(step: str, section: str) -> dict[str, str]:
    lines = active(step).splitlines()
    header = f"        {section}:"
    try:
        start = lines.index(header)
    except ValueError:
        return {}
    values: dict[str, str] = {}
    for line in lines[start + 1 :]:
        if line and len(line) - len(line.lstrip()) <= 8:
            break
        match = re.match(r"^          ([A-Za-z0-9_-]+):\s*(.*?)\s*$", line)
        if match:
            values[match.group(1)] = match.group(2)
    return values


def all_commands(jobs: list[str]) -> list[str]:
    return [
        command
        for job in jobs
        for step in workflow_steps(job)
        for command in step_commands(step)
    ]


def job_blocking_errors(job: str, label: str, *, expected_if: list[str]) -> list[str]:
    errors: list[str] = []
    if scalar_values(job, 4, "if") != expected_if:
        errors.append(f"{label} job condition changed")
    if scalar_values(job, 4, "continue-on-error") or any(
        scalar_values(step, 8, "continue-on-error") for step in workflow_steps(job)
    ):
        errors.append(f"{label} job and steps must not continue on error")
    return errors


def required_command_errors(
    job: str,
    label: str,
    step_name: str,
    expected_commands: list[str],
) -> list[str]:
    steps = named_steps(job, step_name)
    if len(steps) != 1:
        return [f"{label} must have exactly one {step_name!r} step"]
    step = steps[0]
    errors: list[str] = []
    if scalar_values(step, 8, "if"):
        errors.append(f"{label} command step must be unconditional")
    if scalar_values(step, 8, "continue-on-error"):
        errors.append(f"{label} command step must be blocking")
    if step_commands(step) != expected_commands:
        errors.append(f"{label} command changed or is inactive")
    return errors


def sdk_override_errors(job: str, label: str) -> list[str]:
    errors: list[str] = []
    checkout = named_steps(job, "Checkout SDK override")
    use = named_steps(job, "Use SDK override")
    if len(checkout) != 1:
        errors.append(f"{label} must have exactly one SDK override checkout step")
    else:
        step = checkout[0]
        if scalar_values(step, 8, "if") != [SDK_OVERRIDE_IF]:
            errors.append(f"{label} SDK override checkout condition changed")
        if scalar_values(step, 8, "uses") != ["actions/checkout@34e114876b0b11c390a56381ad16ebd13914f8d5 # v4.3.1"]:
            errors.append(f"{label} SDK override checkout action changed")
        if nested_mapping(step, "with") != {
            "repository": "beamfall/plugin-sdk",
            "ref": "${{ inputs.sdk_ref }}",
            "path": "beamfall-plugin-sdk",
        }:
            errors.append(f"{label} SDK override checkout inputs changed")
    if len(use) != 1:
        errors.append(f"{label} must have exactly one SDK override use step")
    else:
        step = use[0]
        if scalar_values(step, 8, "if") != [SDK_OVERRIDE_IF]:
            errors.append(f"{label} SDK override use condition changed")
        if scalar_values(step, 8, "working-directory") != ["beamfall-plugin-familyarchive"]:
            errors.append(f"{label} SDK override working directory changed")
        if step_commands(step) != [SDK_OVERRIDE_COMMAND]:
            errors.append(f"{label} SDK override command changed or is inactive")
    return errors


def scheduled_fuzz_targets(fuzz_job: str) -> set[str]:
    match = re.search(
        r"^        target:\n(?P<body>(?:^          - Fuzz[A-Za-z0-9_]+\n)+)",
        active(fuzz_job),
        flags=re.MULTILINE,
    )
    if not match:
        return set()
    return set(
        re.findall(
            r"^          - (Fuzz[A-Za-z0-9_]+)$",
            match.group("body"),
            re.MULTILINE,
        )
    )


def workflow_errors(workflow: str, discovered_fuzz: set[str]) -> list[str]:
    errors: list[str] = []
    fast_conformance = job_block(workflow, "fast-conformance")
    conformance = job_block(workflow, "conformance")
    race = job_block(workflow, "race")
    fuzz = job_block(workflow, "fuzz")
    jobs = [fast_conformance, conformance, race, fuzz]

    for job, label, step_name, command in (
        (
            fast_conformance,
            "fast conformance",
            "conformance + per-package coverage + depth contract",
            CHECK_COMMAND,
        ),
        (race, "race", "race", RACE_COMMAND),
    ):
        if not job:
            errors.append(f"missing {label} job")
            continue
        errors.extend(job_blocking_errors(job, label, expected_if=[]))
        errors.extend(required_command_errors(job, label, step_name, [command]))
        errors.extend(sdk_override_errors(job, label))
    if all_commands(jobs).count(CHECK_COMMAND) != 1:
        errors.append("canonical depth check must execute exactly once in the workflow")

    if not conformance:
        errors.append("missing preserved conformance aggregator")
    else:
        errors.extend(
            job_blocking_errors(
                conformance,
                "conformance aggregator",
                expected_if=["always()"],
            )
        )
        if scalar_values(conformance, 4, "name") != ["conformance"]:
            errors.append("preserved conformance check name changed")
        if scalar_values(conformance, 4, "needs") != ["[fast-conformance, race]"]:
            errors.append("conformance aggregator must need fast-conformance and race")
        errors.extend(
            required_command_errors(
                conformance,
                "conformance aggregator",
                "Require fast and race jobs",
                list(AGGREGATOR_COMMANDS),
            )
        )
        aggregator_steps = named_steps(conformance, "Require fast and race jobs")
        if len(aggregator_steps) == 1 and nested_mapping(aggregator_steps[0], "env") != {
            "FAST_RESULT": "${{ needs.fast-conformance.result }}",
            "RACE_RESULT": "${{ needs.race.result }}",
        }:
            errors.append("conformance aggregator dependency-result bindings changed")

    if not re.search(
        r"^  schedule:\s*$",
        active(workflow),
        flags=re.MULTILINE,
    ):
        errors.append("workflow is missing its scheduled fuzz trigger")
    if not fuzz:
        errors.append("missing scheduled fuzz job")
    else:
        errors.extend(
            job_blocking_errors(
                fuzz,
                "fuzz",
                expected_if=["github.event_name == 'schedule'"],
            )
        )
        errors.extend(
            required_command_errors(
                fuzz,
                "fuzz",
                "Bounded fuzz target",
                [FUZZ_COMMAND],
            )
        )

    scheduled = scheduled_fuzz_targets(fuzz)
    for target in sorted(REQUIRED_FUZZ_TARGETS - discovered_fuzz):
        errors.append(f"required fuzz target was not discovered: {target}")
    for target in sorted(discovered_fuzz - scheduled):
        errors.append(f"discovered fuzz target is not scheduled: {target}")
    for target in sorted(scheduled - discovered_fuzz):
        errors.append(f"scheduled fuzz target was not discovered: {target}")
    return errors


def assert_clean(errors: list[str]) -> None:
    if errors:
        raise GateError("\n".join(errors))


def check_contract() -> None:
    floors = load_floors()
    packages = discover_packages()
    configured = {package: floors[package] for package in packages if package in floors}
    assert_clean(coverage_errors(floors, packages, configured))
    fuzz_targets = discover_fuzz_targets()
    assert_clean(workflow_errors(WORKFLOW.read_text(encoding="utf-8"), fuzz_targets))
    print(
        "depth contract: "
        f"{len(packages)} floored package(s), "
        f"{len(fuzz_targets)} scheduled fuzz target(s), race required"
    )


def check_coverage() -> None:
    floors = load_floors()
    packages = discover_packages()
    measured = measure_coverage(packages)
    assert_clean(coverage_errors(floors, packages, measured))
    for package in sorted(packages):
        print(f"coverage floor: {package} {measured[package]:.1f}% >= {floors[package]:.1f}%")


def run_self_tests() -> None:
    result = run([sys.executable, ".github/workflows/test_depth_gate_test.py"])
    sys.stdout.write(result.stdout)
    sys.stdout.write(result.stderr)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=("contract", "coverage", "self-test", "check"))
    args = parser.parse_args()
    try:
        if args.command in {"self-test", "check"}:
            run_self_tests()
        if args.command in {"contract", "check"}:
            check_contract()
        if args.command in {"coverage", "check"}:
            check_coverage()
    except GateError as error:
        print(f"depth gate: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
