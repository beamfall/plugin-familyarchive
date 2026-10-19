#!/usr/bin/env python3
"""Mutation proofs for the Family Archive test-depth gate."""

import re
import tempfile
import unittest
from pathlib import Path

import test_depth_gate as gate


class DepthGateMutationTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.workflow = gate.WORKFLOW.read_text(encoding="utf-8")
        cls.discovered = gate.REQUIRED_FUZZ_TARGETS.copy()

    def assert_rejected(self, workflow: str, fragment: str) -> None:
        self.assertIn(fragment, gate.workflow_errors(workflow, self.discovered))

    def job_parts(self, name: str) -> tuple[int, int, str]:
        match = re.search(
            rf"^  {re.escape(name)}:\n.*?(?=^  [A-Za-z0-9_-]+:\n|\Z)",
            self.workflow,
            flags=re.MULTILINE | re.DOTALL,
        )
        self.assertIsNotNone(match)
        assert match is not None
        return match.start(), match.end(), match.group(0)

    def mutate_job(self, name: str, old: str, new: str) -> str:
        start, end, block = self.job_parts(name)
        self.assertIn(old, block)
        return self.workflow[:start] + block.replace(old, new, 1) + self.workflow[end:]

    def drop_step(self, job: str, step_name: str) -> str:
        start, end, block = self.job_parts(job)
        mutated, count = re.subn(
            rf"^      - name: {re.escape(step_name)}\n.*?(?=^      - |\Z)",
            "",
            block,
            count=1,
            flags=re.MULTILINE | re.DOTALL,
        )
        self.assertEqual(count, 1)
        return self.workflow[:start] + mutated + self.workflow[end:]

    def test_planted_aggregate_masking_fails_per_package(self) -> None:
        floors = {"example/high": 80.0, "example/low": 80.0}
        measured = {"example/high": 100.0, "example/low": 70.0}
        self.assertGreater(sum(measured.values()) / len(measured), 80.0)
        self.assertIn(
            "example/low coverage 70.0% is below 80.0%",
            gate.coverage_errors(floors, set(floors), measured),
        )

    def test_required_steps_reject_skip_comment_and_continue(self) -> None:
        jobs = (
            (
                "fast-conformance",
                "fast conformance",
                "conformance + per-package coverage + depth contract",
                f"        run: {gate.CHECK_COMMAND}",
            ),
            ("race", "race", "race", f"        run: {gate.RACE_COMMAND}"),
            ("fuzz", "fuzz", "Bounded fuzz target", f"        run: {gate.FUZZ_COMMAND}"),
            (
                "conformance",
                "conformance aggregator",
                "Require fast and race jobs",
                '          test "$RACE_RESULT" = success',
            ),
        )
        for job, label, step_name, command_line in jobs:
            header = f"      - name: {step_name}\n"
            mutations = (
                (
                    "step-if-false",
                    self.mutate_job(job, header, header + "        if: ${{ false }}\n"),
                    f"{label} command step must be unconditional",
                ),
                (
                    "commented-command",
                    self.mutate_job(
                        job,
                        command_line,
                        command_line[: len(command_line) - len(command_line.lstrip())]
                        + "# "
                        + command_line.lstrip(),
                    ),
                    f"{label} command changed or is inactive",
                ),
                (
                    "step-continue-on-error",
                    self.mutate_job(
                        job,
                        header,
                        header + "        continue-on-error: true\n",
                    ),
                    f"{label} job and steps must not continue on error",
                ),
                (
                    "job-continue-on-error",
                    self.mutate_job(
                        job,
                        f"  {job}:\n",
                        f"  {job}:\n    continue-on-error: true\n",
                    ),
                    f"{label} job and steps must not continue on error",
                ),
            )
            for mutation, workflow, error in mutations:
                with self.subTest(job=job, mutation=mutation):
                    self.assert_rejected(workflow, error)

    def test_aggregator_must_be_active_fail_closed_and_attached(self) -> None:
        mutations = (
            (
                self.mutate_job("conformance", "    if: always()", "    if: false"),
                "conformance aggregator job condition changed",
            ),
            (
                self.mutate_job(
                    "conformance",
                    "    if: always()",
                    "    # if: always()",
                ),
                "conformance aggregator job condition changed",
            ),
            (
                self.mutate_job(
                    "conformance",
                    "    needs: [fast-conformance, race]",
                    "    needs: [fast-conformance]",
                ),
                "conformance aggregator must need fast-conformance and race",
            ),
        )
        for workflow, error in mutations:
            self.assert_rejected(workflow, error)

    def test_sdk_override_removal_is_rejected_in_fast_and_race(self) -> None:
        for job, label in (("fast-conformance", "fast conformance"), ("race", "race")):
            for step, suffix in (
                ("Checkout SDK override", "checkout step"),
                ("Use SDK override", "use step"),
            ):
                with self.subTest(job=job, step=step):
                    self.assert_rejected(
                        self.drop_step(job, step),
                        f"{label} must have exactly one SDK override {suffix}",
                    )
        for step, suffix in (
            ("Checkout SDK override", "checkout"),
            ("Use SDK override", "use"),
        ):
            disabled = self.mutate_job(
                "race",
                f"      - name: {step}\n        if: {gate.SDK_OVERRIDE_IF}",
                f"      - name: {step}\n        if: ${{{{ false }}}}",
            )
            self.assert_rejected(disabled, f"race SDK override {suffix} condition changed")

    def test_floor_and_fuzz_discovery_fail_closed(self) -> None:
        errors = gate.coverage_errors(
            {"example/configured": 80.0},
            {"example/configured", "example/new"},
            {},
        )
        self.assertIn("discovered package has no coverage floor: example/new", errors)
        self.assertIn(
            "required fuzz target was not discovered: FuzzReadSidecar",
            gate.workflow_errors(self.workflow, self.discovered - {"FuzzReadSidecar"}),
        )

    def test_boolean_coverage_floor_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            path = Path(temp_dir) / "coverage-floor.json"
            path.write_text(
                '{"schema": 1, "packages": {"example/package": true}}',
                encoding="utf-8",
            )
            with self.assertRaisesRegex(gate.GateError, "map package strings to numbers"):
                gate.load_floors(path)

    def test_unbounded_or_pr_fuzz_execution_is_rejected(self) -> None:
        self.assert_rejected(
            self.workflow.replace("-fuzztime=20s", "-fuzztime=0", 1),
            "fuzz command changed or is inactive",
        )
        self.assert_rejected(
            self.workflow.replace(
                "if: github.event_name == 'schedule'",
                "if: github.event_name == 'pull_request'",
                1,
            ),
            "fuzz job condition changed",
        )


if __name__ == "__main__":
    unittest.main(verbosity=2)
