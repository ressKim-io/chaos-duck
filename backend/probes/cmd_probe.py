import asyncio
import logging

from .base import BaseProbe, ProbeMode, ProbeResult

logger = logging.getLogger(__name__)


class CmdProbe(BaseProbe):
    """Shell command probe.

    Executes a shell command and validates the exit code and optionally
    checks that the output contains an expected string.
    """

    def __init__(
        self,
        name: str,
        mode: ProbeMode,
        command: str,
        expected_exit_code: int = 0,
        output_contains: str | None = None,
        timeout_seconds: float = 10.0,
    ):
        super().__init__(name, mode)
        self.command = command
        self.expected_exit_code = expected_exit_code
        self.output_contains = output_contains
        self.timeout_seconds = timeout_seconds

    @property
    def probe_type(self) -> str:
        return "cmd"

    async def execute(self) -> ProbeResult:
        proc = await asyncio.create_subprocess_shell(
            self.command,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )

        try:
            stdout, stderr = await asyncio.wait_for(
                proc.communicate(), timeout=self.timeout_seconds
            )
        except TimeoutError:
            proc.kill()
            return ProbeResult(
                probe_name=self.name,
                probe_type=self.probe_type,
                mode=self.mode,
                passed=False,
                error=f"Command timed out after {self.timeout_seconds}s",
            )

        exit_code = proc.returncode
        stdout_text = stdout.decode(errors="replace").strip()
        stderr_text = stderr.decode(errors="replace").strip()

        exit_ok = exit_code == self.expected_exit_code
        output_ok = True
        if self.output_contains and exit_ok:
            output_ok = self.output_contains in stdout_text

        passed = exit_ok and output_ok
        detail = {
            "command": self.command,
            "exit_code": exit_code,
            "expected_exit_code": self.expected_exit_code,
            "stdout": stdout_text[:500],
            "stderr": stderr_text[:500],
            "output_match": output_ok,
        }

        return ProbeResult(
            probe_name=self.name,
            probe_type=self.probe_type,
            mode=self.mode,
            passed=passed,
            detail=detail,
        )
