import json
import logging
from typing import Any

from pydantic import BaseModel, Field, ValidationError

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

    def _extract_json(self, text: str) -> Any:
        """Extract JSON object or array from AI response text."""
        arr_start = text.find("[")
        arr_end = text.rfind("]") + 1
        obj_start = text.find("{")
        obj_end = text.rfind("}") + 1

        if arr_start >= 0 and (obj_start < 0 or arr_start < obj_start):
            return json.loads(text[arr_start:arr_end])
        if obj_start >= 0:
            return json.loads(text[obj_start:obj_end])
        raise ValueError("No JSON found in AI response")

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

        data = self._extract_json(message.content[0].text)
        return AnalysisResult(**data)

    async def generate_experiments(
        self,
        topology: dict[str, Any],
        target_namespace: str = "default",
        count: int = 3,
    ) -> list[dict]:
        """Generate experiment configs from topology analysis."""
        from models.experiment import ExperimentConfig

        client = self._get_client()

        prompt = f"""Analyze this Kubernetes topology and suggest {count} chaos experiments
to test resilience weaknesses.

Topology: {topology}
Target Namespace: {target_namespace}

Respond with a JSON array of experiment configs. Each must have:
- name: descriptive experiment name (string)
- chaos_type: one of [pod_delete, network_latency, network_loss, cpu_stress, memory_stress]
- target_namespace: "{target_namespace}"
- target_labels: dict of label key-value pairs
- parameters: dict of chaos parameters
- description: why this experiment is recommended

Example:
[{{"name": "test-nginx-resilience", "chaos_type": "pod_delete",
  "target_namespace": "{target_namespace}", "target_labels": {{"app": "nginx"}},
  "parameters": {{}}, "description": "Test pod recovery"}}]"""

        message = client.messages.create(
            model=self._model,
            max_tokens=2048,
            messages=[{"role": "user", "content": prompt}],
        )

        raw = self._extract_json(message.content[0].text)
        if not isinstance(raw, list):
            raw = [raw]

        # Validate each config through Pydantic, filter invalid ones
        valid = []
        for item in raw:
            try:
                config = ExperimentConfig(**item)
                valid.append(config.model_dump())
            except (ValidationError, Exception) as e:
                logger.warning("Filtered invalid AI experiment config: %s", e)

        return valid

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

        return self._extract_json(message.content[0].text)

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

    async def review_steady_state(
        self,
        steady_state: dict[str, Any],
    ) -> dict[str, Any]:
        """Review steady state for anomalies before chaos injection."""
        client = self._get_client()

        prompt = f"""Review this Kubernetes steady state snapshot and identify any pre-existing
anomalies or risks before chaos injection.

Steady State: {steady_state}

Respond in JSON with:
- healthy: boolean indicating if the state looks healthy
- anomalies: list of detected issues
- risk_level: low/medium/high
- recommendation: brief advice"""

        message = client.messages.create(
            model=self._model,
            max_tokens=512,
            messages=[{"role": "user", "content": prompt}],
        )

        return self._extract_json(message.content[0].text)

    async def compare_observations(
        self,
        steady_state: dict[str, Any],
        observations: dict[str, Any],
        hypothesis: str | None = None,
    ) -> dict[str, Any]:
        """Compare observations against steady state after chaos injection."""
        client = self._get_client()

        prompt = f"""Compare the post-chaos observations with the original steady state.

Steady State (before): {steady_state}
Observations (after): {observations}
Hypothesis: {hypothesis or 'N/A'}

Respond in JSON with:
- hypothesis_validated: boolean
- impact_summary: brief description of changes
- severity: low/medium/high/critical
- details: list of specific changes detected"""

        message = client.messages.create(
            model=self._model,
            max_tokens=512,
            messages=[{"role": "user", "content": prompt}],
        )

        return self._extract_json(message.content[0].text)

    async def verify_recovery(
        self,
        original_state: dict[str, Any],
        current_state: dict[str, Any],
    ) -> dict[str, Any]:
        """Verify recovery completeness after rollback."""
        client = self._get_client()

        prompt = f"""Verify that the system has fully recovered after chaos rollback.

Original State (before chaos): {original_state}
Current State (after rollback): {current_state}

Respond in JSON with:
- fully_recovered: boolean
- recovery_percentage: 0-100
- remaining_issues: list of unresolved differences
- recommendation: next steps if not fully recovered"""

        message = client.messages.create(
            model=self._model,
            max_tokens=512,
            messages=[{"role": "user", "content": prompt}],
        )

        return self._extract_json(message.content[0].text)
