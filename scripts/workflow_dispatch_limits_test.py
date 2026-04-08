import re
import unittest
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parent.parent
WORKFLOWS_DIR = REPO_ROOT / ".github" / "workflows"
INPUT_NAME_RE = re.compile(r"^\s{6}([A-Za-z0-9_-]+):\s*$")


def workflow_dispatch_inputs(path: Path):
    inputs = []
    inside_dispatch = False
    inside_inputs = False

    for line in path.read_text(encoding="utf-8").splitlines():
        if re.match(r"^\s*workflow_dispatch:\s*$", line):
            inside_dispatch = True
            inside_inputs = False
            continue

        if inside_dispatch and re.match(r"^\s{2}[A-Za-z0-9_-]+:\s*$", line):
            break

        if inside_dispatch and re.match(r"^\s{4}inputs:\s*$", line):
            inside_inputs = True
            continue

        if inside_inputs:
            match = INPUT_NAME_RE.match(line)
            if match:
                inputs.append(match.group(1))
                continue
            if re.match(r"^\s{4}[A-Za-z0-9_-]+:\s*$", line):
                inside_inputs = False

    return inputs


class WorkflowDispatchLimitsTest(unittest.TestCase):
    def test_workflow_dispatch_inputs_do_not_exceed_github_limit(self):
        violations = []
        for path in sorted(WORKFLOWS_DIR.glob("*.yml")):
            inputs = workflow_dispatch_inputs(path)
            if len(inputs) > 25:
                violations.append((path.name, len(inputs), inputs))

        self.assertFalse(
            violations,
            "workflow_dispatch input limit exceeded: "
            + "; ".join(f"{name} has {count} inputs ({', '.join(inputs)})" for name, count, inputs in violations),
        )


if __name__ == "__main__":
    unittest.main()
