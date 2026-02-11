import logging
from typing import Any

from pydantic import BaseModel, Field

logger = logging.getLogger(__name__)


class RecommendedAction(BaseModel):
    action: str
    priority: str = "medium"
    description: str = ""


class AnalysisResult(BaseModel):
    severity: str = Field(description="SEV1-SEV4")
    root_cause: str
    confidence: float = Field(ge=0.0, le=1.0)
    recommendations: list[RecommendedAction] = Field(default_factory=list)
    resilience_score: float = Field(ge=0.0, le=100.0)


class AiEngine:
    """AI analysis engine using Anthropic Claude API.

    Provides experiment analysis, hypothesis generation,
    resilience scoring, and report generation.
    """

    def __init__(self, api_key: str | None = None, model: str = "claude-sonnet-4-5-20250929"):
        self._api_key = api_key
        self._model = model
        self._client = None

    def _get_client(self):
        if self._client is None:
            import anthropic

            self._client = anthropic.Anthropic(api_key=self._api_key)
        return self._client

    async def analyze_experiment(
        self,
        experiment_data: dict[str, Any],
        steady_state: dict[str, Any],
        observations: dict[str, Any],
    ) -> AnalysisResult:
        """Analyze experiment results and provide structured assessment."""
        client = self._get_client()

        prompt = f"""Analyze this chaos engineering experiment and provide a structured assessment.

Experiment: {experiment_data}
Steady State (before): {steady_state}
Observations (after): {observations}

Respond in JSON with these fields:
- severity: SEV1 (critical), SEV2 (major), SEV3 (minor), SEV4 (info)
- root_cause: brief root cause analysis
- confidence: 0.0-1.0 confidence in the analysis
- recommendations: list of {{action, priority, description}}
- resilience_score: 0-100 overall resilience score"""

        message = client.messages.create(
            model=self._model,
            max_tokens=1024,
            messages=[{"role": "user", "content": prompt}],
        )

        import json

        text = message.content[0].text
        # Extract JSON from response
        start = text.find("{")
        end = text.rfind("}") + 1
        data = json.loads(text[start:end])

        return AnalysisResult(**data)

    async def generate_hypothesis(
        self,
        topology: dict[str, Any],
        target: str,
        chaos_type: str,
    ) -> str:
        """Generate a failure hypothesis based on topology and target."""
        client = self._get_client()

        prompt = f"""Given this infrastructure topology and planned chaos experiment,
generate a hypothesis about what will happen.

Topology: {topology}
Target: {target}
Chaos Type: {chaos_type}

Respond with a clear, testable hypothesis in 1-2 sentences."""

        message = client.messages.create(
            model=self._model,
            max_tokens=256,
            messages=[{"role": "user", "content": prompt}],
        )

        return message.content[0].text

    async def calculate_resilience_score(
        self,
        experiments: list[dict[str, Any]],
    ) -> dict[str, Any]:
        """Calculate overall resilience score from experiment history."""
        client = self._get_client()

        prompt = f"""Based on these chaos experiment results, calculate a resilience score.

Experiments: {experiments}

Respond in JSON with:
- overall: 0-100 score
- categories: dict of category name to score
- recommendations: list of improvement suggestions
- details: brief summary"""

        message = client.messages.create(
            model=self._model,
            max_tokens=512,
            messages=[{"role": "user", "content": prompt}],
        )

        import json

        text = message.content[0].text
        start = text.find("{")
        end = text.rfind("}") + 1
        return json.loads(text[start:end])

    async def generate_report(
        self,
        experiment_data: dict[str, Any],
        analysis: dict[str, Any] | None = None,
    ) -> str:
        """Generate a human-readable report for an experiment."""
        client = self._get_client()

        prompt = f"""Generate a concise chaos engineering report.

Experiment: {experiment_data}
Analysis: {analysis}

Format as a brief markdown report with:
- Summary
- Impact Assessment
- Findings
- Recommendations"""

        message = client.messages.create(
            model=self._model,
            max_tokens=1024,
            messages=[{"role": "user", "content": prompt}],
        )

        return message.content[0].text
